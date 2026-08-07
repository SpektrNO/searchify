package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultHTTPPath     = "/mcp"
	defaultRequestBudget = 60 * time.Second
	readHeaderTimeout    = 5 * time.Second
)

// HTTPOptions configures the Streamable HTTP transport.
type HTTPOptions struct {
	Addr string
	Path string
}

// Handler returns an http.Handler with healthz, logging, auth, and MCP.
func (s *Server) Handler(opts HTTPOptions) (http.Handler, error) {
	if strings.TrimSpace(s.cfg.HTTPToken) == "" {
		return nil, fmt.Errorf("%s is required for HTTP mode", "SEARCHIFY_HTTP_TOKEN")
	}

	path := opts.Path
	if path == "" {
		path = defaultHTTPPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcp
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
		Logger:    slog.Default(),
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle(path, bearerAuth(s.cfg.HTTPToken, mcpHandler))
	mux.Handle("/v1/search", bearerAuth(s.cfg.HTTPToken, http.HandlerFunc(s.handleV1Search)))
	mux.Handle("/v1/index", bearerAuth(s.cfg.HTTPToken, http.HandlerFunc(s.handleV1Index)))
	mux.Handle("/v1/files", bearerAuth(s.cfg.HTTPToken, http.HandlerFunc(s.handleV1Files)))
	mux.Handle("/v1/stats", bearerAuth(s.cfg.HTTPToken, http.HandlerFunc(s.handleV1Stats)))

	return requestLogger(mux), nil
}

// RunHTTP serves MCP over Streamable HTTP until ctx is cancelled or the server errors.
func (s *Server) RunHTTP(ctx context.Context, opts HTTPOptions) error {
	handler, err := s.Handler(opts)
	if err != nil {
		return err
	}

	addr := opts.Addr
	if addr == "" {
		addr = s.cfg.HTTPAddr
	}
	if addr == "" {
		addr = ":8080"
	}
	path := opts.Path
	if path == "" {
		path = defaultHTTPPath
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           withRequestTimeout(handler, defaultRequestBudget),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       defaultRequestBudget,
		WriteTimeout:      defaultRequestBudget + 10*time.Second,
		IdleTimeout:       120 * time.Second,
	}

	slog.Info("searchify http listening",
		"addr", addr,
		"path", path,
		"rest", "/v1/search,/v1/index,/v1/files,/v1/stats",
		"auth", true,
		"stateless", true,
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func bearerAuth(token string, next http.Handler) http.Handler {
	want := "Bearer " + token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if got != want {
			w.Header().Set("WWW-Authenticate", `Bearer realm="searchify"`)
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func withRequestTimeout(next http.Handler, budget time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), budget)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
