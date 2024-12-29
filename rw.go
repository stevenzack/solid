package solid

import (
	"encoding/json"
	"io"
	"net/http"
)

func ReadJson[T any](w http.ResponseWriter, r *http.Request) T {
	var v T
	defer r.Body.Close()
	b, e := io.ReadAll(r.Body)
	if e != nil {
		panic(e)
	}
	e = json.Unmarshal(b, &v)
	if e != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		panic(struct{}{})
	}
	return v
}
func WriteJson(w http.ResponseWriter, r *http.Request, data any) {
	w.Header().Set("Content-Type", "application/json")
	b, e := json.Marshal(data)
	if e != nil {
		panic(e)
	}
	_, e = w.Write(b)
	if e != nil {
		panic(e)
	}
}

func Error(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}
