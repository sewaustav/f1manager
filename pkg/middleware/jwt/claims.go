package jwt

import (
	"errors"
	"strconv"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int64
}

func (m *JWTAuthMiddleware) verifyToken(tokenStr string) (*Claims, error) {
	if m.issuer == "" || m.audience == "" {
		m.logger.Error("middleware configuration error: missing issuer or audience")
		return nil, errors.New("auth middleware is not properly configured")
	}

	registered := &jwtlib.RegisteredClaims{}

	token, err := jwtlib.ParseWithClaims(
		tokenStr,
		registered,
		func(t *jwtlib.Token) (any, error) {
			if _, ok := t.Method.(*jwtlib.SigningMethodRSA); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return m.publicKey, nil
		},
		jwtlib.WithIssuer(m.issuer),
		jwtlib.WithAudience(m.audience),
		jwtlib.WithValidMethods([]string{"RS256"}),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	id, err := strconv.ParseInt(registered.Subject, 10, 64)
	if err != nil {
		return nil, errors.New("failed to parse user id")
	}
	if id <= 0 {
		return nil, errors.New("invalid user id in token")
	}

	return &Claims{UserID: id}, nil
}
