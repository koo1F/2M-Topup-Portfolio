package user

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Handler handles HTTP requests for user-related endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new user Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Register handles POST /api/auth/register
//
//	201 Created  — user created, returns RegisterResponse
//	400 Bad Request — validation error or duplicate email
//	500 Internal Server Error
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Register(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrEmailTaken):
			writeError(w, http.StatusConflict, "email already registered")
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Login handles POST /api/auth/login
//
//	200 OK     — returns LoginResponse with JWT token
//	400        — missing/malformed body
//	401        — wrong credentials
//	500        — unexpected error
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
