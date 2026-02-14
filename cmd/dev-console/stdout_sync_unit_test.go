package main

import (
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestIsIgnorableStdoutSyncError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "einval", err: syscall.EINVAL, want: true},
		{name: "ebadf", err: syscall.EBADF, want: true},
		{
			name: "pathErrorEbadf",
			err: &os.PathError{
				Op:   "sync",
				Path: "/dev/stdout",
				Err:  syscall.EBADF,
			},
			want: true,
		},
		{name: "other", err: fmt.Errorf("boom"), want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isIgnorableStdoutSyncError(tc.err)
			if got != tc.want {
				t.Fatalf("isIgnorableStdoutSyncError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
