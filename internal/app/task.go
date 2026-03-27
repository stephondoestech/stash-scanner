package app

import "time"

type StashTaskStatus struct {
	ID          string    `json:"id,omitempty"`
	Status      string    `json:"status,omitempty"`
	Description string    `json:"description,omitempty"`
	Progress    float64   `json:"progress"`
	AddedAt     time.Time `json:"added_at,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	EndedAt     time.Time `json:"ended_at,omitempty"`
	Error       string    `json:"error,omitempty"`
}

func (t StashTaskStatus) active() bool {
	switch t.Status {
	case "READY", "RUNNING", "STOPPING":
		return true
	default:
		return false
	}
}
