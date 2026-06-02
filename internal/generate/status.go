package generate

import "time"

type Status struct {
	Running     bool       `json:"running"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	LastSuccess bool       `json:"last_success"`
	LastFile    string     `json:"last_file,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	TodayDate   string     `json:"today_date,omitempty"`
	TodayReady  bool       `json:"today_ready"`
	Uptime      string     `json:"uptime,omitempty"`
}

type Result struct {
	Date    string `json:"date"`
	File    string `json:"file"`
	Summary string `json:"summary"`
}
