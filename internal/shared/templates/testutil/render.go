// Package testutil provides shared helpers for render smoke tests of the
// zelo Templ component library.
package testutil

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/a-h/templ"
)

// RenderToString renders a Templ component and fails the test on any error.
func RenderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

// Component returns a simple Templ component that writes body.
func Component(body string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, body)
		return err
	})
}
