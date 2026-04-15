.PHONY: all build daemon cli app clean dev restart vet

all: build

build: daemon cli app

daemon:
	@go build -o bin/valetd github.com/loaapp/valet/valetd/cmd/valetd
	@echo "Built bin/valetd"

cli:
	@go build -o bin/valet github.com/loaapp/valet/valetd/cmd/valet
	@echo "Built bin/valet"

app:
	@cd valetapp && wails build

dev:
	@cd valetapp && wails dev

restart: daemon
	@-pkill -f "bin/valetd" 2>/dev/null; sleep 1
	@bin/valetd -dns="" > /dev/null 2>&1 &
	@echo "valetd restarted"

clean:
	@rm -rf bin/
	@rm -rf valetapp/build/bin/
	@echo "Cleaned"

vet:
	@go vet github.com/loaapp/valet/valetd/...
	@go vet github.com/loaapp/valet/pkg/...
	@go vet github.com/loaapp/valet/valetapp/...
	@echo "Vet passed"
