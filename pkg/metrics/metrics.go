package metrics

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

func RegisterGoMetrics(r *prometheus.Registry) {
	r.Register(collectors.NewGoCollector(collectors.WithGoCollections(collectors.GoRuntimeMetricsCollection)))
}

func (m *Metrics) NewCounterVec(name string, labels []string) *prometheus.CounterVec {
	if !strings.HasSuffix(name, "_count") {
		name += "_count"
	}

	// counterVec monitor error count
	var counterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.ns,
			Subsystem: m.serviceName,
			Name:      name,
			Help:      fmt.Sprintf("%s count of /%s/%s", name, m.ns, m.serviceName),
		},
		labels,
	)

	m.registry.MustRegister(counterVec)
	return counterVec
}

// func (m *Metrics) NewMetricVew(name, ns, system string, labels []string) {
// 	prometheus.NewMetricVec(prometheus.NewDesc(fmt.Sprintf("%s_%s_%s", ns, system, name, labels), fmt.Sprintf("%s count of /%s/%s", name, ns, system), labels, nil))
// }

func (m *Metrics) NewGaugeVec(name string, labels []string) *prometheus.GaugeVec {
	if !strings.HasSuffix(name, "_gauge") {
		name += "_gauge"
	}
	// errorCounterVec monitor error count
	var gaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.ns,
			Subsystem: m.serviceName,
			Name:      name,
			Help:      fmt.Sprintf("%s gauge of /%s/%s", name, m.ns, m.serviceName),
		},
		labels,
	)

	m.registry.MustRegister(gaugeVec)
	return gaugeVec
}

func (m *Metrics) NewHistogramVec(name string, labels []string) *prometheus.HistogramVec {
	if !strings.HasSuffix(name, "_duration") {
		name += "_duration"
	}
	// timerHistogramVec monitor something duration
	var timerHistogramVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.ns,
			Subsystem: m.serviceName,
			Name:      name,
			Help:      fmt.Sprintf("%s duration of /%s/%s", name, m.ns, m.serviceName),
		},
		labels,
	)
	m.registry.MustRegister(timerHistogramVec)
	return timerHistogramVec
}

func Middleware(srv *Metrics) gin.HandlerFunc {
	labelKeys := []string{"method"}
	serverTimerHistogramVec := srv.NewHistogram("http_handle", "method")
	labelKeys = append(labelKeys, "code", "desc")
	serverCounterVec := srv.NewCounter("http_handle", labelKeys...)

	return func(c *gin.Context) {
		timer := serverTimerHistogramVec(c.Request.URL.Path)
		defer timer.ObserveDuration()

		c.Next()

		code := c.Writer.Status()
		var errString string
		if err := c.Err(); err != nil {
			errString = err.Error()
		}
		serverCounterVec(c.Request.URL.Path, strconv.Itoa(code), errString)
	}
}

func clearOldMetrics(endpoint, jobName string) {
	endpoint = endpoint + "/metrics/job/" + jobName
	req, _ := http.NewRequest(http.MethodDelete, endpoint, nil)
	_, err := (&http.Client{}).Do(req)
	if err != nil {
		log.Printf("failed to clear old metric, job name: %s, error: %s", jobName, err.Error())
	}
}
