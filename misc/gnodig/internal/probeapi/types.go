package probeapi

import "encoding/json"

// ProtocolVersion is incremented on breaking changes.
const ProtocolVersion = 1

// ---- Error Codes

const (
	ErrAuthFailed  = "auth_failed"
	ErrNotFound    = "not_found"
	ErrTimeout     = "timeout"
	ErrOverloaded  = "overloaded"
	ErrUnavailable = "unavailable"
	ErrInternal    = "internal"
)

// ---- Handshake

// Handshake is the first exchange on connect.
// Server sends Challenge; client replies with Signature + PubKey.
type Handshake struct {
	Version   int    `json:"version"`
	Challenge []byte `json:"challenge,omitempty"`
	Signature []byte `json:"signature,omitempty"`
	PubKey    []byte `json:"pubkey,omitempty"`
}

// ---- Tool Call

// ToolRequest wraps any tool invocation.
type ToolRequest struct {
	Tool   string          `json:"tool"`
	Params json.RawMessage `json:"params"`
}

// ToolResponse wraps any tool result.
type ToolResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ToolError      `json:"error,omitempty"`
}

// ToolError is a structured error returned by the probe.
type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ToolError) Error() string {
	return e.Code + ": " + e.Message
}

// NewErrorResponse creates a ToolResponse with an error.
func NewErrorResponse(code, message string) ToolResponse {
	return ToolResponse{
		Error: &ToolError{Code: code, Message: message},
	}
}

// AuthResponse is returned after a successful authentication.
type AuthResponse struct {
	Token string `json:"token"`
}

// NewResultResponse creates a ToolResponse with a JSON result.
func NewResultResponse(v any) (ToolResponse, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return ToolResponse{}, err
	}
	return ToolResponse{Result: json.RawMessage(data)}, nil
}
