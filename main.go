// Package promwish provides a simple middleware to expose some metrics to Prometheus.
package promwish

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/wish"
	"github.com/gliderlabs/ssh"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Middleware starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
func Middleware(address, app string) wish.Middleware {
	go func() {
		Listen(address)
	}()
	return MiddlewareRegistry(
		prometheus.DefaultRegisterer,
		prometheus.Labels{
			"app": app,
		},
	)
}

// Middleware setup the metrics for the given registry without starting any HTTP servers.
// The caller is then responsible for serving the metrics.
func MiddlewareRegistry(registry prometheus.Registerer, constLabels prometheus.Labels) wish.Middleware {
	sessionsCreated := promauto.With(registry).NewCounter(prometheus.CounterOpts{
		Name:        "wish_sessions_created_total",
		Help:        "The total number of sessions created",
		ConstLabels: constLabels,
	})

	sessionsFinished := promauto.With(registry).NewCounter(prometheus.CounterOpts{
		Name:        "wish_sessions_finished_total",
		Help:        "The total number of sessions created",
		ConstLabels: constLabels,
	})

	sessionsDuration := promauto.With(registry).NewCounter(prometheus.CounterOpts{
		Name:        "wish_sessions_duration_seconds",
		Help:        "The total sessions duration in seconds",
		ConstLabels: constLabels,
	})

	commandsRan := promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Name:        "wish_sessions_command_runs_total",
		Help:        "Total number of times a given SSH command was executed",
		ConstLabels: constLabels,
	}, []string{"command"})

	return func(sh ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			n := time.Now()
			if len(s.Command()) > 0 {
				commandsRan.WithLabelValues(s.Command()[0]).Inc()
			} else {
				commandsRan.WithLabelValues("").Inc()
			}
			sessionsCreated.Inc()
			defer func() {
				sessionsFinished.Inc()
				sessionsDuration.Add(time.Since(n).Seconds())
			}()
			sh(s)
		}
	}
}

// Listen starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
// It handles exit signals to gracefully shutdown the server.
func Listen(address string) {
	srv := &http.Server{
		Addr:    address,
		Handler: promhttp.Handler(),
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start metrics server: %s\n", err)
		}
	}()
	log.Printf("Serving metrics at http://%s/metrics", address)

	<-done
	log.Print("Stopping metrics server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() { cancel() }()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Failed to shutdown metrics server: %+v", err)
	}
	log.Print("Shutdown metrics server")
}
