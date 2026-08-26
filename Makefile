BINARY  := dooray-mcp
VERSION ?= dev
DIST    := dist
LDFLAGS := -s -w -X main.version=$(VERSION)

# Every platform published in a release. Windows and macOS are the primary
# targets; Linux is included for CI and container use.
PLATFORMS := darwin/arm64 darwin/amd64 windows/amd64 windows/arm64 linux/amd64 linux/arm64

.PHONY: all build test vet fmt release clean

all: fmt vet test build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Cross-compiles every platform into dist/ and writes SHA256SUMS.
# CGO is disabled so the binaries are fully static and need no runtime.
release: clean
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out="$(DIST)/$(BINARY)_$${os}_$${arch}$${ext}"; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" . || exit 1; \
		if [ "$$os" = "windows" ]; then \
			(cd $(DIST) && zip -q "$(BINARY)_$${os}_$${arch}.zip" "$(BINARY)_$${os}_$${arch}$${ext}"); \
		else \
			(cd $(DIST) && tar czf "$(BINARY)_$${os}_$${arch}.tar.gz" "$(BINARY)_$${os}_$${arch}"); \
		fi; \
	done
	@cd $(DIST) && { command -v sha256sum >/dev/null && sha256sum *.tar.gz *.zip || shasum -a 256 *.tar.gz *.zip; } > SHA256SUMS
	@ls -1 $(DIST)

clean:
	@rm -rf $(DIST) $(BINARY) $(BINARY).exe

# Stages the cross-compiled binaries into the npm wrapper package so that
# `npx -y dooray-mcp-go` works without a postinstall download.
.PHONY: npm-stage npm-pack
npm-stage: release
	@rm -f npm/binaries/dooray-mcp_*
	@cp $(DIST)/dooray-mcp_darwin_* $(DIST)/dooray-mcp_windows_* $(DIST)/dooray-mcp_linux_* npm/binaries/ 2>/dev/null || true
	@rm -f npm/binaries/*.tar.gz npm/binaries/*.zip
	@ls -1 npm/binaries

npm-pack: npm-stage
	cd npm && npm pack
