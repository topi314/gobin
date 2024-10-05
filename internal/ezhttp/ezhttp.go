package ezhttp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"
)

const (
	HeaderContentType        = "Content-Type"
	HeaderContentLength      = "Content-Length"
	HeaderContentDisposition = "Content-Disposition"
	HeaderUserAgent          = "User-Agent"
	HeaderAuthorization      = "Authorization"
	HeaderLanguage           = "Language"
	HeaderRateLimitLimit     = "X-RateLimit-Limit"
	HeaderRateLimitRemaining = "X-RateLimit-Remaining"
	HeaderRateLimitReset     = "X-RateLimit-Reset"
	HeaderRetryAfter         = "Retry-After"
	HeaderCacheControl       = "Cache-Control"
)

const (
	DefaultContentTyp = "application/octet-stream"
	ContentTypeCSS    = "text/css; charset=UTF-8"
	ContentTypeHTML   = "text/html; charset=UTF-8"
	ContentTypeText   = "text/plain; charset=UTF-8"
	ContentTypeSVG    = "image/svg+xml"
	ContentTypePNG    = "image/png"
	ContentTypeJSON   = "application/json"
)

type ErrorResponse struct {
	Message   string `json:"message"`
	Status    int    `json:"status"`
	Path      string `json:"path"`
	RequestID string `json:"request_id"`
}

type Reader interface {
	io.Reader
	Headers() http.Header
}

func NewHeaderReader(r io.Reader, headers http.Header) Reader {
	return &reader{
		Reader:  r,
		headers: headers,
	}
}

type reader struct {
	io.Reader
	headers http.Header
}

func (r *reader) Headers() http.Header {
	return r.headers
}

var defaultClient = &http.Client{
	Timeout: 10 * time.Second,
}

func Do(method string, path string, token string, body io.Reader) (*http.Response, error) {
	gobinServer := viper.GetString("server")
	rq, err := http.NewRequest(method, gobinServer+path, body)
	if err != nil {
		return nil, err
	}
	if r, ok := body.(Reader); ok {
		rq.Header = r.Headers()
	}

	if token != "" {
		rq.Header.Set(HeaderAuthorization, "Bearer "+token)
	}
	return defaultClient.Do(rq)
}

func Get(path string) (*http.Response, error) {
	return Do(http.MethodGet, path, "", nil)
}

func Post(path string, body io.Reader) (*http.Response, error) {
	return Do(http.MethodPost, path, "", body)
}

func PostToken(path string, token string, body io.Reader) (*http.Response, error) {
	return Do(http.MethodPost, path, token, body)
}

func Patch(path string, token string, body io.Reader) (*http.Response, error) {
	return Do(http.MethodPatch, path, token, body)
}

func Delete(path string, token string) (*http.Response, error) {
	return Do(http.MethodDelete, path, token, nil)
}

func ProcessBody(method string, rs *http.Response, body any) error {
	if rs.StatusCode >= http.StatusOK && rs.StatusCode < http.StatusMultipleChoices {
		if err := json.NewDecoder(rs.Body).Decode(body); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
		return nil
	}
	var errRs ErrorResponse
	if err := json.NewDecoder(rs.Body).Decode(&errRs); err != nil {
		return fmt.Errorf("failed to decode error response: %w", err)
	}
	return fmt.Errorf("failed to %s: %s", method, errRs.Message)
}
