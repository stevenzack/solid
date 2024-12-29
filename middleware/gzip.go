package middleware

import (
	"compress/gzip"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func GZip() (func(next http.HandlerFunc) http.HandlerFunc, bool) {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if gzipDefaultSkipper(r) {
				next(w, r)
				return
			}
			if r.Header.Get("Content-Encoding") == "gzip" {
				zr, e := gzip.NewReader(r.Body)
				if e != nil {
					panic(e)
				}
				defer zr.Close()
				r.Body = zr
			}

			if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				w.Header().Set("Content-Encoding", "gzip")
				gz := gzip.NewWriter(w)
				defer gz.Close()
				w = &gzipResponseWriter{ResponseWriter: w, Writer: gz}
			}
			next(w, r)
		}
	}, true
}

func gzipDefaultSkipper(r *http.Request) bool {
	ext := filepath.Ext(r.URL.Path)
	if ext == "" {
		return false
	}
	m := mime.TypeByExtension(ext)
	m = strings.Split(m, "; ")[0]

	if strings.HasPrefix(m, "text/") {
		return false
	}
	switch m {
	case "application/json":
		return false
	default:
		return true
	}
}
