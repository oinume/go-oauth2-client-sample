package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	oauth2 "github.com/oinume/go-oauth2-client-sample"
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "2345"
	}
	server := oauth2.NewServer(
		strings.TrimSpace(os.Getenv("CLIENT_ID")),
		strings.TrimSpace(os.Getenv("CLIENT_SECRET")),
	)
	fmt.Printf("Listening on %v\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%v", port), server.NewMux())
}
