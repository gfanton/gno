package probeserver

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gnolang/gno/misc/gnodig/internal/probeapi"
	"github.com/gnolang/gno/misc/gnodig/internal/probeauth"
)

const (
	challengeTTL       = 60 * time.Second
	tokenTTL           = 1 * time.Hour
	maxPending         = 1000
	maxTokens          = 10000
	maxBodySize        = 1 << 20 // 1 MiB
	defaultTimeout     = 30 * time.Second
	defaultConcurrency = 64
)

// ToolHandler processes a tool invocation and returns a JSON result.
type ToolHandler func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)

// Config controls server behavior.
type Config struct {
	AuthorizedKeysPath string
	MaxConcurrent      int
	RequestTimeout     time.Duration
}

type tokenEntry struct {
	pub      ed25519.PublicKey
	issuedAt time.Time
}

// Server is the probe HTTP server handling authentication and tool dispatch.
type Server struct {
	keys    *probeauth.KeyStore
	tools   map[string]ToolHandler
	mux     *http.ServeMux
	sem     chan struct{}
	timeout time.Duration
	logger  *slog.Logger
	audit   *AuditLogger

	pendingMu sync.Mutex
	pending   map[string]time.Time // challenge (base64) → issued time

	tokenMu sync.RWMutex
	tokens  map[string]tokenEntry // token → entry
}

// New creates a Server from the given Config.
func New(cfg Config) (*Server, error) {
	keys, err := probeauth.NewKeyStore(cfg.AuthorizedKeysPath)
	if err != nil {
		return nil, err
	}

	maxConc := cfg.MaxConcurrent
	if maxConc <= 0 {
		maxConc = defaultConcurrency
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	logger := slog.Default()
	s := &Server{
		keys:    keys,
		tools:   make(map[string]ToolHandler),
		mux:     http.NewServeMux(),
		sem:     make(chan struct{}, maxConc),
		timeout: timeout,
		logger:  logger,
		audit:   NewAuditLogger(logger),
		pending: make(map[string]time.Time),
		tokens:  make(map[string]tokenEntry),
	}

	s.mux.HandleFunc("POST /v1/auth", s.handleAuth)
	s.mux.HandleFunc("POST /v1/tool/{tool}", s.handleTool)
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)

	return s, nil
}

// HandleTool registers a named tool handler.
// Must be called before the server starts serving HTTP requests.
func (s *Server) HandleTool(name string, handler ToolHandler) {
	s.tools[name] = handler
}

// HasTool reports whether a tool with the given name is registered.
func (s *Server) HasTool(name string) bool {
	_, ok := s.tools[name]
	return ok
}

// ReloadKeys re-reads the authorized keys file.
func (s *Server) ReloadKeys() error {
	return s.keys.Reload()
}

// ServeHTTP applies the body size limit, concurrency limiter, and request
// timeout, then delegates to the internal mux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	// Concurrency limiter — non-blocking.
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	default:
		s.writeError(w, http.StatusTooManyRequests, probeapi.ErrOverloaded, "too many concurrent requests")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	s.mux.ServeHTTP(w, r.WithContext(ctx))
}

// ---- Auth Handshake

func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	phase := r.Header.Get("X-Probe-Phase")

	if phase == "" || phase == "challenge" {
		s.handleAuthChallenge(w)
		return
	}

	if phase == "verify" {
		s.handleAuthVerify(w, r)
		return
	}

	s.writeError(w, http.StatusBadRequest, probeapi.ErrAuthFailed, "unknown auth phase")
}

func (s *Server) handleAuthChallenge(w http.ResponseWriter) {
	s.pendingMu.Lock()
	s.evictExpiredPending()

	if len(s.pending) >= maxPending {
		s.pendingMu.Unlock()
		s.writeError(w, http.StatusServiceUnavailable, probeapi.ErrOverloaded, "too many pending challenges")
		return
	}

	challenge, err := probeauth.GenerateChallenge()
	if err != nil {
		s.pendingMu.Unlock()
		s.writeError(w, http.StatusInternalServerError, probeapi.ErrInternal, "challenge generation failed")
		return
	}

	key := base64.StdEncoding.EncodeToString(challenge)
	s.pending[key] = time.Now()
	s.pendingMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(probeapi.Handshake{
		Version:   probeapi.ProtocolVersion,
		Challenge: challenge,
	}); err != nil {
		s.logger.Warn("write challenge response", "error", err)
	}
}

func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	var hs probeapi.Handshake
	if err := json.NewDecoder(r.Body).Decode(&hs); err != nil {
		s.writeError(w, http.StatusBadRequest, probeapi.ErrAuthFailed, "invalid request body")
		return
	}

	if hs.Version != probeapi.ProtocolVersion {
		s.writeError(w, http.StatusBadRequest, probeapi.ErrAuthFailed, "unsupported protocol version")
		return
	}

	if len(hs.PubKey) != ed25519.PublicKeySize {
		s.writeError(w, http.StatusBadRequest, probeapi.ErrAuthFailed, "invalid public key length")
		return
	}

	// Consume the pending challenge (one-time use).
	challengeKey := base64.StdEncoding.EncodeToString(hs.Challenge)
	s.pendingMu.Lock()
	issued, ok := s.pending[challengeKey]
	if ok {
		delete(s.pending, challengeKey)
	}
	s.pendingMu.Unlock()

	if !ok {
		s.writeError(w, http.StatusUnauthorized, probeapi.ErrAuthFailed, "unknown or expired challenge")
		return
	}
	if time.Since(issued) > challengeTTL {
		s.writeError(w, http.StatusUnauthorized, probeapi.ErrAuthFailed, "challenge expired")
		return
	}

	pub := ed25519.PublicKey(hs.PubKey)
	if !s.keys.Contains(pub) {
		s.writeError(w, http.StatusUnauthorized, probeapi.ErrAuthFailed, "public key not authorized")
		return
	}

	if !probeauth.VerifyChallenge(pub, hs.Challenge, hs.Signature) {
		s.writeError(w, http.StatusUnauthorized, probeapi.ErrAuthFailed, "signature verification failed")
		return
	}

	token, err := base64Rand()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, probeapi.ErrInternal, "token generation failed")
		return
	}

	s.tokenMu.Lock()
	s.evictExpiredTokens()
	if len(s.tokens) >= maxTokens {
		s.tokenMu.Unlock()
		s.writeError(w, http.StatusServiceUnavailable, probeapi.ErrOverloaded, "too many active sessions")
		return
	}
	s.tokens[token] = tokenEntry{pub: pub, issuedAt: time.Now()}
	s.tokenMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(probeapi.AuthResponse{Token: token}); err != nil {
		s.logger.Warn("write auth response", "error", err)
	}
}

