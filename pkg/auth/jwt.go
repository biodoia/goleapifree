package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// ErrInvalidToken indica che il token non è valido
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken indica che il token è scaduto
	ErrExpiredToken = errors.New("token expired")
	// ErrInvalidClaims indica che i claims non sono validi
	ErrInvalidClaims = errors.New("invalid claims")
)

// JWTConfig configurazione JWT
type JWTConfig struct {
	SecretKey       string
	Issuer          string
	AccessDuration  time.Duration
	RefreshDuration time.Duration
}

// Claims rappresenta i claims JWT
type Claims struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	ApiKeyID string `json:"api_key_id,omitempty"`
	jwt.RegisteredClaims
}

// JWTManager gestisce la creazione e validazione di token JWT
type JWTManager struct {
	config JWTConfig
}

// NewJWTManager crea un nuovo JWT manager
func NewJWTManager(config JWTConfig) *JWTManager {
	// Set defaults if not provided
	if config.AccessDuration == 0 {
		config.AccessDuration = 15 * time.Minute
	}
	if config.RefreshDuration == 0 {
		config.RefreshDuration = 7 * 24 * time.Hour
	}
	if config.Issuer == "" {
		config.Issuer = "goleapai-gateway"
	}

	return &JWTManager{
		config: config,
	}
}

// GenerateAccessToken genera un access token JWT
func (m *JWTManager) GenerateAccessToken(userID, email, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.AccessDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.config.Issuer,
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.SecretKey))
}

// GenerateRefreshToken genera un refresh token JWT
func (m *JWTManager) GenerateRefreshToken(userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(m.config.RefreshDuration)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Issuer:    m.config.Issuer,
		Subject:   userID,
		ID:        uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.SecretKey))
}

// ValidateToken valida un token JWT e restituisce i claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verifica che il signing method sia corretto
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(m.config.SecretKey), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

// ValidateRefreshToken valida un refresh token
func (m *JWTManager) ValidateRefreshToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(m.config.SecretKey), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrExpiredToken
		}
		return "", ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", ErrInvalidClaims
	}

	return claims.Subject, nil
}

// RefreshAccessToken genera un nuovo access token da un refresh token valido
func (m *JWTManager) RefreshAccessToken(refreshToken, email, role string) (string, error) {
	userID, err := m.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}

	return m.GenerateAccessToken(userID, email, role)
}
