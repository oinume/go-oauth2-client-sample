package oauth2

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
)

type server struct {
	clientID     string
	clientSecret string
}

func NewServer(clientID, clientSecret string) *server {
	return &server{
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

func (s *server) NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/authorize", s.authorize)
	mux.HandleFunc("/static/", s.static)
	mux.HandleFunc("/", s.index)
	return mux
}

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	t := s.parseHTMLTemplates("template/index.html")
	if err := t.Execute(w, nil); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
}

func (s *server) static(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
}

func (s *server) authorize(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	state := mustState()
	//fmt.Printf("%v\n", r.URL.Path[1:])
	fmt.Printf("state = %v\n", state)
	fmt.Fprint(w, "authorize\n")
	fmt.Fprint(w, state)
}

func (s *server) parseHTMLTemplates(files ...string) *template.Template {
	f := []string{
		"template/_base.html",
	}
	f = append(f, files...)
	return template.Must(template.ParseFiles(f...))
}

func (s *server) writeError(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	fmt.Fprint(w, err.Error())
}

func mustState() string {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}
