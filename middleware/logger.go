package middleware

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type (
	LoggerConfig struct {
		LogDir         string //default apilog
		LogBody        bool
		KeepDays       int                        // default 7
		BufSize        int                        // 1024
		SeperatePrefix []string                   // prefixes that are allowed to log into seperate folders, other requests would be in a common access.log file
		LogContextKeys []string                   // values from *http.Request.Context that need to be appended to the end of log messages, e.g. userId or machineId
		LogHeaderKeys  []string                   // values from *http.Request.Header that need to be appended to the end of log messages, e.g. userId or machineId
		Skipper        func(r *http.Request) bool // log skipper function, return true if you want to skip.
		MaxLogBodyLen  int                        // ignore request/response body when its length exceed this limit. Default 10K
	}
	logMessage struct {
		path    string
		content []byte
	}
	logResponseWriter struct {
		http.ResponseWriter
		Buffer     *bytes.Buffer
		statuscode int
	}
)

const rotateLayout = "2006_01_02"

func Logger(cfg LoggerConfig) (func(http.HandlerFunc) http.HandlerFunc, bool) {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
	fo, e := os.OpenFile("log.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if e != nil {
		panic(e)
	}
	log.SetOutput(fo)

	//default config
	if cfg.LogDir == "" {
		cfg.LogDir = "apilog"
	}
	if cfg.KeepDays <= 0 {
		cfg.KeepDays = 7
	}
	if cfg.BufSize <= 0 {
		cfg.BufSize = 1024
	}
	if cfg.MaxLogBodyLen <= 0 {
		cfg.MaxLogBodyLen = 10 << 10
	}

	day := time.Now().Format(rotateLayout)
	loggerChan := make(chan logMessage, cfg.BufSize)
	loggerChanClosed := false
	go func() {
		defer func() {
			loggerChanClosed = true
			log.Println("err: ", cfg.LogDir, " closed expectly")
		}()
		defer close(loggerChan)
		foMap := make(map[string]*os.File)
		for msg := range loggerChan {
			dst := filepath.Join(cfg.LogDir, "/access.log")
			for _, p := range cfg.SeperatePrefix {
				if strings.HasPrefix(msg.path, p) {
					dst = filepath.Join(cfg.LogDir, msg.path, "access.log")
					break
				}
			}
		FLAG_OPENFILE:
			fo, ok := foMap[msg.path]
			if !ok {
				e := os.MkdirAll(filepath.Dir(dst), 0755)
				if e != nil {
					log.Println(e)
					return
				}

				fo, e = os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if e != nil {
					log.Println(e)
					return
				}
				foMap[msg.path] = fo
			}

			dayNow := time.Now().Format(rotateLayout)
			if dayNow != day {
				// rotate
				fo.Close()
				e := compressLogFile(dst+"-"+day+".gz", dst)
				if e != nil {
					log.Println(e)
					return
				}
				day = dayNow
				delete(foMap, msg.path)
				// delete outdated logs
				deleteOutdatedLogs(cfg.LogDir, cfg.KeepDays)
				goto FLAG_OPENFILE
			}
			_, e := fo.Write(msg.content)
			if e != nil {
				log.Println(e)
				return
			}
		}
	}()
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// skip
			if loggerChanClosed || cfg.Skipper != nil && cfg.Skipper(r) {
				next(w, r)
				return
			}

			s := &logResponseWriter{ResponseWriter: w}
			var rbody []byte
			if cfg.LogBody {
				if !loggerRequestBodySkipper(r) {
					rbody, _ = io.ReadAll(r.Body)
					r.Body = io.NopCloser(bytes.NewReader(rbody))
				}

				if !loggerResponseBodySkipper(r) {
					s.Buffer = new(bytes.Buffer)
				}
			}
			next(s, r)

			content := new(bytes.Buffer)
			content.WriteString(time.Now().Format(time.RFC3339))
			content.WriteRune('\t')
			content.WriteString(strconv.Itoa(s.Status()))
			content.WriteRune('\t')
			content.WriteString(r.Method)
			content.WriteRune('\t')
			content.WriteString(r.RequestURI)

			for _, key := range cfg.LogContextKeys {
				v := contextString(r, key)
				if v != "" {
					content.WriteRune('\t')
					content.WriteString(key + ":")
					content.WriteString(v)
				}
			}
			for _, key := range cfg.LogHeaderKeys {
				v := r.Header.Get(key)
				if v != "" {
					content.WriteRune('\t')
					content.WriteString(key + ":")
					content.WriteString(v)
				}
			}

			content.WriteRune('\t')
			content.WriteString(r.UserAgent())

			content.WriteRune('\n')

			if cfg.LogBody {
				if len(rbody) < cfg.MaxLogBodyLen {
					content.WriteString("r: ")
					content.Write(rbody)
					content.WriteRune('\n')
				}
				if s.Buffer != nil && s.Buffer.Len() < cfg.MaxLogBodyLen {
					content.WriteString("w: ")
					content.Write(s.Buffer.Bytes())
					content.WriteRune('\n')
				}
			}

			loggerChan <- logMessage{path: r.URL.Path[1:], content: content.Bytes()}
		}
	}, false
}

