.PHONY: build test integration clean

build:
	go build -o bin/ionic-telegraf-plugin ./cmd

test:
	go test ./plugins/inputs/nicctl/... ./internal/exec/...

integration:
	INTEGRATION_HOST=10.9.10.145 go test -tags integration ./tests/integration/...

clean:
	rm -rf bin/
