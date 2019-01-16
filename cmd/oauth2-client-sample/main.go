package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	oauth2 "github.com/oinume/go-oauth2-client-sample"
)

var (
	port = flag.Int("port", 2345, "Listen port. default is 2345")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	server := oauth2.NewServer()
	fmt.Printf("Listening on %v\n", *port)
	return http.ListenAndServe(fmt.Sprintf(":%v", *port), server.NewMux())
}
