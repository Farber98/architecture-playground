package tasks

import "encoding/json"

type Task struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	// For delayed decoding until we know task Type
	Payload json.RawMessage `json:"payload"`
	// Lifecycle of the task (Where is the task now?)
	Status string `json:"status"`
	// Output of the task: business level outcoume (What happened?)
	Result string `json:"result"`
}

type PrintPayload struct {
	Message string `json:"message"`
}

type SleepPayload struct {
	Seconds int `json:"seconds"`
}
