package gobin

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/stampede"
	"github.com/go-jose/go-jose/v3/jwt"
)

const maxUnix = int(^int32(0))

var (
	ErrNoPermissions     = errors.New("no permissions provided")
	ErrUnknownPermission = func(p Permission) error {
		return fmt.Errorf("unknown permission: %s", p)
	}
	ErrPermissionDenied = func(p Permission) error {
		return fmt.Errorf("permission denied: %s", p)
	}
)

func (s *Server) cacheKeyFunc(r *http.Request) uint64 {
	return stampede.BytesToHash([]byte(r.Method), []byte(chi.URLParam(r, "documentID")), []byte(chi.URLParam(r, "version")), []byte(r.URL.RawQuery))
}

func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=86400")
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply rate limiting to POST, PATCH, and DELETE requests
		if r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
			next.ServeHTTP(w, r)
			return
		}
		remoteAddr := strings.SplitN(r.RemoteAddr, ":", 2)[0]
		// Filter whitelisted IPs
		if slices.Contains(s.cfg.RateLimit.Whitelist, remoteAddr) {
			next.ServeHTTP(w, r)
			return
		}
		// Filter blacklisted IPs
		if slices.Contains(s.cfg.RateLimit.Blacklist, remoteAddr) {
			retryAfter := maxUnix - int(time.Now().Unix())
			w.Header().Set("X-RateLimit-Limit", "0")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.Itoa(maxUnix))
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			s.rateLimit(w, r)
			return
		}
		if s.rateLimitHandler == nil {
			next.ServeHTTP(w, r)
			return
		}
		s.rateLimitHandler(next).ServeHTTP(w, r)
	})
}

func (s *Server) JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if len(tokenString) > 7 && strings.ToUpper(tokenString[0:6]) == "BEARER" {
			tokenString = tokenString[7:]
		}

		var claims Claims
		if tokenString == "" {
			documentID := chi.URLParam(r, "documentID")
			claims = EmptyClaims(documentID)
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

		next.ServeHTTP(w, SetClaims(r, claims))
	})
}
