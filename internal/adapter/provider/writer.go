package provider

import (
	"errors"
	"io"
)

// IgnoreErrorWriter wraps an io.Writer and ignores io.ErrClosedPipe,
// always reporting a successful full write when it occurs. This is useful in io.TeeReader setups
// where a broken pipe in the secondary branch shouldn't abort the primary reader.
// Other errors are passed through.
type IgnoreErrorWriter struct {
	W io.Writer
}

func (i IgnoreErrorWriter) Write(p []byte) (n int, err error) {
	n, err = i.W.Write(p)
	if err != nil && errors.Is(err, io.ErrClosedPipe) {
		return len(p), nil
	}
	return n, err
}
