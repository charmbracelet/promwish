// Package main provides an example of how to use the promwish package.
//
// You can test with:
//
//	go run main.go
//	ssh -o UserKnownHostsFile=/dev/null -p 2222 localhost
//	curl -s localhost:9222/metrics | grep wish_
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log/v2"
	"github.com/charmbracelet/promwish/v2"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/v2"
	bm "github.com/charmbracelet/wish/v2/bubbletea"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var keyTypeCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "wish_auth_by_type_total",
	Help: "Custom metric example",
}, []string{"type"})

func main() {
	s, err := wish.NewServer(
		wish.WithAddress("localhost:2223"),
		wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
		wish.WithMiddleware(
			bm.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				keyTypeCounter.WithLabelValues(s.PublicKey().Type()).Inc()
				pty, _, active := s.Pty()
				if !active {
					fmt.Println("no active terminal, skipping")
					return nil, nil
				}
				m := model{
					term: pty.Term,
					user: s.User(),
				}
				return m, []tea.ProgramOption{}
			}),
			promwish.Middleware("localhost:9223", "my-app"),
		),
	)
	if err != nil {
		log.Fatal("Fail to start SSH server", "error", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err = s.ListenAndServe(); err != nil {
			log.Fatal("Fail to start HTTP server", "error", err)
		}
	}()
	<-done
	if err := s.Close(); err != nil {
		log.Fatal("Fail to close SSH server", "error", err)
	}
}

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	user, term string
	quitting   bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.quitting {
		m.quitting = true
		return m, func() tea.Msg { return nil }
	}
	return m, tea.Quit
}

func (m model) View() string {
	return fmt.Sprintf("\n\nHello, %s. Your terminal is %s!\n\n\n", m.user, m.term)
}
