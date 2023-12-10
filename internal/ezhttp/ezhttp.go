package ezhttp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"
	"github.com/topi314/gobin/v2/gobin"
)

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
	server := viper.GetString("server")
	rq, err := http.NewRequest(method, server+path, body)
	if err != nil {
		return nil, err
	}
	if r, ok := body.(Reader); ok {
		rq.Header = r.Headers()
	}
	if err != nil {
		return nil, err
	}
	if token != "" {
		rq.Header.Set("Authorization", "Bearer "+token)
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
	if rs.StatusCode >= 200 && rs.StatusCode < 300 {
		if err := json.NewDecoder(rs.Body).Decode(body); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
		return nil
	}
	var errRs gobin.ErrorResponse
	if err := json.NewDecoder(rs.Body).Decode(&errRs); err != nil {
		return fmt.Errorf("failed to decode error response: %w", err)
	}
	return fmt.Errorf("failed to %s: %s", method, errRs.Message)
}
