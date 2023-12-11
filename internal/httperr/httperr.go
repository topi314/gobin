package httperr

import (
	"errors"
	"net/http"
)

type Error struct {
	Err      error
	Status   int
	Location string
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return errors.Is(e.Err, target)
	}
	return t.Status == e.Status && errors.Is(t.Err, e.Err)
}

func (e *Error) As(target any) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	*t = *e
	return true
}

func New(err error, status int) error {
	return &Error{
		Err:    err,
		Status: status,
	}
}

func Found(location string) error {
	return &Error{
		Status:   http.StatusFound,
		Location: location,
	}
}

func NotFound(err error) error {
	return New(err, http.StatusNotFound)
}

func BadRequest(err error) error {
	return New(err, http.StatusBadRequest)
}

func Unauthorized(err error) error {
	return New(err, http.StatusUnauthorized)
}

func Forbidden(err error) error {
	return New(err, http.StatusForbidden)
}

func TooManyRequests(err error) error {
	return New(err, http.StatusTooManyRequests)
}

func InternalServerError(err error) error {
	return New(err, http.StatusInternalServerError)
}
