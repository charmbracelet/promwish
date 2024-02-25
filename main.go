// Package promwish provides a simple middleware to expose some metrics to Prometheus.
package promwish

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Option func(*Options)

type Options struct {
	SkipDefaultDoneSignals bool
	WaitGroup              *sync.WaitGroup
}

// SkipDefaultDoneSignals skips the default done signals (os.Interrupt, syscall.SIGINT, syscall.SIGTERM) to shutdown the HTTP server.
func SkipDefaultDoneSignals() Option {
	return func(o *Options) {
		o.SkipDefaultDoneSignals = true
	}
}

// WaitGroup sets the wait group to use for the HTTP server.
func WithWaitGroup(wg *sync.WaitGroup) Option {
	return func(o *Options) {
		o.WaitGroup = wg
	}
}

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

// MiddlewareWithCommand() starts a HTTP server on the given address, serving
// the metrics from the default registerer to /metrics, using the given
// CommandFn to extract the `command` label value.
func MiddlewareWithCommand(address, app string, fn CommandFn, opts ...Option) wish.Middleware {
	return MiddlewareWithContextSignalCommand(context.Background(), nil, address, app, fn, opts...)
}

// MiddlewareWithContext starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
// It handles provided context to gracefully shutdown the server.
func MiddlewareWithContext(context context.Context, address, app string, opts ...Option) wish.Middleware {
	return MiddlewareWithContextSignal(context, nil, address, app, opts...)
}

// MiddlewareWithContextSignal starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
// It handles provided context and exit signals to gracefully shutdown the server.
func MiddlewareWithContextSignal(context context.Context, done chan os.Signal, address, app string, opts ...Option) wish.Middleware {
	return MiddlewareWithContextSignalCommand(context, done, address, app, DefaultCommandFn, opts...)
}

// MiddlewareWithContextSignalCommand starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
// It handles provided context and exit signals to gracefully shutdown the server.
// It uses the given CommandFn to extract the `command` label value.
func MiddlewareWithContextSignalCommand(context context.Context, done chan os.Signal, address, app string, fn CommandFn, opts ...Option) wish.Middleware {
	var o Options
	for _, opt := range opts {
		opt(&o)
	}
	go func() {
		ListenWithContextSignal(context, done, address, opts...)
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

// Listen starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
// It handles exit signals to gracefully shutdown the server.
func Listen(address string, opts ...Option) {
	ListenWithContextSignal(context.Background(), nil, address, opts...)
}

// DefaultDoneSignals are the signals that will trigger the shutdown of the HTTP server.
var DefaultDoneSignals = []os.Signal{os.Interrupt, syscall.SIGINT, syscall.SIGTERM}

// ListenWithContextSignal starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
// It handles provided context and exit signals to gracefully shutdown the server.
func ListenWithContextSignal(ctx context.Context, done chan os.Signal, address string, opts ...Option) {
	o := Options{}
	for _, opt := range opts {
		opt(&o)
	}

	srv := &http.Server{
		Addr:    address,
		Handler: promhttp.Handler(),
	}

	if o.WaitGroup != nil {
		o.WaitGroup.Add(1)
		log.Info("Incrementing wait group for metrics server", "address", address)
		defer func() {
			log.Info("Decrementing wait group for metrics server", "address", address)
			o.WaitGroup.Done()
		}()
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start metrics server:", "error", err)
		}
	}()
	log.Info("Starting metrics server", "address", "http://"+address+"/metrics")

	if done == nil {
		done = make(chan os.Signal, 1)
	}
	if !o.SkipDefaultDoneSignals {
		signal.Notify(done, DefaultDoneSignals...)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		log.Info("Context done, stopping metrics server")
	case <-done:
		log.Info("Received signal, stopping metrics server")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Failed to shutdown metrics server", "error", err)
	}
	log.Info("Shutdown metrics server")
}
