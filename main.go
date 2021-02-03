package main

import (
	"flag"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Llongfile)

	r := mux.NewRouter()
	r.HandleFunc("/app", appHandler)

	log.Fatal(http.ListenAndServe(":40000", r))
}
