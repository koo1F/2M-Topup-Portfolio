package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/2m-topup/backend/pkg/user"
)

// --- Mock Service ---

type mockService struct {
	registerFn func(ctx context.Context, req user.RegisterRequest) (*user.RegisterResponse, error)
	loginFn    func(ctx context.Context, req user.LoginRequest) (*user.LoginResponse, error)
}

func (m *mockService) Register(ctx context.Context, req user.RegisterRequest) (*user.RegisterResponse, error) {
	return m.registerFn(ctx, req)
}

func (m *mockService) Login(ctx context.Context, req user.LoginRequest) (*user.LoginResponse, error) {
	return m.loginFn(ctx, req)
}

// --- Register tests ---

func TestRegister_Success(t *testing.T) {
	now := time.Now()
	svc := &mockService{
		registerFn: func(_ context.Context, req user.RegisterRequest) (*user.RegisterResponse, error) {
			return &user.RegisterResponse{ID: 1, Email: req.Email, CreatedAt: now}, nil
		},
	}
	h := user.NewHandler(svc)

	body := mustMarshal(t, map[string]string{"email": "test@test.com", "password": "secret123"})
	w, r := newRequest(t, http.MethodPost, "/api/auth/register", body)
	h.Register(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp user.RegisterResponse
	mustDecode(t, w.Body.Bytes(), &resp)

	if resp.ID != 1 {
		t.Errorf("expected id=1, got %d", resp.ID)
	}
	if resp.Email != "test@test.com" {
		t.Errorf("expected email=test@test.com, got %s", resp.Email)
	}
}

func TestRegister_InvalidInput(t *testing.T) {
	svc := &mockService{
		registerFn: func(_ context.Context, req user.RegisterRequest) (*user.RegisterResponse, error) {
			return nil, user.ErrInvalidInput
		},
	}
	h := user.NewHandler(svc)

	body := mustMarshal(t, map[string]string{"email": "not-an-email", "password": "x"})
	w, r := newRequest(t, http.MethodPost, "/api/auth/register", body)
	h.Register(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc := &mockService{
		registerFn: func(_ context.Context, _ user.RegisterRequest) (*user.RegisterResponse, error) {
			return nil, user.ErrEmailTaken
		},
	}
	h := user.NewHandler(svc)

	body := mustMarshal(t, map[string]string{"email": "dup@test.com", "password": "secret123"})
	w, r := newRequest(t, http.MethodPost, "/api/auth/register", body)
	h.Register(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestRegister_BadJSON(t *testing.T) {
	h := user.NewHandler(&mockService{})

	w, r := newRequest(t, http.MethodPost, "/api/auth/register", []byte(`{bad json`))
	h.Register(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Login tests ---

func TestLogin_Success(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour)
	svc := &mockService{
		loginFn: func(_ context.Context, _ user.LoginRequest) (*user.LoginResponse, error) {
			return &user.LoginResponse{Token: "jwt.token.here", ExpiresAt: expiresAt}, nil
		},
	}
	h := user.NewHandler(svc)

	body := mustMarshal(t, map[string]string{"email": "test@test.com", "password": "secret123"})
	w, r := newRequest(t, http.MethodPost, "/api/auth/login", body)
	h.Login(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp user.LoginResponse
	mustDecode(t, w.Body.Bytes(), &resp)

	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	svc := &mockService{
		loginFn: func(_ context.Context, _ user.LoginRequest) (*user.LoginResponse, error) {
			return nil, user.ErrInvalidCredentials
		},
	}
	h := user.NewHandler(svc)

	body := mustMarshal(t, map[string]string{"email": "test@test.com", "password": "wrong"})
	w, r := newRequest(t, http.MethodPost, "/api/auth/login", body)
	h.Login(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogin_BadJSON(t *testing.T) {
	h := user.NewHandler(&mockService{})

	w, r := newRequest(t, http.MethodPost, "/api/auth/login", []byte(`{bad`))
	h.Login(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Helpers ---

func newRequest(t *testing.T, method, path string, body []byte) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return httptest.NewRecorder(), r
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func mustDecode(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode response: %v — raw: %s", err, data)
	}
}
