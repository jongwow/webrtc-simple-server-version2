package main

import (
	"flag"
	"github.com/gorilla/mux"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Llongfile)

	r := mux.NewRouter()
	r.HandleFunc("/app", appHandler)

	log.Fatal(http.ListenAndServe(":40000", r))
}
