package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type databasePinger interface {
	Ping(context.Context) error
}

type healthHandler struct {
	database    databasePinger
	pingTimeout time.Duration
}

func NewHandler(database databasePinger, pingTimeout time.Duration) http.Handler {
	mux := http.NewServeMux()
	health := healthHandler{database: database, pingTimeout: pingTimeout}
	mux.HandleFunc("GET /healthz", health.handle)
	return mux
}

func (h healthHandler) handle(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), h.pingTimeout)
	defer cancel()

	status := http.StatusOK
	payload := map[string]string{"status": "ok", "database": "up"}
	if err := h.database.Ping(ctx); err != nil {
		status = http.StatusServiceUnavailable
		payload = map[string]string{"status": "unavailable", "database": "down"}
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}
