package main

import (
	"testing"
)

func setupV4TestServer(t *testing.T) *V4Server {
	t.Helper()
	return NewV4Server()
}
