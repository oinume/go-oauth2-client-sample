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
	authorizationEndpoint = "https://accounts.google.com/o/oauth2/auth"
	tokenEndpoint         = "https://accounts.google.com/o/oauth2/token"
	stateCookieName       = "oauthState"
	redirectURI           = "http://localhost:2345/oauth2/callback"
)

var (
	scopes = []string{
		"email",
		"https://www.googleapis.com/auth/gmail.readonly",
	}
)

type server struct {
	clientID     string
	clientSecret string
}

type tokenEntity struct {
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

func (s *server) authorize(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	u, err := s.createAuthorizationRequestURL(redirectURI, scopes, state)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	log.Printf("authorization request url = %v\n", u)

	// Set state to cookie
	cookie := &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)

	// Send authorization request by redirection
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *server) callback(w http.ResponseWriter, r *http.Request) {
	log.Printf("callback: state=%v, code=%v", r.FormValue("state"), r.FormValue("code"))

	if e := r.FormValue("error"); e != "" {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("error returned in authorization: %v", e))
		return
	}
	if err := validateState(r); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	code := r.FormValue("code")
	if code == "" {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("code is required"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	token, err := s.exchange(ctx, code)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "accessToken = %v", token.AccessToken)
	// save token to database or do something
}

func (s *server) createAuthorizationRequestURL(
	redirectURI string,
	scopes []string,
	state string,
) (*url.URL, error) {
	u, err := url.Parse(authorizationEndpoint)
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

func generateState() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate state: %v", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func validateState(r *http.Request) error {
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

func (s *server) exchange(ctx context.Context, code string) (*tokenEntity, error) {
	v := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret}, // need this? there is no spec on OAuth2.0
	}
	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Client authentication: https://tools.ietf.org/html/rfc6749#section-4.1.3
	req.SetBasicAuth(url.QueryEscape(s.clientID), url.QueryEscape(s.clientSecret))

	// Send token request
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
		log.Printf("token request failed: statusCode=%v, body=%v\n", code, string(body))
		return nil, fmt.Errorf("oauth2: token request failed: statusCode=%v", code)
	}

	// Create tokenEntity from response
	var token *tokenEntity
	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("oauth2: failed to parse Content-Type header: %v", err)
	}
	switch contentType {
	// TODO: need to support this mime type?
	case "application/x-www-form-urlencoded", "text/plain":
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		token = &tokenEntity{
			AccessToken:  vals.Get("access_token"),
			TokenType:    vals.Get("token_type"),
			RefreshToken: vals.Get("refresh_token"),
		}
		e := vals.Get("expires_in")
		expires, _ := strconv.Atoi(e)
		if expires != 0 {
			token.expiry = time.Now().Add(time.Duration(expires) * time.Second)
		}
	case "application/json":
		token = &tokenEntity{}
		if err = json.Unmarshal(body, token); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("oauth2: invalid Content-Type in response: %v", contentType)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("oauth2: server response missing access_token")
	}

	return token, nil
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

func (s *server) writeError(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	fmt.Fprint(w, err.Error())
}

func (s *server) parseHTMLTemplates(files ...string) *template.Template {
	f := []string{
		"template/_base.html",
	}
	f = append(f, files...)
	return template.Must(template.ParseFiles(f...))
}
