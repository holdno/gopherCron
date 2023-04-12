package metrics

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

func NewMetrics(serviceName, instance string) *Metrics {
	return &Metrics{
		serviceName: serviceName,
		instance:    instance,
		counter:     make(map[string]*prometheus.CounterVec),
		histogram:   make(map[string]*prometheus.HistogramVec),
	}
}

type Metrics struct {
	serviceName string
	instance    string
	lock        sync.Mutex
	counter     map[string]*prometheus.CounterVec
	histogram   map[string]*prometheus.HistogramVec
}

func (m *Metrics) CustomErrorInc(name string, key, desc string) {
	m.lock.Lock()
	counter, exist := m.counter[name]
	if !exist {
		m.counter[name] = NewCounterVec(name, "gophercron", "center", []string{"key", "desc", "instance"})
		counter = m.counter[name]
	}
	m.lock.Unlock()

	counter.With(prometheus.Labels{
		"desc":     desc,
		"key":      key,
		"instance": m.instance,
	}).Inc()
}

func (m *Metrics) CustomHistogramSet(name string, keys ...string) *prometheus.Timer {
	m.lock.Lock()
	histogram, exist := m.histogram[name]
	if !exist {
		m.histogram[name] = NewHistogramVec(name, "gophercron", "center", []string{"key", "instance"})
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
