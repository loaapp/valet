.PHONY: all build daemon cli app clean dev restart vet dmg sign notarize

VERSION ?= 0.1.0
APPLE_IDENTITY ?= Developer ID Application: Richard Clayton

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
	@go build -o valetapp/build/bin/Valet.app/Contents/MacOS/valetd \
		github.com/loaapp/valet/valetd/cmd/valetd
	@go build -o valetapp/build/bin/Valet.app/Contents/MacOS/valet \
		github.com/loaapp/valet/valetd/cmd/valet
	@echo "Built Valet.app with bundled daemon"

dmg: app
	@rm -f Valet-$(VERSION).dmg
	@create-dmg --volname "Valet" --window-size 600 400 \
		--icon-size 100 --icon "Valet.app" 175 190 \
		--app-drop-link 425 190 --no-internet-enable \
		"Valet-$(VERSION).dmg" "valetapp/build/bin/" || true
	@echo "Created Valet-$(VERSION).dmg"

sign: app
	@codesign --force --deep --timestamp --options=runtime \
		-s "$(APPLE_IDENTITY)" valetapp/build/bin/Valet.app
	@echo "Signed Valet.app"

notarize: dmg
	@xcrun notarytool submit Valet-$(VERSION).dmg \
		--keychain-profile "valet-notary" --wait
	@xcrun stapler staple Valet-$(VERSION).dmg
	@echo "Notarized Valet-$(VERSION).dmg"

dev:
	@cd valetapp && wails dev

restart: daemon
	@-pkill -f "bin/valetd" 2>/dev/null; sleep 1
	@bin/valetd -dns="" > /dev/null 2>&1 &
	@echo "valetd restarted"

clean:
	@rm -rf bin/
	@rm -rf valetapp/build/bin/
	@rm -f Valet-*.dmg
	@echo "Cleaned"

vet:
	@go vet github.com/loaapp/valet/valetd/...
	@go vet github.com/loaapp/valet/pkg/...
	@go vet github.com/loaapp/valet/valetapp/...
	@echo "Vet passed"
