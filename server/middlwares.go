package server

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

	"github.com/topi314/gobin/v2/internal/ezhttp"
	"github.com/topi314/gobin/v2/internal/httperr"
)

const maxUnix = int(^int32(0)) * 1000

var (
	ErrNoPermissions     = errors.New("no permissions provided")
	ErrUnknownPermission = func(p string) error {
		return fmt.Errorf("unknown permission: %s", p)
	}
	ErrPermissionDenied = func(p string) error {
		return fmt.Errorf("permission denied: %s", p)
	}
)

func (s *Server) cacheKeyFunc(r *http.Request) uint64 {
	return stampede.BytesToHash([]byte(r.Method), []byte(chi.URLParam(r, "documentID")), []byte(chi.URLParam(r, "version")), []byte(r.URL.RawQuery))
}

func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set(ezhttp.HeaderCacheControl, "public, max-age=86400")
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set(ezhttp.HeaderCacheControl, "no-cache, no-store, must-revalidate")
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
			w.Header().Set(ezhttp.HeaderRateLimitLimit, strconv.Itoa(s.cfg.RateLimit.Requests))
			w.Header().Set(ezhttp.HeaderRateLimitRemaining, "0")
			w.Header().Set(ezhttp.HeaderRateLimitReset, strconv.Itoa(maxUnix))
			w.Header().Set(ezhttp.HeaderRetryAfter, strconv.Itoa(maxUnix-int(time.Now().UnixMilli())))
			w.WriteHeader(http.StatusTooManyRequests)
			s.error(w, r, httperr.TooManyRequests(ErrRateLimit))
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
		tokenString := r.Header.Get(ezhttp.HeaderAuthorization)
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
				s.error(w, r, httperr.Unauthorized(err))
				return
			}

			if err = token.Claims([]byte(s.cfg.JWTSecret), &claims); err != nil {
				s.error(w, r, httperr.Unauthorized(err))
				return
			}
		}

		next.ServeHTTP(w, SetClaims(r, claims))
	})
}
