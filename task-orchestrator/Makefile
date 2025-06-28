test-integration:
	go test -tags=integration ./... -v -timeout=30s

test-unit:
	go test -race -cover -count=1 ./...

test: test-unit test-integration

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

test-given-status:
	@if [ -n "$(ID)" ]; then \
		TASK_ID="$(ID)"; \
	elif [ ! -t 0 ]; then \
		TASK_ID=$$(cat); \
	else \
		echo "❌ Error: Task ID is required"; \
		echo "Usage: make check-task-advanced ID=<task-id>"; \
		echo "   or: echo '<task-id>' | make check-task-advanced"; \
		exit 1; \
	fi; \
	echo "🔍 Checking status for task: $$TASK_ID"; \
	curl -s -X GET http://localhost:8080/tasks/$$TASK_ID/status | jq '.'

.PHONY: test run test-print test-sleep test-health load-test test-status test-given-status test-integration test-unit