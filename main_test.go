package promwish_test

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/promwish"
	"github.com/charmbracelet/wish/testsession"
	"github.com/gliderlabs/ssh"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	gossh "golang.org/x/crypto/ssh"
)

func TestMiddleware(t *testing.T) {
	var srv = httptest.NewServer(promhttp.Handler())
	t.Cleanup(srv.Close)

	if err := setup(t).Run(""); err != nil {
		t.Error(err)
	}
	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Error(err)
	}
	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	for _, m := range []string{
		"wish_sessions_created_total 1",
		"wish_sessions_finished_total 1",
	} {
		if !strings.Contains(string(bts), m) {
			t.Errorf("expected to find %q, got %s", m, string(bts))
		}
	}
}

func setup(t *testing.T) *gossh.Session {
	session, _, cleanup := testsession.New(t, &ssh.Server{
		Handler: promwish.Middleware("")(func(s ssh.Session) {
			s.Write([]byte("test"))
		}),
	}, nil)
	t.Cleanup(cleanup)
	return session
}
