// Package promwish provides a simple middleware to expose some metrics to Prometheus.
package promwish

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DefaultCommandFn is a CommandFn that returns the first part of s.Command().
func DefaultCommandFn(s ssh.Session) string {
	if len(s.Command()) > 0 {
		return s.Command()[0]
	}
	return ""
}

// CommandFn is used to get the value of the `command` label in the Prometheus metrics.
type CommandFn func(s ssh.Session) string

// Middleware starts a HTTP server on the given address, serving the metrics
// from the default registerer to /metrics.
func Middleware(address, app string) wish.Middleware {
	return MiddlewareWithCommand(address, app, DefaultCommandFn)
}

// MiddlewareWithServer() starts a HTTP server on the given address, serving
// the metrics from the default registerer to /metrics.
func MiddlewareWithServer(server *Server, app string) wish.Middleware {
	return MiddlewareWithServerAndCommand(server, app, DefaultCommandFn)
}

// MiddlewareWithCommand() starts a HTTP server on the given address, serving
// the metrics from the default registerer to /metrics, using the given
// CommandFn to extract the `command` label value.
func MiddlewareWithCommand(address, app string, fn CommandFn) wish.Middleware {
	return MiddlewareWithServerAndCommand(NewServer(address), app, fn)
}

// MiddlewareWithServerAndCommand() starts a HTTP server on the given address,
// serving the metrics from the default registerer to /metrics, using the given
// CommandFn to extract the `command` label value.
func MiddlewareWithServerAndCommand(server *Server, app string, fn CommandFn) wish.Middleware {
	go func() {
		ListenServer(server)
	}()
	return MiddlewareRegistry(
		prometheus.DefaultRegisterer,
		prometheus.Labels{
			"app": app,
		},
		fn,
	)
}

// Middleware setup the metrics for the given registry without starting any HTTP servers.
// The caller is then responsible for serving the metrics.
func MiddlewareRegistry(registry prometheus.Registerer, constLabels prometheus.Labels, fn CommandFn) wish.Middleware {
	sessionsCreated := promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Name:        "wish_sessions_created_total",
		Help:        "The total number of sessions created",
		ConstLabels: constLabels,
	}, []string{"command"})

	sessionsFinished := promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Name:        "wish_sessions_finished_total",
		Help:        "The total number of sessions created",
		ConstLabels: constLabels,
	}, []string{"command"})

	sessionsDuration := promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Name:        "wish_sessions_duration_seconds",
		Help:        "The total sessions duration in seconds",
		ConstLabels: constLabels,
	}, []string{"command"})

	return func(sh ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			n := time.Now()
			sessionsCreated.WithLabelValues(fn(s)).Inc()
			defer func() {
				sessionsFinished.WithLabelValues(fn(s)).Inc()
				sessionsDuration.WithLabelValues(fn(s)).Add(time.Since(n).Seconds())
			}()
			sh(s)
		}
	}
}

// Server is a simple HTTP server that serves the metrics from the default registerer to /metrics.
type Server struct {
	srv *http.Server
}

// NewServer returns a new Server.
// The address should be in the form "host:port".
func NewServer(address string) *Server {
	srv := &http.Server{
		Addr:    address,
		Handler: promhttp.Handler(),
	}
	return &Server{srv: srv}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	log.Info("Starting metrics server", "address", "http://"+s.srv.Addr+"/metrics")
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not start metrics server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Info("Shutting down metrics server")
	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("could not shut down metrics server: %w", err)
	}
	return nil
}

// Listen starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
// It handles exit signals to gracefully shutdown the server.
func Listen(address string) {
	ListenServer(NewServer(address))
}

// ListenServer starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
// It handles exit signals to gracefully shutdown the server.
func ListenServer(srv *Server) {
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatal("Failed to start metrics server:", "error", err)
		}
	}()

	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() { cancel() }()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Failed to shutdown metrics server", "error", err)
	}
	log.Info("Shutdown metrics server")
}
