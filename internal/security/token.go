package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid authentication token")

type TokenClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret []byte, ttl time.Duration) TokenManager {
	return TokenManager{secret: append([]byte(nil), secret...), ttl: ttl}
}

func (m TokenManager) Issue(userID, username string, now time.Time) (string, error) {
	claims := TokenClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m TokenManager) Parse(encoded string, now time.Time) (TokenClaims, error) {
	claims := TokenClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	token, err := parser.ParseWithClaims(encoded, &claims, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil || !token.Valid || claims.UserID == "" || claims.Username == "" {
		return TokenClaims{}, ErrInvalidToken
	}
	return claims, nil
}
