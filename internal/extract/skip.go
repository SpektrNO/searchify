package extract

import "fmt"

// SkipError means the file should be skipped without counting as an index error
// (e.g. image when OCR is disabled).
type SkipError struct {
	Message string
}

func (e *SkipError) Error() string {
	if e == nil || e.Message == "" {
		return "skipped"
	}
	return e.Message
}

func Skip(format string, args ...any) error {
	return &SkipError{Message: fmt.Sprintf(format, args...)}
}
