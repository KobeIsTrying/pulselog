package metrics

import "testing"

func TestRegisterDoesNotPanic(t *testing.T) {
	Register()
	Register()
}
