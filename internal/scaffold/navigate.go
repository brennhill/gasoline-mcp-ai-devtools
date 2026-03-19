// navigate.go — Auto-navigate to dev server after it's ready.

package scaffold

import (
	"context"
	"fmt"
)

// NavigateFunc is a function that navigates the browser to a URL.
// This is a dependency injection point for the MCP interact(navigate) call.
type NavigateFunc func(ctx context.Context, url string) error

// AutoNavigate navigates the browser to the dev server on the given port.
func AutoNavigate(ctx context.Context, port int, navigate NavigateFunc) error {
	url := fmt.Sprintf("http://localhost:%d", port)
	return navigate(ctx, url)
}
