# Run from this directory: cd demo && make <target>
# Optional: cp .env.example .env and edit — all run-* targets load .env when present.

.PHONY: help compose-up-local run-api run-worker run-api-otel run-worker-otel run-api-logs run-worker-logs run-api-dev run-worker-dev

help:
	@echo "Targets (loads .env if present):"
	@echo "  make compose-up-local - docker compose up without api/worker"
	@echo "  make run-api          - API only (go run)"
	@echo "  make run-worker       - worker only (go run)"
	@echo "  make run-api-otel     - API + Jaeger OTLP (OTEL_TRACING_ENABLED=true)"
	@echo "  make run-worker-otel  - worker + Jaeger OTLP"
	@echo "  make run-api-logs     - API + LOG_FILE=var/host-logs/video-api.log (Filebeat)"
	@echo "  make run-worker-logs  - worker + LOG_FILE=var/host-logs/video-worker.log"
	@echo "  make run-api-dev      - API + OTLP + log file (typical local stack)"
	@echo "  make run-worker-dev   - worker + OTLP + log file"

# shell: load .env then run command (paths relative to demo/)
define WITH_ENV
	bash -c 'set -a && [ -f .env ] && . ./.env; set +a && $(1)'
endef

compose-up-local:
	docker compose up -d localstack mongodb redis jaeger elasticsearch kibana filebeat

run-api:
	$(call WITH_ENV,exec go run ./api/cmd/server)

run-worker:
	$(call WITH_ENV,exec go run ./worker/cmd/worker)

run-api-otel:
	$(call WITH_ENV,export OTEL_TRACING_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=$${OTEL_EXPORTER_OTLP_ENDPOINT:-http://localhost:4318} && exec go run ./api/cmd/server)

run-worker-otel:
	$(call WITH_ENV,export OTEL_TRACING_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=$${OTEL_EXPORTER_OTLP_ENDPOINT:-http://localhost:4318} && exec go run ./worker/cmd/worker)

run-api-logs:
	@mkdir -p var/host-logs
	$(call WITH_ENV,export LOG_FILE=var/host-logs/video-api.log && exec go run ./api/cmd/server)

run-worker-logs:
	@mkdir -p var/host-logs
	$(call WITH_ENV,export LOG_FILE=var/host-logs/video-worker.log && exec go run ./worker/cmd/worker)

run-api-dev:
	@mkdir -p var/host-logs
	$(call WITH_ENV,export OTEL_TRACING_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=$${OTEL_EXPORTER_OTLP_ENDPOINT:-http://localhost:4318} LOG_FILE=var/host-logs/video-api.log && exec go run ./api/cmd/server)

run-worker-dev:
	@mkdir -p var/host-logs
	$(call WITH_ENV,export OTEL_TRACING_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=$${OTEL_EXPORTER_OTLP_ENDPOINT:-http://localhost:4318} LOG_FILE=var/host-logs/video-worker.log && exec go run ./worker/cmd/worker)
