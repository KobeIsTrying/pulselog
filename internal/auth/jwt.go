package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func NewIssuer(secret string, ttl time.Duration) (*Issuer, error) {
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Issuer{secret: []byte(secret), ttl: ttl}, nil
}

func (i *Issuer) Issue(userID uuid.UUID, email string, now time.Time) (token, jti string, exp time.Time, err error) {
	jti = uuid.NewString()
	exp = now.Add(i.ttl)
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = t.SignedString(i.secret)
	return token, jti, exp, err
}

func (i *Issuer) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return i.secret, nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrUnauthorized
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok {
		return nil, ErrUnauthorized
	}
	return claims, nil
}

type DenyStore interface {
	Deny(ctx context.Context, jti string, ttl time.Duration) error
	Denied(ctx context.Context, jti string) (bool, error)
}

type RedisDeny struct {
	Set    func(ctx context.Context, key string, ttl time.Duration) error
	Exists func(ctx context.Context, key string) (bool, error)
}

func denyKey(jti string) string { return "jwt:deny:" + jti }

func (d RedisDeny) Deny(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return d.Set(ctx, denyKey(jti), ttl)
}

func (d RedisDeny) Denied(ctx context.Context, jti string) (bool, error) {
	return d.Exists(ctx, denyKey(jti))
}

type MemoryDeny struct {
	items map[string]time.Time
}

func NewMemoryDeny() *MemoryDeny {
	return &MemoryDeny{items: map[string]time.Time{}}
}

func (m *MemoryDeny) Deny(_ context.Context, jti string, ttl time.Duration) error {
	m.items[jti] = time.Now().Add(ttl)
	return nil
}

func (m *MemoryDeny) Denied(_ context.Context, jti string) (bool, error) {
	exp, ok := m.items[jti]
	if !ok {
		return false, nil
	}
	if time.Now().After(exp) {
		delete(m.items, jti)
		return false, nil
	}
	return true, nil
}
