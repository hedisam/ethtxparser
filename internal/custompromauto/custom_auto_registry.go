package custompromauto

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var registry *prometheus.Registry
var auto promauto.Factory

func init() {
	registry = prometheus.NewRegistry()
	auto = promauto.With(registry)
}

func Auto() promauto.Factory {
	return auto
}

func Registry() *prometheus.Registry {
	return registry
}
