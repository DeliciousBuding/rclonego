package core

import (
	"testing"
	"time"
)

func TestSettingsPersistAndPendingParametersUseSameValidation(t *testing.T) {
	app := testApp(t)
	settings := defaultTransferSettings()
	settings.Transfers, settings.Checkers, settings.CPUParallelism = 1, 2, 1
	if err := app.SaveSettings(SettingsRequest{Defaults: &settings}); err != nil {
		t.Fatal(err)
	}
	job, err := app.queue.Add(JobCreateRequest{Name: "copy", Operation: "copy", Source: "s", Destination: "d"})
	if err != nil || job.Transfers != 1 || job.Checkers != 2 || job.CPUParallelism != 1 {
		t.Fatalf("defaults not shared: %+v / %v", job, err)
	}
	settings.ChunkSize = "3M"
	if err := app.queue.UpdateParameters(job.ID, JobUpdateRequest{Settings: settings}); err == nil {
		t.Fatal("invalid Drive chunk accepted")
	}
	if app.store.Snapshot().Jobs[0].ChunkSize != "16M" {
		t.Fatal("invalid edit changed the job")
	}
	settings.ChunkSize = "8M"
	job.Status = "running"
	if err := app.store.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	if err := app.queue.UpdateParameters(job.ID, JobUpdateRequest{Settings: settings}); err == nil {
		t.Fatal("running parameters were changed without restart")
	}
}

func TestCPUCounterNormalizationAndBufferBudget(t *testing.T) {
	if got := cpuUsage(1e7, time.Second, 8); got != 12.5 {
		t.Fatalf("CPU uses the wrong denominator: %v", got)
	}
	if got := defaultTransferSettings().BufferBudget(); got != 48<<20 {
		t.Fatalf("unexpected buffer estimate: %d", got)
	}
	for _, chunk := range []string{"0", "3M", "128K", "2G"} {
		s := defaultTransferSettings()
		s.ChunkSize = chunk
		if s.Validate() == nil {
			t.Fatalf("invalid chunk %s was accepted", chunk)
		}
	}
}
