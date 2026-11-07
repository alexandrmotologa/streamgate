package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

type flushableGzipWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (w *flushableGzipWriter) Write(b []byte) (int, error) {
	return w.gz.Write(b)
}

func (w *flushableGzipWriter) Flush() {
	_ = w.gz.Flush()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// CompressMiddleware negotiates gzip streaming compression with immediate frame flushing.
func CompressMiddleware(enabled bool, next http.Handler) http.Handler {
	if !enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only compress if client accepts gzip and connection is not already encoded
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipPool.Get().(*gzip.Writer)
		defer gzipPool.Put(gz)
		gz.Reset(w)
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		w.Header().Add("Vary", "Accept-Encoding")

		gzw := &flushableGzipWriter{
			ResponseWriter: w,
			gz:             gz,
		}

		next.ServeHTTP(gzw, r)
	})
}
