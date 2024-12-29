package solid

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
)

type serveMux struct {
	middlewares []Middleware
	OnRecover   RecoverHandler
}
type (
	Middleware     func(next http.HandlerFunc) http.HandlerFunc
	RecoverHandler func(w http.ResponseWriter, r *http.Request, value any)
)

func New() *serveMux {
	return &serveMux{
		OnRecover: func(w http.ResponseWriter, r *http.Request, v any) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			if _, ok := v.(struct{}); ok {
				return
			}
			stack := debug.Stack()
			s := strings.Split(string(stack), "\n")
			line := s[0]
			const lineIndex = 10
			if len(s) > lineIndex {
				line = strings.TrimSpace(s[lineIndex])
			}
			log.Println(line, v)
		},
	}
}
func (sm *serveMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fn := sm.serveWithRecover
	for i := len(sm.middlewares) - 1; i >= 0; i-- {
		fn = sm.middlewares[i](fn)
	}
	fn(w, r)
}
func (sm *serveMux) serveWithRecover(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if v := recover(); v != nil {
			if sm.OnRecover != nil {
				sm.OnRecover(w, r, v)
			}
		}
	}()
	http.DefaultServeMux.ServeHTTP(w, r)
}
func (sm *serveMux) Use(middleware Middleware, hightPriority bool) {
	if hightPriority {
		sm.middlewares = append([]Middleware{middleware}, sm.middlewares...)
		return
	}
	sm.middlewares = append(sm.middlewares, middleware)
}
