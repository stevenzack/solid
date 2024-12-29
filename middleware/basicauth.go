package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func BasicAuth(username, password string, skippers ...func(r *http.Request) bool) (func(http.HandlerFunc) http.HandlerFunc, bool) {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			for _, skipper := range skippers {
				if skipper(r) {
					next(w, r)
					return
				}
			}
			s := r.Header.Get("Authorization")
			s = strings.TrimPrefix(s, "Basic ")
			b, e := base64.StdEncoding.DecodeString(s)
			if e != nil {
				panic(e)
			}
			ss := strings.Split(string(b), ":")
			u := ss[0]
			p := ""
			if len(ss) > 1 {
				p = ss[1]
			}
			if u != username || p != password {
				w.Header().Set("WWW-Authenticate", "Basic realm=\"staging server\"")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}, false
}
