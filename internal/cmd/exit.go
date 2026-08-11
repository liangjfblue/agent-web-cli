package cmd

import (
	"errors"

	"github.com/agent/web-cli/internal/ipc"
	"github.com/agent/web-cli/internal/proto"
)

const (
	ExitInvalidArguments = 2
	ExitAuthRequired     = 10
	ExitLoginTimeout     = 11
	ExitHostUnavailable  = 20
	ExitProfileRequired  = 21
	ExitProfileNotFound  = 22
	ExitExtensionFailure = 30
)

// ExitError lets commands expose a stable process contract to business CLIs.
type ExitError struct {
	Code   int
	Err    error
	Silent bool
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

func NewExitError(code int, err error) *ExitError {
	return &ExitError{Code: code, Err: err}
}

func SilentExitError(code int, err error) *ExitError {
	return &ExitError{Code: code, Err: err, Silent: true}
}

func ExitCode(err error) int {
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	var se *ipc.ErrSetup
	if errors.As(err, &se) {
		return ExitHostUnavailable
	}
	var wire *proto.WireError
	if errors.As(err, &wire) {
		return ExitExtensionFailure
	}
	return 1
}

func IsSilent(err error) bool {
	var ee *ExitError
	return errors.As(err, &ee) && ee.Silent
}
