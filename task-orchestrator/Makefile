run:
	go run ./cmd/task-orchestrator

test-print:
	curl -X POST http://localhost:8080/submit \
	  -H "Content-Type: application/json" \
	  -d '{"type": "print", "payload": {"message": "Hello from Juan!"}}'

test-sleep:
	curl -X POST http://localhost:8080/submit \
	  -H "Content-Type: application/json" \
	  -d '{"type": "sleep", "payload": {"seconds": 2}}'