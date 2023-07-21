package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
)

type MockServiceHandler struct {
}

func (dh *MockServiceHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	_ = req.Context()
	body := req.Body
	defer body.Close()
	rw.WriteHeader(http.StatusAccepted)
	rw.Header().Set("Content-Type", "application/json")
	var b bytes.Buffer
	io.Copy(&b, body)
	log.Print(b.String())
	rw.Write(b.Bytes())
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/mock", &MockServiceHandler{})
	return mux
}

func main() {
	fmt.Println("mock-application")
	srv := &http.Server{
		Addr:    ":8081",
		Handler: NewHandler(),
	}

	log.Println("Starting mock application ...")
	log.Fatal(srv.ListenAndServe())
}
