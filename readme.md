# solid http server

```go
package main

import (
	"net/http"

	"gitlab.com/stevenzack/solid"
	"gitlab.com/stevenzack/solid/middleware"
)

func main() {
	s := solid.New()
	s.Use(middleware.GZip())
	s.Use(middleware.CORS())

	http.HandleFunc("POST /tokens", login)
	http.ListenAndServe(":8080", s)
}
func login(w http.ResponseWriter, r *http.Request) {
	type Request struct {
		Account  string
		Password string
	}
	req := solid.ReadJson[Request](w, r)

	if req.Account != "foo" {
		solid.Error(w, http.StatusNotFound)
		return
	}
	if req.Password != "bar" {
		solid.Error(w, http.StatusUnauthorized)
		return
	}

	solid.WriteJson(w, r, []string{
		"Asd",
		req.Account,
		req.Password,
	})
}
```

# Error handling

500 internal server error via panic(error)
```go
var user User
e:=dbConn.First(&user,12).Error
if e !=nil {
	panic(e)
}
```

Custom response, this will return 400-invalid parameter: name
```go
http.Error(w, "invalid parameter: name",http.StatusBadRequest)
panic(struct{}{})
```

Here is our embeded default recover function
```go
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
}
```
You can also write your own recover function
```go
s := solid.New()
s.OnRecover = func(w http.ResponseWriter, r *http.Request, v any) {
	if i, ok := v.(int); ok {
		w.WriteHeader(400)
		fmt.Fprintf(w, "{\"code\":%d,\"msg\":\"%s\"}", i, ec.Text(i, lang.IsChinese(r)))
		return
	}
	if _, ok := v.(struct{}); ok {
		return
	}

	stack := debug.Stack()
	s := strings.Split(string(stack), "\n")
	log.Println(strings.TrimSpace(s[8]), v)
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "{\"code\":%d,\"msg\":\"%s\"}", 0, ec.Text(0, lang.IsChinese(r)))
}
```