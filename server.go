package oauth2

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
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
	const redirectURI = "http://localhost:2345/oauth2/callback"
	state := mustState()
	scopes := []string{
		"email",
		"profile",
		"https://www.googleapis.com/auth/urlshortener",
	}
	u, err := s.createAuthorizationRequestURL(redirectURI, scopes, state)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("authorization request url = %v\n", u)

	cookie := &http.Cookie{
		Name:     "oauth2State",
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, u.String(), http.StatusFound)
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

func (s *server) createAuthorizationRequestURL(
	redirectURI string,
	scopes []string,
	state string,
) (*url.URL, error) {
	const endpoint = "https://accounts.google.com/o/oauth2/auth"
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", s.clientID)
	if redirectURI != "" {
		q.Set("redirect_uri", redirectURI)
	}
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}
	q.Set("state", state)
	q.Set("prompt", "consent")
	u.RawQuery = q.Encode()

	return u, nil
}

func mustState() string {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}
