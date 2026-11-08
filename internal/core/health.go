package core

import "time"

// ServiceHealth describes whether starting another transfer is safe.
// Messages deliberately exclude database locations and engine output.
type ServiceHealth struct {
	Ready      bool       `json:"ready"`
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	NextAction string     `json:"next_action,omitempty"`
	Since      *time.Time `json:"since,omitempty"`
}

func readyHealth() ServiceHealth {
	return ServiceHealth{Ready: true, Code: "ready", Message: "Task state is available"}
}

func storageHealth(since time.Time) ServiceHealth {
	return ServiceHealth{
		Code: "storage_unavailable", Message: "Task state could not be saved; new transfers are suspended",
		NextAction: "Restore writable storage, then recover the queue", Since: cloneTime(&since),
	}
}

// An unknown identity remains blocked. Legacy records never gain ownership
// merely because their recorded PID happens to exist on the next startup.
func interruptedProcessActive(job Job) bool {
	if job.PID <= 0 || !processExists(job.PID) {
		return false
	}
	if job.ProcessIdentity == "" {
		return true
	}
	current := processIdentity(job.PID)
	return current == "" || current == job.ProcessIdentity
}
