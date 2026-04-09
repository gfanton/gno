// misc/gnodig/cmd/gnodig/probe.go
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
	"github.com/gnolang/gno/misc/gnodig/internal/nodedata"
	"github.com/gnolang/gno/misc/gnodig/internal/probeauth"
	"github.com/gnolang/gno/misc/gnodig/internal/probeserver"
)

// ---- probe dispatcher

func cmdProbe(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gnodig probe <serve|init|keys>")
	}
	switch args[0] {
	case "serve":
		return cmdProbeServe(args[1:])
	case "init":
		return cmdProbeInit(args[1:])
	case "keys":
		return cmdProbeKeys(args[1:])
	default:
		return fmt.Errorf("unknown probe command: %s", args[0])
	}
}

// ---- probe serve

func cmdProbeServe(args []string) error {
	fs := flag.NewFlagSet("probe serve", flag.ContinueOnError)
	listen := fs.String("listen", ":9090", "address to listen on")
	dataDir := fs.String("data-dir", "", "path to gnoland data directory")
	rpcURL := fs.String("rpc", "", "node RPC URL")
	logFile := fs.String("log-file", "", "path to gnoland log file")
	authorizedKeys := fs.String("authorized-keys", "", "path to authorized keys file (required)")
	maxConcurrent := fs.Int("max-concurrent", 5, "maximum concurrent requests")
	memoryLimit := fs.String("memory-limit", "512MB", "process memory limit (e.g. 512MB, 1GB)")
	requestTimeout := fs.Duration("request-timeout", 30*time.Second, "per-request timeout")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *authorizedKeys == "" {
		return fmt.Errorf("--authorized-keys is required")
	}

	memLimit, err := parseMemLimit(*memoryLimit)
	if err != nil {
		return fmt.Errorf("--memory-limit: %w", err)
	}
	debug.SetMemoryLimit(memLimit)

	srv, err := probeserver.New(probeserver.Config{
		AuthorizedKeysPath: *authorizedKeys,
		MaxConcurrent:      *maxConcurrent,
		RequestTimeout:     *requestTimeout,
	})
	if err != nil {
		return fmt.Errorf("probe serve: %w", err)
	}

	if *rpcURL != "" {
		probeserver.RegisterRPCTools(srv, chainrpc.New(*rpcURL))
	}

	if *dataDir != "" {
		dd, err := nodedata.Open(*dataDir)
		if err != nil {
			return fmt.Errorf("probe serve: open data dir: %w", err)
		}
		defer dd.Close()
		probeserver.RegisterNodeDataTools(srv, dd)
	}

	if *logFile != "" {
		fmt.Fprintln(os.Stderr, "warning: --log-file not yet supported, ignoring")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("probe serve: listen %s: %w", *listen, err)
	}

	// SIGHUP: reload authorized keys.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	go func() {
		defer signal.Stop(sigCh)
		for {
			select {
			case <-sigCh:
				if err := srv.ReloadKeys(); err != nil {
					fmt.Fprintf(os.Stderr, "probe serve: reload keys: %v\n", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	httpSrv := &http.Server{Handler: srv}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "probe serve: shutdown: %v\n", err)
		}
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("probe serve: %w", err)
		}
		return nil
	}
}

// parseMemLimit parses a human-readable memory limit string (e.g. "512MB",
// "1GB") and returns the value in bytes.
func parseMemLimit(limit string) (int64, error) {
	switch {
	case strings.HasSuffix(limit, "GB"):
		n, err := strconv.ParseInt(strings.TrimSuffix(limit, "GB"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory limit %q: %w", limit, err)
		}
		return n * 1 << 30, nil
	case strings.HasSuffix(limit, "MB"):
		n, err := strconv.ParseInt(strings.TrimSuffix(limit, "MB"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory limit %q: %w", limit, err)
		}
		return n * 1 << 20, nil
	default:
		return 0, fmt.Errorf("unknown format %q: expected suffix MB or GB", limit)
	}
}

// ---- probe init (stub)

func cmdProbeInit(_ []string) error {
	return errors.New("probe init: not yet implemented")
}

// ---- probe keys

func cmdProbeKeys(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gnodig probe keys <add|list|remove>")
	}
	switch args[0] {
	case "add":
		return cmdProbeKeysAdd(args[1:])
	case "list":
		return cmdProbeKeysList(args[1:])
	case "remove":
		return cmdProbeKeysRemove(args[1:])
	default:
		return fmt.Errorf("unknown keys command: %s", args[0])
	}
}

func cmdProbeKeysAdd(args []string) error {
	fs := flag.NewFlagSet("probe keys add", flag.ContinueOnError)
	name := fs.String("name", "", "key name/comment (required)")
	file := fs.String("file", "authorized_keys", "path to authorized_keys file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: gnodig probe keys add --name <name> <base64-pubkey>")
	}

	encoded := fs.Arg(0)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("invalid base64 key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid key length: expected %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}

	f, err := os.OpenFile(*file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", *file, err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "ssh-ed25519 %s %s\n", encoded, *name); err != nil {
		return fmt.Errorf("write key: %w", err)
	}

	fmt.Printf("Added key %q to %s\n", *name, *file)
	return nil
}

func cmdProbeKeysList(args []string) error {
	fs := flag.NewFlagSet("probe keys list", flag.ContinueOnError)
	file := fs.String("file", "authorized_keys", "path to authorized_keys file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ks, err := probeauth.ParseAuthorizedKeys(*file)
	if err != nil {
		return fmt.Errorf("read keys: %w", err)
	}

	entries := ks.Entries()
	if len(entries) == 0 {
		fmt.Println("No authorized keys.")
		return nil
	}

	for _, e := range entries {
		fmt.Printf("ssh-ed25519 %s %s\n", base64.StdEncoding.EncodeToString(e.PubKey), e.Comment)
	}
	return nil
}

func cmdProbeKeysRemove(args []string) error {
	fs := flag.NewFlagSet("probe keys remove", flag.ContinueOnError)
	file := fs.String("file", "authorized_keys", "path to authorized_keys file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: gnodig probe keys remove [--file <path>] <name>")
	}
	name := fs.Arg(0)

	data, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("read %s: %w", *file, err)
	}

	lines := strings.Split(string(data), "\n")
	var kept []string
	found := false
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.Join(fields[2:], " ") == name {
			found = true
			continue
		}
		kept = append(kept, line)
	}
	if !found {
		return fmt.Errorf("key %q not found in %s", name, *file)
	}

	tmp := *file + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(kept, "\n")), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, *file); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, *file, err)
	}

	fmt.Printf("Removed key %q from %s\n", name, *file)
	return nil
}
