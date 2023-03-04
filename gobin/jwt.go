package gobin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-jose/go-jose/v3/jwt"
)

var (
	ErrNoPermissions     = errors.New("no permissions provided")
	ErrUnknownPermission = func(p Permission) error {
		return fmt.Errorf("unknown permission: %s", p)
	}
	ErrPermissionDenied = func(p Permission) error {
		return fmt.Errorf("permission denied: %s", p)
	}
)

type Permission string

const (
	PermissionWrite  Permission = "write"
	PermissionDelete Permission = "delete"
	PermissionShare  Permission = "share"
)

func (p Permission) IsValid() bool {
	return p == PermissionWrite || p == PermissionDelete || p == PermissionShare
}

type Claims struct {
	jwt.Claims
	Permissions []Permission `json:"permissions"`
}

type claimsKey struct{}

var ClaimsKey = claimsKey{}

func (s *Server) JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := TokenFromHeader(r)

		var claims Claims
		if tokenString == "" {
			documentID := chi.URLParam(r, "documentID")
			claims = newClaims(documentID, nil)
		} else {
			token, err := jwt.ParseSigned(tokenString)
			if err != nil {
				s.error(w, r, err, http.StatusUnauthorized)
				return
			}

			if err = token.Claims([]byte(s.cfg.JWTSecret), &claims); err != nil {
				s.error(w, r, err, http.StatusUnauthorized)
				return
			}
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, &claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) GetClaims(r *http.Request) *Claims {
	return r.Context().Value(ClaimsKey).(*Claims)
}

func (s *Server) NewToken(documentID string, permissions []Permission) (string, error) {
	claims := newClaims(documentID, permissions)
	return jwt.Signed(s.signer).Claims(claims).CompactSerialize()
}

func newClaims(documentID string, permissions []Permission) Claims {
	return Claims{
		Claims: jwt.Claims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Subject:  documentID,
		},
		Permissions: permissions,
	}
}

func TokenFromHeader(r *http.Request) string {
	bearer := r.Header.Get("Authorization")
	if len(bearer) > 7 && strings.ToUpper(bearer[0:6]) == "BEARER" {
		return bearer[7:]
	}
	return ""
}
