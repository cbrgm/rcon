package rconhttp

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/cbrgm/rcon/rconclient"
)

// Handler executes RCON commands over reused, auto-reconnecting sessions. It
// implements http.Handler and io.Closer.
//
// Requests to the same backend serialize, since RCON has no request
// multiplexing; different backends run independently. The handler is
// path-agnostic: it treats any accepted request as "run one command", so you
// decide where to mount it. It executes administrative commands, so place it
// behind your own authentication and TLS.
type Handler struct {
	resolver Resolver
	cache    *sessionCache
	logger   *slog.Logger
}

// New returns a Handler. By default it targets backend; pass WithResolver to
// switch backends per request. New performs no I/O; sessions are dialed lazily
// and cached. Call Close to release them. A request whose resolved backend has
// no address (an empty fixed backend with no resolver, or a resolver that
// returns an empty address) fails with 500.
func New(backend Backend, opts ...Option) *Handler {
	cfg := config{
		resolver: fixedResolver{backend},
		client:   rconclient.New(),
		idle:     DefaultIdleTimeout,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	h := &Handler{
		resolver: cfg.resolver,
		cache:    newCache(cfg.client, cfg.idle, cfg.logger),
		logger:   cfg.logger,
	}
	h.cache.startReaper()
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	backend, err := h.resolver.Resolve(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeError(w, r, status, err)
		return
	}
	if backend.Addr == "" {
		writeError(w, r, http.StatusInternalServerError, errNoBackend)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	cmd, err := parseCommand(r)
	if err != nil {
		status := http.StatusBadRequest
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, r, status, err)
		return
	}

	out, err := h.cache.execute(r.Context(), backend, cmd)
	if err != nil {
		writeError(w, r, executeStatus(err), err)
		return
	}
	writeResult(w, r, cmd, out)
}

// Close releases all cached backend sessions. It expects no concurrent
// in-flight requests: call it during shutdown, after the server has stopped
// accepting new requests, because a request racing Close could otherwise dial a
// session that Close does not observe.
func (h *Handler) Close() error {
	return h.cache.close()
}
