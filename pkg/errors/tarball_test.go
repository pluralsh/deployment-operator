package errors

import (
	"fmt"
	"testing"
)

func TestIsDigestMismatchError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "not a digest mismatch", err: fmt.Errorf("not a digest mismatch"), want: false},
		{name: "digest mismatch", err: NewDigestMismatchError("expected", "actual"), want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := IsDigestMismatchError(test.err)
			if got != test.want {
				t.Errorf("IsDigestMismatchError(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}
