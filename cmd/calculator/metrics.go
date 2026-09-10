package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type rpsBucket struct {
	second int64
	count  uint64
}

var (
	rpsMu      sync.Mutex
	rpsBuckets [60]rpsBucket

	httpRPS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_rps",
			Help: "Requests per second for the last 60 seconds",
		},
		[]string{"seconds_ago"},
	)
)

var cCallDuration = promauto.NewSummary(prometheus.SummaryOpts{
	Name: "c_call_duration_seconds",
	Help: "C function call duration in seconds",
	Objectives: map[float64]float64{
		0.95: 0.01,
		0.99: 0.001,
	},
})

var rustCallDuration = promauto.NewSummary(prometheus.SummaryOpts{
	Name: "rust_call_duration_seconds",
	Help: "Rust function call duration in seconds",
	Objectives: map[float64]float64{
		0.95: 0.01,
		0.99: 0.001,
	},
})

func recordRequest() {
	second := time.Now().Unix()
	index := int(second % 60)

	rpsMu.Lock()
	defer rpsMu.Unlock()

	bucket := &rpsBuckets[index]

	if bucket.second != second {
		bucket.second = second
		bucket.count = 0
	}

	bucket.count++
}

func updateRPSMetric() {
	now := time.Now().Unix()

	rpsMu.Lock()
	defer rpsMu.Unlock()

	for secondsAgo := 0; secondsAgo < 60; secondsAgo++ {
		second := now - int64(secondsAgo)
		index := int(second % 60)

		var count uint64

		if rpsBuckets[index].second == second {
			count = rpsBuckets[index].count
		}

		httpRPS.
			WithLabelValues(strconv.Itoa(secondsAgo)).
			Set(float64(count))
	}
}
