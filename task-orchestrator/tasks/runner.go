package tasks

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type Runner interface {
	Run(task *Task) error
}

type BasicRunner struct{}

func (r *BasicRunner) Run(task *Task) error {
	log.Printf("running task: %+v\n", task)
	switch task.Type {
	case "print":
		var p PrintPayload
		if err := json.Unmarshal(task.Payload, &p); err != nil {
			return fmt.Errorf("invalid payload for print task %s: %w", task.Payload, err)
		}
		log.Printf("Executing print task: %s\n", p.Message)
		task.Result = "printed: " + p.Message

	case "sleep":
		var p SleepPayload
		if err := json.Unmarshal(task.Payload, &p); err != nil {
			return fmt.Errorf("invalid payload for sleep task %s: %w", task.Payload, err)
		}

		log.Printf("Executing sleep task: %d seconds\n", p.Seconds)
		time.Sleep(time.Duration(p.Seconds) * time.Second)
		task.Result = fmt.Sprintf("slept: %d seconds", p.Seconds)

	default:
		return fmt.Errorf("unsupported task type: %s", task.Type)
	}
	log.Printf("task runned sucessfully: %+v\n", task)

	task.Status = "done"
	return nil
}
