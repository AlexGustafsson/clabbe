package state

import "github.com/prometheus/client_golang/prometheus"

var _ prometheus.Collector = (*Metrics)(nil)

type Metrics struct {
	SongsPlayed    prometheus.Counter
	DurationPlayed prometheus.Counter
	ActiveStreams  prometheus.Gauge
}

func NewMetrics() *Metrics {
	return &Metrics{
		SongsPlayed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "clabbe",
			Subsystem: "core",
			Name:      "songs_played_total",
			Help:      "Total number of songs for which playback was started",
		}),
		DurationPlayed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "clabbe",
			Subsystem: "core",
			Name:      "duration_played_total_seconds",
			Help:      "Total number of seconds music has been played",
		}),
		ActiveStreams: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "clabbe",
			Subsystem: "core",
			Name:      "active_streams",
			Help:      "Number of currently active streams",
		}),
	}
}

// Collect implements prometheus.Collector.
func (m *Metrics) Collect(c chan<- prometheus.Metric) {
	m.SongsPlayed.Collect(c)
	m.DurationPlayed.Collect(c)
}

// Describe implements prometheus.Collector.
func (m *Metrics) Describe(d chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(m, d)
}
