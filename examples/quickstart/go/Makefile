.PHONY: run-server run-client test

run-server:
	@echo "Starting Go quickstart server..."
	go run -tags server_only server.go

run-client:
	@echo "Running Go quickstart client..."
	go run -tags client_only client.go

test: run-server &
	@sleep 2
	@$(MAKE) run-client
