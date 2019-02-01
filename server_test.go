package oauth2

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_server_authorize(t *testing.T) {
	s := NewServer("a", "b")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/oauth2/authorize", nil)
	s.authorize(rr, req)
	if want, got := http.StatusFound, rr.Result().StatusCode; want != got {
		t.Errorf("response status doesn't match: want=%v, got=%v", want, got)
	}
}
