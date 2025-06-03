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

.PHONY: test run test-print test-sleep test-health