package utils

import (
	"fmt"
)

// WrapError provides a consistent way to wrap errors with a message for more context.
func WrapError(context string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}
