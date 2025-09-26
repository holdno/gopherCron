package metrics

import (
	"fmt"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

var (
	GOPHERCRON_METRICS_NAMESPACE             = "gophercron"
	GOPHERCRON_METRICS_DEFAULT_PUSH_JOB_NAME = "gophercron_pusher"
)

func NewMetrics(serviceName, instance, pushGatewayEndpoint, pushGatewayJobName string) *Metrics {
	m := &Metrics{
		serviceName: serviceName,
		ns:          GOPHERCRON_METRICS_NAMESPACE,
		instance:    instance,
		counter:     make(map[string]*prometheus.CounterVec),
		gauge:       make(map[string]*prometheus.GaugeVec),
		histogram:   make(map[string]*prometheus.HistogramVec),
		registry:    prometheus.NewRegistry(),
	}

	m.GlobalMetrics = GlobalMetrics{
		PeriodTaskScheduleCounter: m.NewCounterVec("period_task_schedule", []string{"task"}),
		ElectionCounter:           m.NewCounterVec("election", []string{"action"}),
		InternalError:             m.NewCounterVec("internal_error", []string{"action"}),
	}

	if pushGatewayEndpoint != "" {
		if pushGatewayJobName == "" {
			pushGatewayJobName = GOPHERCRON_METRICS_DEFAULT_PUSH_JOB_NAME
		}
		m.pusher = push.New(pushGatewayEndpoint, pushGatewayJobName)
	}

	RegisterGoMetrics(m.registry)

	if m.pusher != nil {
		m.pusher.Collector(m.registry)
	}

	return m
}

type Metrics struct {
	serviceName string
	ns          string
	instance    string
	lock        sync.Mutex
	gauge       map[string]*prometheus.GaugeVec
	counter     map[string]*prometheus.CounterVec
	histogram   map[string]*prometheus.HistogramVec

	GlobalMetrics GlobalMetrics

	registry *prometheus.Registry
	pusher   *push.Pusher
}

type GlobalMetrics struct {
	PeriodTaskScheduleCounter *prometheus.CounterVec
	ElectionCounter           *prometheus.CounterVec
	InternalError             *prometheus.CounterVec
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

func (m *Metrics) CustomInc(name string, key, desc string) {
	m.lock.Lock()
	counter, exist := m.counter[name]
	if !exist {
		m.counter[name] = m.NewCounterVec(name, []string{"key", "desc", "instance"})
		counter = m.counter[name]
	}
	m.lock.Unlock()

	counter.With(prometheus.Labels{
		"desc":     desc,
		"key":      key,
		"instance": m.instance,
	}).Inc()
}

func (m *Metrics) NewGaugeFunc(name string, labels ...string) func(add float64, labels ...string) {
	m.lock.Lock()
	gauge, exist := m.gauge[name]
	if !exist {
		m.gauge[name] = m.NewGaugeVec(name, append([]string{"instance"}, labels...))
		gauge = m.gauge[name]
	} else {
		wlog.Error("duplic metric registered", zap.String("metric", name), zap.String("component", "metrics.gauge"))
	}
	m.lock.Unlock()

	labelsLen := len(labels)
	return func(add float64, values ...string) {
		if len(values) != labelsLen {
			wlog.Warn("invalid labels", zap.String("component", "metrics.counter"), zap.String("metric_name", name), zap.Error(fmt.Errorf("need %d, got %d", labelsLen, len(values))))
			return
		}
		pl := prometheus.Labels{
			"instance": m.instance,
		}
		for i, v := range values {
			pl[labels[i]] = v
		}

		gauge.With(pl).Add(add)
	}
}

func (m *Metrics) CustomIncFunc(name string, key, desc string) func() {
	m.lock.Lock()
	counter, exist := m.counter[name]
	if !exist {
		m.counter[name] = m.NewCounterVec(name, []string{"key", "desc", "instance"})
		counter = m.counter[name]
	}
	m.lock.Unlock()

	incer := counter.With(prometheus.Labels{
		"desc":     desc,
		"key":      key,
		"instance": m.instance,
	})
	return func() {
		incer.Inc()
	}
}

func (m *Metrics) NewCounter(name string, labels ...string) func(labels ...string) {
	m.lock.Lock()
	counter, exist := m.counter[name]
	if !exist {
		m.counter[name] = m.NewCounterVec(name, append([]string{"instance"}, labels...))
		counter = m.counter[name]
	} else {
		wlog.Error("duplic metric registered", zap.String("metric", name), zap.String("component", "metrics.counter"))
	}
	m.lock.Unlock()
	labelsLen := len(labels)
	return func(values ...string) {
		if len(values) != labelsLen {
			wlog.Warn("invalid labels", zap.String("component", "metrics.counter"), zap.String("metric_name", name), zap.Error(fmt.Errorf("need %d, got %d", labelsLen, len(values))))
			return
		}
		pl := prometheus.Labels{
			"instance": m.instance,
		}
		for i, v := range values {
			pl[labels[i]] = v
		}

		counter.With(pl).Inc()
	}
}

func (m *Metrics) NewHistogram(name string, labels ...string) func(labels ...string) *prometheus.Timer {
	m.lock.Lock()
	histogram, exist := m.histogram[name]
	if !exist {
		m.histogram[name] = m.NewHistogramVec(name, append([]string{"instance"}, labels...))
		histogram = m.histogram[name]
	} else {
		wlog.Error("duplic metric registered", zap.String("metric", name), zap.String("component", "metrics.histogram"))
	}
	m.lock.Unlock()
	labelsLen := len(labels)
	return func(values ...string) *prometheus.Timer {
		if len(values) != labelsLen {
			wlog.Warn("invalid labels", zap.String("component", "metrics.counter"), zap.String("metric_name", name), zap.Error(fmt.Errorf("need %d, got %d", labelsLen, len(values))))
			return nil
		}
		pl := prometheus.Labels{
			"instance": m.instance,
		}
		for i, v := range values {
			pl[labels[i]] = v
		}

		return prometheus.NewTimer(histogram.With(pl))
	}
}

func (m *Metrics) CustomHistogramSet(name string, keys ...string) *prometheus.Timer {
	m.lock.Lock()
	histogram, exist := m.histogram[name]
	if !exist {
		m.histogram[name] = m.NewHistogramVec(name, []string{"key", "instance"})
		histogram = m.histogram[name]
	}
	m.lock.Unlock()

	label := prometheus.Labels{
		"key":      "",
		"instance": m.instance,
	}
	if len(keys) > 0 {
		label["key"] = strings.Join(keys, "_")
	}
	return prometheus.NewTimer(histogram.With(label))
}
