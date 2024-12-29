package main

import (
	"net/http"

	"gitlab.com/stevenzack/solid"
	"gitlab.com/stevenzack/solid/middleware"
	"gorm.io/gorm"
)
var dbc *gorm.DB
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
