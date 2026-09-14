package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Received          *prometheus.CounterVec
	Accepted          *prometheus.CounterVec
	Parsed            *prometheus.CounterVec
	ParseErrors       *prometheus.CounterVec
	Stored            *prometheus.CounterVec
	Dropped           *prometheus.CounterVec
	Retried           *prometheus.CounterVec
	BytesReceived     *prometheus.CounterVec
	BytesStored       prometheus.Counter
	QueueEvents       prometheus.Gauge
	QueueBytes        prometheus.Gauge
	ActiveConnections *prometheus.GaugeVec
	StorageDuration   *prometheus.HistogramVec
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		Received:          prometheus.NewCounterVec(prometheus.CounterOpts{Name: "syslogx_messages_received_total", Help: "Messages read from a transport."}, []string{"transport"}),
		Accepted:          prometheus.NewCounterVec(prometheus.CounterOpts{Name: "syslogx_messages_accepted_total", Help: "Messages admitted to the bounded pipeline."}, []string{"transport"}),
		Parsed:            prometheus.NewCounterVec(prometheus.CounterOpts{Name: "syslogx_messages_parsed_total", Help: "Messages parsed by format."}, []string{"format"}),
		ParseErrors:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "syslogx_messages_parse_error_total", Help: "Parser errors by parser and reason."}, []string{"parser", "reason"}),
		Stored:            prometheus.NewCounterVec(prometheus.CounterOpts{Name: "syslogx_messages_stored_total", Help: "Messages accepted by storage."}, []string{"backend"}),
		Dropped:           prometheus.NewCounterVec(prometheus.CounterOpts{Name: "syslogx_messages_dropped_total", Help: "Messages dropped by stage and bounded reason."}, []string{"stage", "reason"}),
		Retried:           prometheus.NewCounterVec(prometheus.CounterOpts{Name: "syslogx_messages_retried_total", Help: "Storage retry attempts."}, []string{"backend"}),
		BytesReceived:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "syslogx_bytes_received_total", Help: "Payload bytes read from a transport."}, []string{"transport"}),
		BytesStored:       prometheus.NewCounter(prometheus.CounterOpts{Name: "syslogx_bytes_stored_total", Help: "Encoded bytes accepted by storage."}),
		QueueEvents:       prometheus.NewGauge(prometheus.GaugeOpts{Name: "syslogx_ingestion_queue_events", Help: "Events currently held in the ingestion queue."}),
		QueueBytes:        prometheus.NewGauge(prometheus.GaugeOpts{Name: "syslogx_ingestion_queue_bytes", Help: "Payload bytes currently held in the ingestion queue."}),
		ActiveConnections: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "syslogx_active_connections", Help: "Active stream connections."}, []string{"transport"}),
		StorageDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "syslogx_storage_request_duration_seconds", Help: "Storage append latency.", Buckets: prometheus.DefBuckets}, []string{"backend", "outcome"}),
	}
	reg.MustRegister(m.Received, m.Accepted, m.Parsed, m.ParseErrors, m.Stored, m.Dropped, m.Retried, m.BytesReceived, m.BytesStored, m.QueueEvents, m.QueueBytes, m.ActiveConnections, m.StorageDuration)
	return m
}
