package api_test

import (
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/api"
)

func TestSampleHostTelemetry(t *testing.T) {
	cpu, memUsed, memPct := api.SampleHostTelemetry()

	// Wait 500ms to allow tick delta
	time.Sleep(500 * time.Millisecond)
	cpu, memUsed, memPct = api.SampleHostTelemetry()

	if cpu < 0 || cpu > 100 {
		t.Errorf("cpu percent should be between 0 and 100, got %f", cpu)
	}
	if memPct < 0 || memPct > 100 {
		t.Errorf("mem percent should be between 0 and 100, got %f", memPct)
	}
	if memUsed < 0 {
		t.Errorf("mem used should be >= 0, got %d", memUsed)
	}
	t.Logf("Sampled Host Telemetry: CPU=%.2f%%, MemUsed=%d bytes, MemPercent=%.2f%%", cpu, memUsed, memPct)
}
