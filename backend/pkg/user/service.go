package user

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost     = 12
	tokenTTL       = 24 * time.Hour
	minPasswordLen = 6
)

var emailRegexp = regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`)

// ErrInvalidCredentials is returned when email/password do not match.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrInvalidInput is returned when the request payload fails validation.
var ErrInvalidInput = errors.New("invalid input")

// Service is the interface for user business logic.
type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
}

// jwtClaims are the claims stored inside the JWT token.
type jwtClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

type service struct {
	repo      Repository
	jwtSecret []byte
}

// NewService creates a new user Service.
func NewService(repo Repository, jwtSecret string) Service {
	return &service{repo: repo, jwtSecret: []byte(jwtSecret)}
}

// Register validates input, hashes the password, and creates a user + wallet atomically.
func (s *service) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	if err := validateRegister(req); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u, err := s.repo.CreateUserWithWallet(ctx, req.Email, string(hash))
	if err != nil {
		return nil, err // ErrEmailTaken propagates as-is
	}

	return &RegisterResponse{
		ID:        u.ID,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}, nil
}

// Login verifies credentials and returns a signed JWT.
func (s *service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	u, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if u == nil {
		return nil, ErrInvalidCredentials
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, expiresAt, err := s.generateToken(u.ID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &LoginResponse{Token: token, ExpiresAt: expiresAt}, nil
}

// generateToken signs a JWT containing user_id with a 24h expiry.
func (s *service) generateToken(userID int64) (string, time.Time, error) {
	expiresAt := time.Now().Add(tokenTTL)
	claims := jwtClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	return signed, expiresAt, err
}

// validateRegister checks email format and password length.
func validateRegister(req RegisterRequest) error {
	if !emailRegexp.MatchString(req.Email) {
		return fmt.Errorf("%w: invalid email format", ErrInvalidInput)
	}
	if len(req.Password) < minPasswordLen {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordLen)
	}
	return nil
}
