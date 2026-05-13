package xray

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/InazumaV/V2bX/conf"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// FallbackServer runs a tiny HTTP server on a unix socket that xray-core's
// VLESS fallback hands off to. Same listener handles both HTTP/1.1 and HTTP/2
// (via h2c) so the response looks consistent regardless of the client's
// negotiated ALPN — xray-core terminates TLS, so traffic here is plaintext.
type FallbackServer struct {
	tag      string
	sockPath string
	listener net.Listener
	httpSrv  *http.Server
	spec     conf.BuiltinFallbackSpec
	mu       sync.Mutex
	stopped  bool
}

func NewFallbackServer(tag string, spec conf.BuiltinFallbackSpec) *FallbackServer {
	return &FallbackServer{tag: tag, spec: spec}
}

// Start binds a unix socket and serves until Stop. Returns the socket path
// that callers should hand to xray-core as the fallback dest ("unix:<path>").
func (s *FallbackServer) Start() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	safeTag := sanitizeForFilename(s.tag)
	s.sockPath = filepath.Join(os.TempDir(), fmt.Sprintf("v2bx-fb-%s-%d.sock", safeTag, os.Getpid()))
	_ = os.Remove(s.sockPath)

	ln, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return "", fmt.Errorf("listen unix %s: %w", s.sockPath, err)
	}
	if err := os.Chmod(s.sockPath, 0o660); err != nil {
		_ = ln.Close()
		_ = os.Remove(s.sockPath)
		return "", fmt.Errorf("chmod %s: %w", s.sockPath, err)
	}

	handler := h2c.NewHandler(http.HandlerFunc(s.serve), &http2.Server{})
	s.httpSrv = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.listener = ln

	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.WithField("tag", s.tag).WithError(err).Warn("builtin fallback server exited")
		}
	}()
	log.WithField("tag", s.tag).WithField("sock", s.sockPath).Info("Builtin fallback server started")
	return s.sockPath, nil
}

func (s *FallbackServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return nil
	}
	s.stopped = true

	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(ctx)
	}
	if s.sockPath != "" {
		_ = os.Remove(s.sockPath)
	}
	return nil
}

func (s *FallbackServer) serve(w http.ResponseWriter, _ *http.Request) {
	mode := s.spec.Mode
	if mode == "" {
		mode = "empty200"
	}
	switch mode {
	case "empty200":
		w.WriteHeader(http.StatusOK)
	case "empty204":
		w.WriteHeader(http.StatusNoContent)
	case "notfound":
		w.WriteHeader(http.StatusNotFound)
	case "custom":
		for k, v := range s.spec.Headers {
			w.Header().Set(k, v)
		}
		status := s.spec.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		if s.spec.Body != "" {
			_, _ = w.Write([]byte(s.spec.Body))
		}
	default:
		w.WriteHeader(http.StatusOK)
	}
}

// sanitizeForFilename keeps the unix socket path predictable and free of
// characters that would confuse downstream tooling.
func sanitizeForFilename(s string) string {
	r := strings.NewReplacer("/", "_", ":", "_", " ", "_", "[", "", "]", "")
	out := r.Replace(s)
	if len(out) > 40 {
		out = out[:40]
	}
	if out == "" {
		out = "node"
	}
	return out
}
