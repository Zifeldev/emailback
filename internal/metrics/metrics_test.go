package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRegisterAndIncrementMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()


	EmailsProcessed = prometheus.NewCounter(prometheus.CounterOpts{Name: "emailback_emails_processed_total", Help: ""})
	EmailsFailed = prometheus.NewCounter(prometheus.CounterOpts{Name: "emailback_emails_failed_total", Help: ""})
	EmailProcessingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "emailback_email_processing_seconds", Help: ""})

	if err := reg.Register(EmailsProcessed); err != nil {
		t.Fatalf("register processed: %v", err)
	}
	if err := reg.Register(EmailsFailed); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if err := reg.Register(EmailProcessingDuration); err != nil {
		t.Fatalf("register hist: %v", err)
	}

	EmailsProcessed.Inc()
	EmailsProcessed.Add(2)
	EmailsFailed.Inc()
	EmailProcessingDuration.Observe(0.1)
	EmailProcessingDuration.Observe(0.5)

	if v := testutil.ToFloat64(EmailsProcessed); v != 3 {
		t.Fatalf("emails processed expected 3, got %v", v)
	}
	if v := testutil.ToFloat64(EmailsFailed); v != 1 {
		t.Fatalf("emails failed expected 1, got %v", v)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "emailback_email_processing_seconds" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("histogram metric not gathered")
	}
}
