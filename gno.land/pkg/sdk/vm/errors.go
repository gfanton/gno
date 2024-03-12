package vm

import (
	"errors"
	"fmt"
)

// for convenience:
type abciError struct{}

func (abciError) AssertABCIError() {}

// declare all script errors.
// NOTE: these are meant to be used in conjunction with pkgs/errors.
type (
	InvalidPkgPathError struct{ abciError }
	InvalidStmtError    struct{ abciError }
	InvalidExprError    struct{ abciError }
)

func (e InvalidPkgPathError) Error() string { return "invalid package path" }
func (e InvalidStmtError) Error() string    { return "invalid statement" }
func (e InvalidExprError) Error() string    { return "invalid expression" }

func ErrInvalidPkgPath(msg string) error {
	return fmt.Errorf("%s: %w", msg, InvalidPkgPathError{})
}

func ErrInvalidStmt(msg string) error {
	return errors.Wrap("%s: %w", msg, InvalidStmtError{})
}

func ErrInvalidExpr(msg string) error {
	return errors.Wrap("%s: %w", msg, InvalidExprError{})
}
