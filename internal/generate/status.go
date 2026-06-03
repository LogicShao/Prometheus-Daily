package generate

import "time"

type Status struct {
	Running       bool       `json:"running"`
	LastRun       *time.Time `json:"last_run,omitempty"`
	LastSuccess   bool       `json:"last_success"`
	LastFile      string     `json:"last_file,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	Attempts      int        `json:"attempts"`
	MaxAttempts   int        `json:"max_attempts"`
	LastStage     string     `json:"last_stage,omitempty"`
	AttemptErrors []string   `json:"attempt_errors,omitempty"`
	TodayDate     string     `json:"today_date,omitempty"`
	TodayReady    bool       `json:"today_ready"`
	Uptime        string     `json:"uptime,omitempty"`
}

type Result struct {
	Date     string `json:"date"`
	File     string `json:"file"`
	Summary  string `json:"summary"`
	Attempts int    `json:"attempts"`
}
