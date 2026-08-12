package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken issues an access token with the given ttl (Stage 26A2:
// previously a hardcoded 24h package const — now the caller's
// responsibility, sourced from config.Config.JWTAccessTokenTTLMinutes via
// Service, so it's configurable without a code change).
func GenerateToken(secret string, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID.String(),
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken verifies signature, algorithm, and expiration before ever
// returning claims to a caller — RequireAuth (middleware.go) only ever
// reads UserID/Role from this function's return value, never from a raw
// decode, so every identity this backend trusts has passed all three
// checks below (Stage 26A2 hardening; behavior for a token that was
// already valid before this change is unchanged — only previously-invalid
// tokens that happened to slip past the looser checks are newly rejected).
//
//   - jwt.WithValidMethods pins acceptance to exactly "HS256" — the single
//     algorithm GenerateToken ever issues — checked by the parser itself
//     before the keyfunc even runs. This is the library's own recommended
//     defense against algorithm-confusion attacks (see its doc comment,
//     which cites the specific published vulnerability class this
//     prevents: a token claiming "alg: none", or an asymmetric algorithm
//     whose public key gets misused as an HMAC secret, is rejected
//     outright rather than reaching signature verification at all).
//   - jwt.WithExpirationRequired makes the "exp" claim mandatory. By the
//     library's own default, a token with no "exp" claim at all is
//     treated as never-expiring — every token GenerateToken issues always
//     sets ExpiresAt, so this changes nothing for legitimate tokens, and
//     closes off a class of hand-crafted token that omits "exp" entirely
//     to bypass expiration.
//   - Signature verification itself is the library's core behavior,
//     unchanged: the keyfunc below only ever returns the server's own
//     secret, so any signature not produced with that exact secret fails
//     regardless of the two options above.
func ParseToken(secret, tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
