package metrics

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func NewCounterVec(name, ns, system string, labels []string) *prometheus.CounterVec {
	if !strings.HasSuffix(name, "_count") {
		name += "_count"
	}
	// errorCounterVec monitor error count
	var errorCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: system,
			Name:      name,
			Help:      fmt.Sprintf("%s count of /%s/%s", name, ns, system),
		},
		labels,
	)

	prometheus.MustRegister(errorCounterVec)
	return errorCounterVec
}

func NewHistogramVec(name, ns, system string, labels []string) *prometheus.HistogramVec {
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
	prometheus.MustRegister(timerHistogramVec)
	return timerHistogramVec
}

func Middleware(namespace, service, instance string) gin.HandlerFunc {
	labelKeys := []string{"method", "instance"}
	serverTimerHistogramVec := NewHistogramVec("http_handle", namespace, service, labelKeys)
	labelKeys = append(labelKeys, "code", "desc")
	serverErrorCounterVec := NewCounterVec("http_handle_error", namespace, service, labelKeys)

	return func(c *gin.Context) {
		labels := prometheus.Labels{
			"method":   c.Request.RequestURI,
			"instance": instance,
		}

		timer := prometheus.NewTimer(serverTimerHistogramVec.With(labels))
		defer timer.ObserveDuration()

		for _, v := range labelKeys {
			if _, exist := labels[v]; !exist {
				labels[v] = ""
			}
		}

		c.Next()

		if err := c.Err(); err != nil {
			code := c.Request.Response.StatusCode
			labels["code"] = strconv.Itoa(code)
			labels["desc"] = err.Error()
			serverErrorCounterVec.With(labels).Inc()
			return
		}

		return
	}
}
