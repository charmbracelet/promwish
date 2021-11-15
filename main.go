package wishprometheus

import (
	"net/http"

	"github.com/charmbracelet/wish"
	"github.com/gliderlabs/ssh"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Middleware(address string) wish.Middleware {
	sessionsCreated := promauto.NewCounter(prometheus.CounterOpts{
		Name: "wish_sessions_created_total",
		Help: "The total number of sessions created",
	})

	sessionsFinished := promauto.NewCounter(prometheus.CounterOpts{
		Name: "wish_sessions_finished_total",
		Help: "The total number of sessions created",
	})

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(address, nil)
	return func(sh ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			sessionsCreated.Inc()
			defer sessionsFinished.Inc()
			sh(s)
		}
	}
}
