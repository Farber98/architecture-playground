package main

import (
	"log"
	"net/http"
	handlers "task-orchestrator/handlers"
	"task-orchestrator/tasks"
)

func main() {
	http.HandleFunc("/submit", handlers.NewSubmitHandler(&tasks.BasicRunner{}))
	log.Println("Server listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
