package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type databasePinger interface{ Ping(context.Context) error }

type healthHandler struct {
	database    databasePinger
	pingTimeout time.Duration
}

func NewHandler(database databasePinger, pingTimeout time.Duration, service appointmentService, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	health := healthHandler{database: database, pingTimeout: pingTimeout}
	booking := appointmentHandler{service: service}
	mux.HandleFunc("/healthz", onlyMethod(http.MethodGet, health.handle))
	mux.HandleFunc("/appointments", onlyMethod(http.MethodPost, booking.create))
	mux.HandleFunc("/appointments/{id}", onlyMethod(http.MethodGet, booking.get))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, 404, "ROUTE_NOT_FOUND", "Route not found.")
	})
	return requestLogging(mux, logger)
}

func onlyMethod(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeError(w, r, 405, "METHOD_NOT_ALLOWED", "Method not allowed for this route.")
			return
		}
		next(w, r)
	}
}

func (h healthHandler) handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.pingTimeout)
	defer cancel()
	if err := h.database.Ping(ctx); err != nil {
		r.Context().Value(requestInfoKey{}).(*requestInfo).code = "SERVICE_UNAVAILABLE"
		writeJSON(w, 503, map[string]string{"status": "unavailable", "database": "down"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok", "database": "up"})
}

type requestInfo struct {
	id, code      string
	appointmentID int64
}
type requestInfoKey struct{}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	info := r.Context().Value(requestInfoKey{}).(*requestInfo)
	info.code = code
	writeJSON(w, status, map[string]any{"error": map[string]string{
		"code": code, "message": message, "request_id": info.id,
	}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func requestLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		info := &requestInfo{id: rand.Text()}
		r = r.WithContext(context.WithValue(r.Context(), requestInfoKey{}, info))
		w.Header().Set("X-Request-ID", info.id)
		response := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(response, r)
		level := slog.LevelInfo
		if response.status >= 500 {
			level = slog.LevelError
		}
		// Route templates avoid logging customer IDs, request bodies, or URL queries.
		fields := []any{
			"request_id", info.id, "method", r.Method, "route", r.Pattern,
			"status", response.status, "code", info.code, "duration_ms", time.Since(start).Milliseconds(),
		}
		if info.appointmentID > 0 {
			fields = append(fields, "appointment_id", info.appointmentID)
		}
		logger.Log(r.Context(), level, "HTTP request completed", fields...)
	})
}
