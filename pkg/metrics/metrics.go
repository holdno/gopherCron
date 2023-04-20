package metrics

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/spacegrower/watermelon/infra/wlog"
	"go.uber.org/zap"
)

func RegisterGoMetrics(r *prometheus.Registry) {
	r.Register(collectors.NewGoCollector(collectors.WithGoCollections(collectors.GoRuntimeMetricsCollection)))
}

func (m *Metrics) NewCounterVec(name, ns, system string, labels []string) *prometheus.CounterVec {
	if !strings.HasSuffix(name, "_count") {
		name += "_count"
	}
	// counterVec monitor error count
	var counterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: system,
			Name:      name,
			Help:      fmt.Sprintf("%s count of /%s/%s", name, ns, system),
		},
		labels,
	)

	m.registry.MustRegister(counterVec)
	return counterVec
}

func (m *Metrics) NewGaugeVec(name, ns, system string, labels []string) *prometheus.GaugeVec {
	if !strings.HasSuffix(name, "_gauge") {
		name += "_gauge"
	}
	// errorCounterVec monitor error count
	var gaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: system,
			Name:      name,
			Help:      fmt.Sprintf("%s gauge of /%s/%s", name, ns, system),
		},
		labels,
	)

	m.registry.MustRegister(gaugeVec)
	return gaugeVec
}

func (m *Metrics) NewHistogramVec(name, ns, system string, labels []string) *prometheus.HistogramVec {
	if !strings.HasSuffix(name, "_duration") {
		name += "_duration"
	}
	// timerHistogramVec monitor something duration
	var timerHistogramVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: ns,
			Subsystem: system,
			Name:      name,
			Help:      fmt.Sprintf("%s duration of /%s/%s", name, ns, system),
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

func setupPusherFromEnv(ctx context.Context, service string) *push.Pusher {
	endpoint := os.Getenv("PROM_PUSHGATEWAY_ENDPOINT")
	job := os.Getenv("PROM_PUSHGATEWAY_JOB_NAME")

	if endpoint == "" || job == "" {
		return nil
	}

	username := os.Getenv("PROM_PUSHGATEWAY_USERNAME")
	password := os.Getenv("PROM_PUSHGATEWAY_PASSWORD")

	pusher := push.New(endpoint, job).
		Grouping("service", service).
		Grouping("uuid", ulid.Make().String())
	if username != "" {
		pusher.BasicAuth(username, password)
	}

	clearOldMetrics(endpoint, job)

	go func() {
		ticker := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if err := pusher.Push(); err != nil {
					wlog.Error("prometheus failed to push metrics", zap.Error(err))
				}
			}
		}
	}()

	return pusher
}
