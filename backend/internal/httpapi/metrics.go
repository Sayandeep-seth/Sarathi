package httpapi

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registry = prometheus.NewRegistry()

	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "sarathi_http_requests_total", Help: "HTTP requests by route pattern and status.",
	}, []string{"route", "status"})

	// WSConnections is set by the realtime hub owner.
	WSConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sarathi_ws_connections", Help: "WebSocket clients connected to this replica.",
	})
)

func init() {
	registry.MustRegister(httpRequests, WSConnections,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

// observe counts a finished request. Route patterns (not raw paths) keep label
// cardinality bounded and keep identifiers out of metrics.
func observe(pattern string, w http.ResponseWriter) {
	status := http.StatusOK
	if s, ok := w.(interface{ Status() int }); ok {
		status = s.Status()
	}
	httpRequests.WithLabelValues(pattern, strconv.Itoa(status)).Inc()
}

func metricsHandler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
