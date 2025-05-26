# 🧱 Task Orchestrator

Build a job orchestration service in Go that accepts tasks over HTTP and processes them asynchronously via a message queue.

This project aims to exercise real-world software architecture skills: decoupling, modularity, resilience, and observability.


## 🧩 Project Goals

- Accept task submissions (`POST /submit`)
- Queue tasks for background processing
- Execute tasks via workers
- Evolve system architecture over versions


## 🔁 Evolution Stages

1. **V1 – In-Process Runner**  
   Simple job submission + direct execution (goroutines).

2. **V2 – Message Queue Integration**  
   Decouple API and workers using a queue (e.g. NATS, Redis).

3. **V3 – Modular & Observable**  
   Job interfaces, pluggable handlers, metrics, retries, dead-letter queue.

4. **V4 – Scalable & Persistent**  
   Multi-worker, graceful shutdown, persistence (Postgres/SQLite).

