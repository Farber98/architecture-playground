test:
	go test -race -cover -count=1 ./...

run:
	LOG_LEVEL=DEBUG PORT=8080 go run cmd/task-orchestrator/main.go

test-print:
	curl -X POST http://localhost:8080/submit \
	  -H "Content-Type: application/json" \
	  -d '{"type": "print", "payload": {"message": "Hello from Juan!"}}'

test-sleep:
	curl -X POST http://localhost:8080/submit \
	  -H "Content-Type: application/json" \
	  -d '{"type": "sleep", "payload": {"seconds": 2}}'

test-health:
	curl -X GET http://localhost:8080/health

test-load:
	@for i in $$(seq 1 100); do \
        curl -X POST http://localhost:8080/submit \
          -H "Content-Type: application/json" \
          -d "{\"type\": \"print\", \"payload\": {\"message\": \"Load test $$i\"}}" & \
    done; wait

test-status:
	@echo "=== Testing Task Status Endpoint ==="
	@echo "1. Submitting a task..."
	@TASK_ID=$$(curl -s -X POST http://localhost:8080/submit \
		-H "Content-Type: application/json" \
		-d '{"type": "print", "payload": {"message": "Status test task"}}' | \
		jq -r '.task_id'); \
	echo "Task ID: $$TASK_ID"; \
	echo "2. Checking task status..."; \
	curl -X GET http://localhost:8080/tasks/$$TASK_ID/status

.PHONY: test run test-print test-sleep test-health load-test test-status