package logwriter

import (
	"fmt"
	"io"
	"time"
)

type LogWriter struct {
	tz     *time.Location
	format string
}

func (w *LogWriter) Write(data []byte) (int, error) {
	return fmt.Printf("%v %v", time.Now().In(w.tz).Format(w.format), string(data))
}

func New(tz *time.Location, format string) io.Writer {
	return &LogWriter{
		tz:     tz,
		format: format,
	}
}

func Default() io.Writer {
	return New(time.UTC, time.RFC3339)
}
