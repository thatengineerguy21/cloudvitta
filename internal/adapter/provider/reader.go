package provider

import "io"

// CountingReader wraps an io.Reader and tracks the total count of bytes read.
type CountingReader struct {
	r io.Reader
	n int64
}

// NewCountingReader wraps r with byte counting.
func NewCountingReader(r io.Reader) *CountingReader {
	return &CountingReader{r: r}
}

func (c *CountingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// BytesRead returns the total number of bytes read through the reader.
func (c *CountingReader) BytesRead() int64 {
	return c.n
}
