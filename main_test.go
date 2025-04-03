package promwish_test

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/promwish/v2"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/v2/testsession"
)

func TestMiddleware(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	if err := listener.Close(); err != nil {
		t.Error(err)
	}

	srv := &ssh.Server{
		Handler: promwish.Middleware(listener.Addr().String(), "test")(func(s ssh.Session) {
			_, _ = s.Write([]byte("test"))
			time.Sleep(500 * time.Millisecond)
		}),
	}

	if err := testsession.New(t, srv, nil).Run(""); err != nil {
		t.Error(err)
	}

	if err := testsession.New(t, srv, nil).Run("my-cmd foo bar args"); err != nil {
		t.Error(err)
	}

	resp, err := http.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Error(err)
	}
	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	for _, m := range []string{
		`wish_sessions_created_total{app="test",command=""} 1`,
		`wish_sessions_created_total{app="test",command="my-cmd"} 1`,
		`wish_sessions_finished_total{app="test",command=""} 1`,
		`wish_sessions_finished_total{app="test",command="my-cmd"} 1`,
		`wish_sessions_duration_seconds{app="test",command=""} 0.5`,
		`wish_sessions_duration_seconds{app="test",command="my-cmd"} 0.5`,
	} {
		if !strings.Contains(string(bts), m) {
			t.Errorf("expected to find %q, got %s", m, string(bts))
		}
	}
}
