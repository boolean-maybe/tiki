package controller

import (
	"errors"
	"strings"

	"github.com/boolean-maybe/tiki/service"
)

// rejectionMessage extracts a clean user-facing message from an error.
// For RejectionError: returns just the rejection reasons.
// For other errors: unwraps to the root cause to strip wrapper prefixes
// like "failed to update tiki: failed to save tiki:".
func rejectionMessage(err error) string {
	var re *service.RejectionError
	if errors.As(err, &re) {
		reasons := make([]string, len(re.Rejections))
		for i, r := range re.Rejections {
			reasons[i] = r.Reason
		}
		return strings.Join(reasons, "; ")
	}
	for {
		inner := errors.Unwrap(err)
		if inner == nil {
			break
		}
		err = inner
	}
	return err.Error()
}
