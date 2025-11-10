package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	EmailsProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "emailback_emails_processed_total",
		Help: "Total number of successfully processed emails",
	})

	EmailsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "emailback_emails_failed_total",
		Help: "Total number of emails that failed to process",
	})

	EmailProcessingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "emailback_email_processing_seconds",
		Help:    "Histogram of email processing durations in seconds",
		Buckets: prometheus.DefBuckets,
	})
)


func RegisterMetrics() {
	prometheus.MustRegister(EmailsProcessed)
	prometheus.MustRegister(EmailsFailed)
	prometheus.MustRegister(EmailProcessingDuration)
}
