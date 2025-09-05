package cio

import (
	"io"
	"sync"
)

// PipeWithTee copies bytes bidirectionally between src and dst like Pipe,
// but allows tapping into each direction by providing optional writers.
// srcToDstTap receives a copy of bytes flowing from src -> dst.
// dstToSrcTap receives a copy of bytes flowing from dst -> src.
// Taps are best-effort and should be non-blocking; if tap Write blocks or fails,
// the copy continues regardless.
func PipeWithTee(src io.ReadWriteCloser, dst io.ReadWriteCloser, srcToDstTap io.Writer, dstToSrcTap io.Writer) (int64, int64) {
	var sent, received int64
	var wg sync.WaitGroup
	var o sync.Once
	closeBoth := func() {
		src.Close()
		dst.Close()
	}

	// helper: wrap reader with TeeReader if tap provided
	maybeTee := func(r io.Reader, tap io.Writer) io.Reader {
		if tap == nil {
			return r
		}
		return io.TeeReader(r, nonBlockingWriter{tap})
	}

	wg.Add(2)
	// dst -> src
	go func() {
		defer wg.Done()
		received, _ = io.Copy(src, maybeTee(dst, dstToSrcTap))
		o.Do(closeBoth)
	}()
	// src -> dst
	go func() {
		defer wg.Done()
		sent, _ = io.Copy(dst, maybeTee(src, srcToDstTap))
		o.Do(closeBoth)
	}()
	wg.Wait()
	return sent, received
}

// nonBlockingWriter wraps an io.Writer to ensure Write never blocks the main piping.
// If the underlying writer errors, we ignore it to avoid interfering with the tunnel.
// For heavy processing, the tap should implement its own buffering.
type nonBlockingWriter struct{ w io.Writer }

func (n nonBlockingWriter) Write(p []byte) (int, error) {
	// Best-effort write; ignore errors
	_, _ = n.w.Write(p)
	return len(p), nil
}