func loggerResponseBodySkipper(r *http.Request) bool {
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

func loggerRequestBodySkipper(r *http.Request) bool {
	m := r.Header.Get("Content-Type")
	if m == "" {
		return false
	}
	m = strings.Split(m, "; ")[0]

	if strings.HasPrefix(m, "text/") {
		return false
	}
	switch m {
	case "application/json", "application/x-www-form-urlencoded":
		return false
	default:
		return true
	}
}

func contextString(r *http.Request, key string) string {
	v := r.Context().Value(key)
	if v == nil {
		return ""
	}
	if val, ok := v.(string); ok {
		return val
	}
	return fmt.Sprint(v)
}
func deleteOutdatedLogs(dst string, keepDays int) {
	list, e := os.ReadDir(filepath.Dir(dst))
	if e != nil {
		log.Println(e)
		return
	}
	for _, f := range list {
		if f.IsDir() {
			continue
		}
		if strings.HasSuffix(f.Name(), ".gz") {
			t, e := getDateOfFilename(f.Name())
			if e != nil {
				log.Println(e)
				continue
			}
			if time.Now().Sub(t) > time.Duration(keepDays)*24*time.Hour {
				e := os.Remove(filepath.Join(filepath.Dir(dst), f.Name()))
				if e != nil {
					log.Println(e)
					continue
				}
			}
		}
	}
}
func getDateOfFilename(filename string) (time.Time, error) {
	filename = strings.TrimSuffix(filename, ".gz")
	ss := strings.Split(filename, ".log-")
	if len(ss) != 2 {
		return time.Time{}, errors.New("invalid filename")
	}
	return time.ParseInLocation(rotateLayout, ss[1], time.Local)
}
func compressLogFile(dst, src string) error {
	e := os.MkdirAll(filepath.Dir(dst), 0755)
	if e != nil {
		log.Println(e)
		return e
	}

	fo, e := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if e != nil {
		log.Println(e)
		return e
	}
	defer fo.Close()
	zw := gzip.NewWriter(fo)
	defer zw.Close()
	fi, e := os.Open(src)
	if e != nil {
		log.Println(e)
		return e
	}
	defer fi.Close()
	_, e = io.Copy(zw, fi)
	if e != nil {
		log.Println(e)
		return e
	}

	e = os.Remove(src)
	if e != nil {
		log.Println(e)
		return e
	}

	return nil
}

func (w *logResponseWriter) Write(b []byte) (int, error) {
	if w.Buffer != nil {
		_, e := w.Buffer.Write(b)
		if e != nil {
			log.Println(e)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *logResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.statuscode = code
}

func (w *logResponseWriter) Status() int {
	if w.statuscode <= 0 {
		return 200
	}
	return w.statuscode
}
