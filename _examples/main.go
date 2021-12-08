package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/promwish"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/gliderlabs/ssh"
)

func main() {
	s, err := wish.NewServer(
		wish.WithAddress("localhost:2222"),
		wish.WithMiddleware(
			bm.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
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
			promwish.Middleware("localhost:9222", "my-app"),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err = s.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()
	<-done
	if err := s.Close(); err != nil {
		log.Fatalln(err)
	}
}

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	user, term string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case quitMsg:
		return m, tea.Quit
	}
	return m, tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
		return quitMsg("quit")
	})
}

func (m model) View() string {
	return fmt.Sprintf("\n\nHello, %s. Your terminal is %s!\n\n\n", m.user, m.term)
}

type quitMsg string
