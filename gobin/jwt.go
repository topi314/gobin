package gobin

import (
	"context"
	"net/http"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/topi314/gobin/internal/flags"
)

type Permissions int

const (
	PermissionWrite Permissions = 1 << iota
	PermissionDelete
	PermissionShare
	PermissionWebhook
)

var AllPermissions = PermissionWrite |
	PermissionDelete |
	PermissionShare |
	PermissionWebhook

var AllStringPermissions = []string{"write", "delete", "share", "webhook"}

type Claims struct {
	jwt.Claims
	Permissions Permissions `json:"pms"`
}

type claimsKey struct{}

var claimsContextKey = claimsKey{}

func GetClaims(r *http.Request) Claims {
	return r.Context().Value(claimsContextKey).(Claims)
}

func SetClaims(r *http.Request, claims Claims) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), claimsContextKey, claims))
}

func (s *Server) NewToken(documentID string, permissions Permissions) (string, error) {
	claims := newClaims(documentID, permissions)
	return jwt.Signed(s.signer).Claims(claims).CompactSerialize()
}

func newClaims(documentID string, permissions Permissions) Claims {
	return Claims{
		Claims: jwt.Claims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Subject:  documentID,
		},
		Permissions: permissions,
	}
}

func EmptyClaims(documentID string) Claims {
	return newClaims(documentID, 0)
}

func parsePermissions(perms Permissions, stringPerms []string) (Permissions, error) {
	var permissions Permissions
	for _, perm := range stringPerms {
		switch perm {
		case "write":
			if flags.Misses(perms, PermissionWrite) {
				return 0, ErrPermissionDenied(perm)
			}
			permissions = flags.Add(permissions, PermissionWrite)
		case "delete":
			if flags.Misses(perms, PermissionDelete) {
				return 0, ErrPermissionDenied(perm)
			}
			permissions = flags.Add(permissions, PermissionDelete)
		case "share":
			if flags.Misses(perms, PermissionShare) {
				return 0, ErrPermissionDenied(perm)
			}
			permissions = flags.Add(permissions, PermissionShare)
		case "webhook":
			if flags.Misses(perms, PermissionWebhook) {
				return 0, ErrPermissionDenied(perm)
			}
			permissions = flags.Add(permissions, PermissionWebhook)
		}
	}
	return permissions, nil
}
