package common

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DelayResponseWriter buffers the response and headers
// until it is explicitly transparently flushed or written to after committed.
type DelayResponseWriter struct {
	gin.ResponseWriter
	buffer     bytes.Buffer
	statusCode int
	headers    http.Header
	committed  bool
	isFlushed  bool // track actual flush to lower ResponseWriter

	// autoFlushCallback is called on each Write. If it returns true, FlushHeadersAndBuffer is called automatically.
	autoFlushCallback func(b []byte) bool
}

func NewDelayResponseWriter(w gin.ResponseWriter, autoFlushCallback func(b []byte) bool) *DelayResponseWriter {
	return &DelayResponseWriter{
		ResponseWriter:    w,
		headers:           make(http.Header),
		statusCode:        http.StatusOK, // default to 200
		autoFlushCallback: autoFlushCallback,
	}
}

// WriteHeader buffers the status code
func (w *DelayResponseWriter) WriteHeader(code int) {
	if w.committed {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	w.statusCode = code
}

// Write buffers the data if not committed, otherwise writes directly to the underlying ResponseWriter
func (w *DelayResponseWriter) Write(b []byte) (int, error) {
	if w.committed {
		return w.ResponseWriter.Write(b)
	}
	n, err := w.buffer.Write(b)
	if w.autoFlushCallback != nil {
		if w.autoFlushCallback(b) {
			w.FlushHeadersAndBuffer()
		} else if w.buffer.Len() > 2048 {
			// fallback flush if buffer gets too large to prevent memory issues for non-zero responses
			w.FlushHeadersAndBuffer()
		}
	}
	return n, err
}

// WriteString buffers the string if not committed
func (w *DelayResponseWriter) WriteString(s string) (int, error) {
	if w.committed {
		return w.ResponseWriter.WriteString(s)
	}
	n, err := w.buffer.WriteString(s)
	if w.autoFlushCallback != nil {
		if w.autoFlushCallback([]byte(s)) {
			w.FlushHeadersAndBuffer()
		} else if w.buffer.Len() > 2048 {
			w.FlushHeadersAndBuffer()
		}
	}
	return n, err
}

// FlushHeader forces to flush headers and status code to the underlying ResponseWriter
func (w *DelayResponseWriter) FlushHeader() {
	if !w.committed {
		// apply headers
		for k, v := range w.headers {
			for _, val := range v {
				w.ResponseWriter.Header().Add(k, val)
			}
		}
		w.ResponseWriter.WriteHeader(w.statusCode)
		w.committed = true
	}
}

// FlushHeadersAndBuffer flushes headers and all buffered data.
func (w *DelayResponseWriter) FlushHeadersAndBuffer() error {
	w.FlushHeader()
	if w.buffer.Len() > 0 {
		_, err := w.ResponseWriter.Write(w.buffer.Bytes())
		if err != nil {
			return err
		}
		w.buffer.Reset()
	}
	if !w.isFlushed {
		w.ResponseWriter.Flush()
		w.isFlushed = true
	}
	return nil
}

// Clear flushes headers conceptually but resets the buffer. Used when dropping response.
func (w *DelayResponseWriter) Clear() {
	w.buffer.Reset()
}

// Header returns the buffered headers if not committed, or the underlying headers
func (w *DelayResponseWriter) Header() http.Header {
	if w.committed {
		return w.ResponseWriter.Header()
	}
	return w.headers
}

// WriteHeaderNow forces to write the http header (status code + headers) immediately.
func (w *DelayResponseWriter) WriteHeaderNow() {
	w.FlushHeader()
	w.ResponseWriter.WriteHeaderNow()
}

// Status returns the buffered status code or the underlying one
func (w *DelayResponseWriter) Status() int {
	if w.committed {
		return w.ResponseWriter.Status()
	}
	return w.statusCode
}

// Size returns the buffered size if not committed
func (w *DelayResponseWriter) Size() int {
	if w.committed {
		return w.ResponseWriter.Size()
	}
	return w.buffer.Len()
}

// Written returns true if headers were flushed
func (w *DelayResponseWriter) Written() bool {
	return w.committed || w.ResponseWriter.Written()
}

// Flush implements the http.Flusher interface
func (w *DelayResponseWriter) Flush() {
	if w.committed {
		w.ResponseWriter.Flush()
		w.isFlushed = true
	}
}

// Pusher implements the http.Pusher interface
func (w *DelayResponseWriter) Pusher() http.Pusher {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher
	}
	return nil
}
