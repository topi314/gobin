package httprate

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/topi314/gobin/v3/internal/ezhttp"
)

func NewRateLimiter(requestLimit int, windowLength time.Duration, onRequestLimit http.HandlerFunc) *RateLimiter {
	c := &counter{
		counters:     make(map[uint64]*count),
		windowLength: windowLength,
		requestLimit: requestLimit,
	}

	go c.Cleanup()

	return &RateLimiter{
		requestLimit:   requestLimit,
		limitCounter:   c,
		onRequestLimit: onRequestLimit,
	}
}

type RateLimiter struct {
	requestLimit   int
	limitCounter   *counter
	onRequestLimit http.HandlerFunc
	mu             sync.Mutex
}

func (l *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := getKey(r)

		l.mu.Lock()
		ok, remaining, reset := l.limitCounter.Try(key)
		w.Header().Set(ezhttp.HeaderRateLimitLimit, strconv.Itoa(l.requestLimit))
		w.Header().Set(ezhttp.HeaderRateLimitRemaining, strconv.Itoa(remaining))
		w.Header().Set(ezhttp.HeaderRateLimitReset, strconv.FormatInt(reset.Unix(), 10))

		if !ok {
			w.Header().Set(ezhttp.HeaderRetryAfter, strconv.FormatInt(int64(math.Ceil(time.Until(reset).Seconds())), 10))
			l.onRequestLimit(w, r)
			l.mu.Unlock()
			return
		}
		l.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func getKey(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return canonicalizeIP(ip) + ":" + r.URL.Path
}

// canonicalizeIP returns a form of ip suitable for comparison to other IPs.
// For IPv4 addresses, this is simply the whole string.
// For IPv6 addresses, this is the /64 prefix.
func canonicalizeIP(ip string) string {
	isIPv6 := false
	// This is how net.ParseIP decides if an address is IPv6
	// https://cs.opensource.google/go/go/+/refs/tags/go1.17.7:src/net/ip.go;l=704
	for i := 0; !isIPv6 && i < len(ip); i++ {
		switch ip[i] {
		case '.':
			// IPv4
			return ip
		case ':':
			// IPv6
			isIPv6 = true
		}
	}
	if !isIPv6 {
		// Not an IP address at all
		return ip
	}

	ipv6 := net.ParseIP(ip)
	if ipv6 == nil {
		return ip
	}

	return ipv6.Mask(net.CIDRMask(64, 128)).String()
}
