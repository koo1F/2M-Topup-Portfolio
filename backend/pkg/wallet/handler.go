package wallet

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/2m-topup/backend/pkg/middleware"
)

// Handler handles HTTP requests for wallet endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new wallet Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// GetBalance handles GET /api/wallet
//
//	200 OK  → {"balance":500.00,"currency":"THB"}
//	401     → missing/invalid token
//	500     → unexpected error
func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resp, err := h.svc.GetBalance(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetTransactions handles GET /api/wallet/transactions?page=1&limit=20
//
//	200 OK  → {"data":[...],"pagination":{...}}
//	401     → missing/invalid token
//	500     → unexpected error
func (h *Handler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 20)

	resp, err := h.svc.GetTransactions(r.Context(), userID, page, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- helpers ---

func queryInt(r *http.Request, key string, def int) int {
	if s := r.URL.Query().Get(key); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return v
		}
	}
	return def
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
