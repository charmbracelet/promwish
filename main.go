// Package promwish provides a simple middleware to expose some metrics to Prometheus.
package promwish

import (
	"net/http"

	"github.com/charmbracelet/wish"
	"github.com/gliderlabs/ssh"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Middleware starts a HTTP server on the given address, serving the metrics from the default registerer to /metrics.
func Middleware(address string) wish.Middleware {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(address, nil) // TODO: should probably better handle server shutdown
	return MiddlewareRegistry(prometheus.DefaultRegisterer)
}

// Middleware setup the metrics for the given registry without starting any HTTP servers.
// The caller is then responsible for serving the metrics.
func MiddlewareRegistry(registry prometheus.Registerer) wish.Middleware {
	sessionsCreated := promauto.With(registry).NewCounter(prometheus.CounterOpts{
		Name: "wish_sessions_created_total",
		Help: "The total number of sessions created",
	})

	sessionsFinished := promauto.With(registry).NewCounter(prometheus.CounterOpts{
		Name: "wish_sessions_finished_total",
		Help: "The total number of sessions created",
	})

	return func(sh ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			sessionsCreated.Inc()
			defer sessionsFinished.Inc()
			sh(s)
		}
	}
}
