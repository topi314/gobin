package gobin

import (
	"context"
	"net/http"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
)

type Permission string

const (
	PermissionWrite   Permission = "write"
	PermissionDelete  Permission = "delete"
	PermissionShare   Permission = "share"
	PermissionWebhook Permission = "webhook"
)

var AllPermissions = []Permission{
	PermissionWrite,
	PermissionDelete,
	PermissionShare,
	PermissionWebhook,
}

func (p Permission) IsValid() bool {
	return p == PermissionWrite || p == PermissionDelete || p == PermissionShare || p == PermissionWebhook
}

type Claims struct {
	jwt.Claims
	Permissions []Permission `json:"permissions"`
}

type claimsKey struct{}

var claimsContextKey = claimsKey{}

func GetClaims(r *http.Request) Claims {
	return r.Context().Value(claimsContextKey).(Claims)
}

func SetClaims(r *http.Request, claims Claims) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), claimsContextKey, claims))
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

func EmptyClaims(documentID string) Claims {
	return newClaims(documentID, nil)
}
