package entity

import "fmt"

// ValidationError names the field that failed and why, so callers can surface it verbatim.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s %s", e.Field, e.Message)
}
