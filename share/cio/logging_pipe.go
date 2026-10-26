package cio

import (
	"io"
	"sync"
)

// Logging reader/writes

type LoggingReadWriteCloser struct {
	rw     io.ReadWriteCloser
	logger *Logger
	prefix string
}

func NewLoggingReadWriteCloser(rw io.ReadWriteCloser, logger *Logger, prefix string) *LoggingReadWriteCloser {
	return &LoggingReadWriteCloser{rw: rw, logger: logger, prefix: prefix}
}

func (l *LoggingReadWriteCloser) Read(p []byte) (n int, err error) {
	n, err = l.rw.Read(p)
	if n > 0 {
		l.logger.Infof("\n================== %s: Read ==================\n%s", l.prefix, formatOutput(p[:n]))
	}
	return n, err
}

func (l *LoggingReadWriteCloser) Write(p []byte) (n int, err error) {
	n, err = l.rw.Write(p)
	if n > 0 {
		l.logger.Infof("\n================== %s: Write ==================\n%s", l.prefix, formatOutput(p[:n]))
	}
	return n, err
}

func (l *LoggingReadWriteCloser) Close() error {
	return l.rw.Close()
}

func formatOutput(data []byte) string {
	const maxLogLength = 1024
	if len(data) > maxLogLength {
		data = data[:maxLogLength]
	}
	return string(data)
}

func LoggingPipe(src io.ReadWriteCloser, dst io.ReadWriteCloser) (int64, int64) {
	var sent, received int64
	var wg sync.WaitGroup
	var o sync.Once
	close := func() {
		_ = src.Close()
		_ = dst.Close()
	}
	wg.Add(2)
	go func() {
		received, _ = io.Copy(src, dst)
		o.Do(close)
		wg.Done()
	}()
	go func() {
		sent, _ = io.Copy(dst, src)
		o.Do(close)
		wg.Done()
	}()
	wg.Wait()
	return sent, received
}
