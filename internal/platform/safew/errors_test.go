package safew

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsPermanentChatError(t *testing.T) {
	perm := []error{
		&APIError{HTTPStatus: 400, Code: 400, Description: "CHANNEL_PRIVATE"},
		fmt.Errorf("wrap: %w", &APIError{HTTPStatus: 403, Code: 403, Description: "x"}),
		errors.New("Bad Request: chat not found"),
		errors.New("Forbidden: bot was kicked from the group chat"),
	}
	for _, e := range perm {
		if !IsPermanentChatError(e) {
			t.Errorf("expected permanent: %v", e)
		}
	}
	for _, e := range []error{nil, errors.New("connection reset"), &APIError{HTTPStatus: 429, Code: 429, Description: "Too Many Requests"}, &APIError{HTTPStatus: 502, Description: "Bad Gateway"}} {
		if IsPermanentChatError(e) {
			t.Errorf("expected transient: %v", e)
		}
	}
}
