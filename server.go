package oauth2

import (
	"fmt"
	"html/template"
	"net/http"
)

type server struct{}

func NewServer() *server {
	return &server{}
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
		// TODO: internalServerError
		panic(err)
		//internalServerError(w, errors.NewInternalError(
		//	errors.WithError(err),
		//	errors.WithMessage("Failed to template.Execute()"),
		//), 0)
		return
	}
}

func (s *server) static(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
}

func (s *server) authorize(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Printf("%v\n", r.URL.Path[1:])
	fmt.Fprint(w, "authorize")
}

func (s *server) parseHTMLTemplates(files ...string) *template.Template {
	f := []string{
		"template/_base.html",
	}
	f = append(f, files...)
	return template.Must(template.ParseFiles(f...))
}