// Authenticate performs a full challenge-response handshake and returns a
// bearer token. Intended for testing.
func (s *Server) Authenticate(pub ed25519.PublicKey, priv ed25519.PrivateKey) (string, error) {
	// Phase 1: get a challenge.
	challenge, err := probeauth.GenerateChallenge()
	if err != nil {
		return "", err
	}

	key := base64.StdEncoding.EncodeToString(challenge)
	s.pendingMu.Lock()
	s.pending[key] = time.Now()
	s.pendingMu.Unlock()

	// Phase 2: sign and verify. Consume the challenge regardless of outcome.
	sig := probeauth.SignChallenge(priv, challenge)

	s.pendingMu.Lock()
	delete(s.pending, key)
	s.pendingMu.Unlock()

	if !s.keys.Contains(pub) {
		return "", &probeapi.ToolError{Code: probeapi.ErrAuthFailed, Message: "public key not authorized"}
	}
	if !probeauth.VerifyChallenge(pub, challenge, sig) {
		return "", &probeapi.ToolError{Code: probeapi.ErrAuthFailed, Message: "signature verification failed"}
	}

	token, err := base64Rand()
	if err != nil {
		return "", err
	}

	s.tokenMu.Lock()
	s.tokens[token] = tokenEntry{pub: pub, issuedAt: time.Now()}
	s.tokenMu.Unlock()

	return token, nil
}

// ---- Tool Dispatch

func (s *Server) handleTool(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticateRequest(w, r)
	if !ok {
		return
	}

	toolName := r.PathValue("tool")
	handler, ok := s.tools[toolName]
	if !ok {
		s.writeError(w, http.StatusNotFound, probeapi.ErrNotFound, "tool not found: "+toolName)
		return
	}

	var req probeapi.ToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, probeapi.ErrInternal, "invalid request body")
		return
	}

	start := time.Now()
	result, err := handler(r.Context(), req.Params)
	duration := time.Since(start)

	s.audit.LogToolCall(toolName, identity, duration, err != nil)

	if err != nil {
		s.writeError(w, http.StatusInternalServerError, probeapi.ErrInternal, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if encErr := json.NewEncoder(w).Encode(probeapi.ToolResponse{Result: result}); encErr != nil {
		s.logger.Warn("write tool response", "error", encErr)
	}
}

// ---- Health

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`)) // best-effort, nothing to do on error
}

// ---- Auth Helpers

// authenticateRequest validates the Authorization header. Returns the
// identity (key comment) and true on success. On failure, writes an error
// response and returns ("", false).
func (s *Server) authenticateRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		s.writeError(w, http.StatusUnauthorized, probeapi.ErrAuthFailed, "missing authorization header")
		return "", false
	}

	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok {
		s.writeError(w, http.StatusUnauthorized, probeapi.ErrAuthFailed, "invalid authorization scheme")
		return "", false
	}

	// Token lookup: map index is not constant-time, but tokens are 256-bit
	// random values, making incremental guessing infeasible.
	s.tokenMu.RLock()
	entry, found := s.tokens[token]
	s.tokenMu.RUnlock()

	if !found {
		s.writeError(w, http.StatusUnauthorized, probeapi.ErrAuthFailed, "invalid token")
		return "", false
	}

	if time.Since(entry.issuedAt) > tokenTTL {
		s.tokenMu.Lock()
		delete(s.tokens, token)
		s.tokenMu.Unlock()
		s.writeError(w, http.StatusUnauthorized, probeapi.ErrAuthFailed, "token expired")
		return "", false
	}

	return s.keys.Comment(entry.pub), true
}

// ---- Eviction

// evictExpiredPending removes challenges older than challengeTTL.
// Must be called with pendingMu held.
func (s *Server) evictExpiredPending() {
	now := time.Now()
	for k, issued := range s.pending {
		if now.Sub(issued) > challengeTTL {
			delete(s.pending, k)
		}
	}
}

// evictExpiredTokens removes tokens older than tokenTTL.
// Must be called with tokenMu held for writing.
func (s *Server) evictExpiredTokens() {
	now := time.Now()
	for k, entry := range s.tokens {
		if now.Sub(entry.issuedAt) > tokenTTL {
			delete(s.tokens, k)
		}
	}
}

// ---- Response Helpers

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(probeapi.NewErrorResponse(code, message)); err != nil {
		s.logger.Warn("write error response", "error", err)
	}
}

// base64Rand generates a 32-byte random value encoded as URL-safe base64.
func base64Rand() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
