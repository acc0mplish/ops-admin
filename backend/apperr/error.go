package apperr

import "errors"

// Error carries a stable API error code and optional interpolation parameters.
// The wrapped cause is retained for server-side diagnostics but is not returned
// to clients by the HTTP response layer.
type Error struct {
	Code   string
	Params map[string]any
	Cause  error
}

func New(code string, params map[string]any) *Error {
	if params == nil {
		params = map[string]any{}
	}
	return &Error{Code: code, Params: params}
}

func Wrap(code string, cause error, params map[string]any) *Error {
	if params == nil {
		params = map[string]any{}
	}
	return &Error{Code: code, Params: params, Cause: cause}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return e.Code + ": " + e.Cause.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func Extract(err error) (string, map[string]any, bool) {
	var target *Error
	if !errors.As(err, &target) || target == nil || target.Code == "" {
		return "", nil, false
	}
	return target.Code, target.Params, true
}
