.PHONY: all build daemon cli app clean dev restart vet pkg sign

VERSION ?= 0.1.0

all: build

build: daemon cli app

daemon:
	@cd valetd && go build -o ../bin/valetd ./cmd/valetd
	@echo "Built bin/valetd"

cli:
	@cd valetd && go build -o ../bin/valet ./cmd/valet
	@echo "Built bin/valet"

app:
	@cd valetapp && wails build
	@cd valetd && go build -o ../valetapp/build/bin/Valet.app/Contents/MacOS/valetd ./cmd/valetd
	@mkdir -p valetapp/build/bin/Valet.app/Contents/Resources/bin
	@cd valetd && go build -o ../valetapp/build/bin/Valet.app/Contents/Resources/bin/valet ./cmd/valet
	@echo "Built Valet.app with bundled daemon"

pkg:
	@VERSION=$(VERSION) ./scripts/build-pkg.sh

pkg-unsigned:
	@VERSION=$(VERSION) ./scripts/build-pkg.sh --no-notarize

dev:
	@cd valetapp && wails dev

restart: daemon
	@-pkill -f "bin/valetd" 2>/dev/null; sleep 1
	@bin/valetd -dns="" > /dev/null 2>&1 &
	@echo "valetd restarted"

clean:
	@rm -rf bin/ build/
	@rm -rf valetapp/build/bin/
	@echo "Cleaned"

vet:
	@(cd valetd && go vet ./...)
	@(cd pkg && go vet ./...)
	@(cd valetapp && go vet ./...)
	@echo "Vet passed"
