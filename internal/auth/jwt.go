package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID        uint   `json:"uid"`
	Email         string `json:"email"`
	IsAdmin       bool   `json:"admin"`
	IsStaff       bool   `json:"staff"`
	TokenVersion  int    `json:"tver"`
	ID            string `json:"jti"`
	Authenticated bool   `json:"auth"`
	jwt.RegisteredClaims
}

type JWTAuth struct {
	secret    string
	issuer    string
	expiresIn time.Duration
}

type JWTOption func(*JWTAuth)

func WithIssuer(issuer string) JWTOption {
	return func(j *JWTAuth) {
		j.issuer = issuer
	}
}

func WithExpiresIn(d time.Duration) JWTOption {
	return func(j *JWTAuth) {
		j.expiresIn = d
	}
}

func NewJWTAuth(secret string, opts ...JWTOption) *JWTAuth {
	j := &JWTAuth{
		secret:    secret,
		issuer:    "xboard",
		expiresIn: 24 * time.Hour,
	}
	for _, opt := range opts {
		opt(j)
	}
	return j
}

func (j *JWTAuth) GenerateToken(userID uint, email string, isAdmin, isStaff bool, tokenVersion int) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:        userID,
		Email:         email,
		IsAdmin:       isAdmin,
		IsStaff:       isStaff,
		Authenticated: true,
		TokenVersion:  tokenVersion,
		ID:            uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.expiresIn)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secret))
}

func (j *JWTAuth) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
