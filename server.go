package oauth2

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	stateCookieName = "oauthState"
	redirectURI     = "http://localhost:2345/oauth2/callback"
)

var (
	errAccessDenied = fmt.Errorf("oauth2: access_denied")
	errUnknown      = fmt.Errorf("oauth2: unknown error")
)

type server struct {
	clientID     string
	clientSecret string
}

type token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	expiry       time.Time
}

func NewServer(clientID, clientSecret string) *server {
	return &server{
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

func (s *server) NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/callback", s.callback)
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
	state := mustState()
	scopes := []string{
		"email",
		"profile",
		"https://www.googleapis.com/auth/gmail.readonly",
	}
	u, err := s.createAuthorizationRequestURL(redirectURI, scopes, state)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("authorization request url = %v\n", u)

	cookie := &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *server) callback(w http.ResponseWriter, r *http.Request) {
	log.Printf("callback: state=%v, code=%v", r.FormValue("state"), r.FormValue("code"))

	if err := checkState(r); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tk, err := s.exchange(ctx, r)
	if err != nil {
		if err == errAccessDenied {
			// TODO: show error?
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "accessToken = %v", tk.AccessToken)
	// save tk to database or do something
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

func checkState(r *http.Request) error {
	state := r.FormValue("state")
	oauthState, err := r.Cookie(stateCookieName)
	if err != nil {
		return fmt.Errorf("failed to get cookie %v", stateCookieName)
	}
	if state != oauthState.Value {
		return fmt.Errorf("state doesn't match")
	}
	return nil
}

func (s *server) exchange(ctx context.Context, r *http.Request) (*token, error) {
	if e := r.FormValue("error"); e != "" {
		switch e {
		case "access_denied":
			return nil, errAccessDenied
		default:
			return nil, errUnknown
		}
	}

	code := r.FormValue("code")
	// TODO: check code exists
	tk, err := s.retrieveToken(ctx, code, redirectURI)
	if err != nil {
		return nil, err
	}

	return tk, nil
}

func (s *server) retrieveToken(ctx context.Context, code, redirectURI string) (*token, error) {
	v := url.Values{
		"grant_type": {"authorization_code"},
		"code":       {code},
	}
	if redirectURI != "" {
		v.Set("redirect_uri", redirectURI)
	}

	const tokenEndpoint = "https://accounts.google.com/o/oauth2/token"
	v.Set("client_id", s.clientID)
	v.Set("client_secret", s.clientSecret) // need this? there is no spec on OAuth2.0
	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// https://tools.ietf.org/html/rfc6749#section-4.1.3
	// Client authentication
	req.SetBasicAuth(url.QueryEscape(s.clientID), url.QueryEscape(s.clientSecret))
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("oauth2: cannot fetch token: %v", err)
	}
	if code := resp.StatusCode; code < 200 || code > 299 {
		return nil, fmt.Errorf("token request failed: statusCode=%v", code)
	}

	var tk *token
	content, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Content-Type header: %v", err)
	}
	switch content {
	// TODO: need to support this mime type?
	case "application/x-www-form-urlencoded", "text/plain":
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		tk = &token{
			AccessToken:  vals.Get("access_token"),
			TokenType:    vals.Get("token_type"),
			RefreshToken: vals.Get("refresh_token"),
			//Raw:          vals,
		}
		e := vals.Get("expires_in")
		expires, _ := strconv.Atoi(e)
		if expires != 0 {
			tk.expiry = time.Now().Add(time.Duration(expires) * time.Second)
		}
	default:
		tk = &token{}
		if err = json.Unmarshal(body, tk); err != nil {
			return nil, err
		}
	}
	if tk.AccessToken == "" {
		return nil, fmt.Errorf("oauth2: server response missing access_token")
	}
	return tk, nil
}
