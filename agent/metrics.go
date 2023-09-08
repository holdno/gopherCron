package agent

import (
	"github.com/holdno/gopherCron/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	provider    *metrics.Metrics
	jobs        *prometheus.GaugeVec
	systemError *prometheus.CounterVec
}

func NewMonitor(instance string) *Metrics {
	m := &Metrics{
		provider: metrics.NewMetrics("agent", instance),
	}

	m.jobs = m.provider.NewGaugeVec("job", nil)
	m.systemError = m.provider.NewCounterVec("system_error", []string{"reason"})
	return m
}

func (s *Metrics) SetJobCount(count int) {
	s.jobs.WithLabelValues().Set(float64(count))
}

func (s *Metrics) SystemErrInc(reason string) {
	s.systemError.WithLabelValues(reason).Inc()
}
