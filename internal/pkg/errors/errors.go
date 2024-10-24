package errors

import (
	"errors"
	"fmt"
	"unsafe"
)

// Wrapf convinient function to wrap errors
func Wrapf(err error, text string, args ...interface{}) error {
	return fmt.Errorf(text+": %w", append(args, err)...)
}

// New is equivalent of errors.New
func New(text string) error {
	return errors.New(text)
}

// Is is equivalent of errrors.Is
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// Unwrap is equivalent of errrors.Is
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// Join is an equivalent of errors.Join however it doesn't add new line when printing errors
func Join(errs ...error) error {
	n := 0
	for _, err := range errs {
		if err != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	e := &joinError{
		errs: make([]error, 0, n),
	}
	for _, err := range errs {
		if err != nil {
			e.errs = append(e.errs, err)
		}
	}
	return e
}

type joinError struct {
	errs []error
}

func (e *joinError) Error() string {
	// Since Join returns nil if every value in errs is nil,
	// e.errs cannot be empty.
	if len(e.errs) == 1 {
		return e.errs[0].Error()
	}

	b := []byte(e.errs[0].Error())
	for _, err := range e.errs[1:] {
		b = append(b, ':', ' ')
		b = append(b, err.Error()...)
	}
	// At this point, b has at least one byte '\n'.
	return unsafe.String(&b[0], len(b))
}

func (e *joinError) Unwrap() []error {
	return e.errs
}
