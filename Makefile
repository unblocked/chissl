VERSION=$(shell git describe --abbrev=0 --tags)
BUILD=$(shell git rev-parse HEAD)
DIRBASE=./build
DIR=${DIRBASE}/${VERSION}/${BUILD}/bin

LDFLAGS=-ldflags "-s -w ${XBUILD} -buildid=${BUILD} -X github.com/NextChapterSoftware/chissl/share.BuildVersion=${VERSION}"

GOFILES=`go list ./...`
GOFILESNOTEST=`go list ./... | grep -v test`

# Make Directory to store executables
$(shell mkdir -p ${DIR})

all:
	@goreleaser build --skip-validate --single-target --config .github/goreleaser.yml

freebsd: lint
	env CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-freebsd_amd64 .

linux: lint
	env CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-linux_amd64 .

linux-on-darwin: lint
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-linux_amd64 .

# Install cross-compilers
# brew tap messense/macos-cross-toolchains
# brew install x86_64-unknown-linux-gnu
linux-on-darwin-with-cgo: lint
	GOARCH=amd64 GOOS=linux CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc go build -tags server -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-linux_amd64 .

linux-on-darwin-with-cgo-with-tests: lint security-tests
	GOARCH=amd64 GOOS=linux CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc go build -tags server -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-linux_amd64 .


make-linux-in-docker-on-darwin: lint
	@docker run --rm -v "$$PWD":/src -w /src golang:1.22-bookworm bash -lc "set -euo pipefail; apt-get update; apt-get install -y gcc g++ make pkg-config; make linux"


# Install/ensure macOS cross-compilers for Linux builds
macos-toolchains:
	@if [ "$(shell uname -s)" != "Darwin" ]; then echo "This target is intended for macOS (Darwin). Detected: $(shell uname -s)"; exit 1; fi
	@if ! command -v brew >/dev/null 2>&1; then echo "Homebrew (brew) is required. Install from https://brew.sh"; exit 1; fi
	@set -euo pipefail; \
	  echo "[toolchains] Checking and cleaning problematic Homebrew taps..."; \
	  for t in homebrew/homebrew-cask-versions homebrew/cask-versions; do \
	    if brew tap | grep -q "^$$t$$"; then \
	      echo "[toolchains] Untapping $$t"; \
	      brew untap $$t || true; \
	    fi; \
	  done; \
	  TAP_DIR="$$(brew --repository)/Library/Taps/homebrew/homebrew-cask-versions"; \
	  if [ -d "$$TAP_DIR" ]; then echo "[toolchains] Removing leftover tap dir $$TAP_DIR"; rm -rf "$$TAP_DIR"; fi; \
	  echo "[toolchains] Running brew update..."; \
	  brew update || true; \
	  echo "[toolchains] Tapping messense/macos-cross-toolchains..."; \
	  brew tap messense/macos-cross-toolchains || true; \
	  echo "[toolchains] Installing/ensuring cross-compilers..."; \
	  brew ls --versions x86_64-unknown-linux-gnu >/dev/null 2>&1 || brew install x86_64-unknown-linux-gnu || true; \
	  brew ls --versions aarch64-unknown-linux-gnu >/dev/null 2>&1 || brew install aarch64-unknown-linux-gnu || true; \
	  brew ls --versions arm-unknown-linux-gnueabihf >/dev/null 2>&1 || brew install arm-unknown-linux-gnueabihf || true; \
	  echo "[toolchains] Ensuring expected compiler command names are available..."; \
	  BREW_PREFIX="$$(brew --prefix)"; \
	  TARGET_BIN="$$BREW_PREFIX/bin"; \
	  if [ ! -w "$$TARGET_BIN" ]; then TARGET_BIN="$$HOME/.local/bin"; fi; \
	  mkdir -p "$$TARGET_BIN"; \
	  if ! command -v x86_64-linux-gnu-gcc >/dev/null 2>&1 && command -v x86_64-unknown-linux-gnu-gcc >/dev/null 2>&1; then \
	    ln -sf "$$(command -v x86_64-unknown-linux-gnu-gcc)" "$$TARGET_BIN/x86_64-linux-gnu-gcc"; \
	  fi; \
	  if ! command -v aarch64-linux-gnu-gcc >/dev/null 2>&1 && command -v aarch64-unknown-linux-gnu-gcc >/dev/null 2>&1; then \
	    ln -sf "$$(command -v aarch64-unknown-linux-gnu-gcc)" "$$TARGET_BIN/aarch64-linux-gnu-gcc"; \
	  fi; \
	  if ! command -v arm-linux-gnueabihf-gcc >/dev/null 2>&1 && command -v arm-unknown-linux-gnueabihf-gcc >/dev/null 2>&1; then \
	    ln -sf "$$(command -v arm-unknown-linux-gnueabihf-gcc)" "$$TARGET_BIN/arm-linux-gnueabihf-gcc"; \
	  fi; \
	  case ":$$PATH:" in *":$$TARGET_BIN:"*) ;; *) echo "[toolchains] NOTE: Add $$TARGET_BIN to your PATH to use created symlinks."; ;; esac; \
	  echo "[toolchains] Done."

windows: lint
	env CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-windows_amd64 .

darwin:
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-darwin_amd64 .

docker:
	@docker build .

dep: ## Get the dependencies
	@go get -u github.com/goreleaser/goreleaser
	@go get -u github.com/boumenot/gocover-cobertura
	@go get -v -d ./...
	@go get -u all
	@go mod tidy

lint: ## Lint the files
	@go fmt ${GOFILES}
	@go vet ${GOFILESNOTEST}

test: ## Run unit tests
	@go test ./... -coverprofile=${DIR}/coverage.out -race -short
	@go tool cover -html=${DIR}/coverage.out -o ${DIR}/coverage.html
	$(MAKE) security-tests

security-tests: ## Run security tests to verify API authorization and authentication
	@echo "Running security tests..."
	@go test ./tests -v -timeout 30s
	@echo "Security tests completed successfully!"

release: lint test
	goreleaser release --config .github/goreleaser.yml

clean:
	rm -rf ${DIRBASE}/*

.PHONY: all freebsd linux windows docker dep lint test security-tests linux-on-darwin-with-cgo-with-tests make-linux-in-docker-on-darwin macos-toolchains release clean

# --- Docs helpers -----------------------------------------------------------
.PHONY: docs-serve docs-build

# Serve docs locally on http://127.0.0.1:8001
# Prefer MkDocs if installed and mkdocs.yml exists; otherwise, serve static docs/
docs-serve:
	@if command -v mkdocs >/dev/null 2>&1 && [ -f mkdocs.yml ]; then \
		echo "[docs] Serving with MkDocs on http://127.0.0.1:8001"; \
		mkdocs serve -a 127.0.0.1:8001 || (echo "[docs] MkDocs failed to serve. Falling back to static server" && python3 -m http.server -d docs 8001); \
	else \
		echo "[docs] MkDocs not found. Serving static docs/ via Python http.server on http://127.0.0.1:8001"; \
		python3 -m http.server -d docs 8001; \
	fi

# Build static site using MkDocs (outputs to site/) if available; otherwise no-op
docs-build:
	@if command -v mkdocs >/dev/null 2>&1 && [ -f mkdocs.yml ]; then \
		echo "[docs] Building MkDocs site into ./site"; \
		mkdocs build -d site; \
	else \
		echo "[docs] MkDocs not installed; skipping build. (Install: pip install mkdocs mkdocs-material)"; \
	fi


# ----------------------
# Client builds (CGO disabled)
# ----------------------
client-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-darwin_amd64 .

client-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-darwin_arm64 .

client-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-windows_amd64.exe .

client-windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-windows_arm64.exe .

client-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-linux_amd64 .

client-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-linux_arm64 .

client-linux-armv7:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-linux_armv7 .

client-all: client-darwin-amd64 client-darwin-arm64 client-windows-amd64 client-windows-arm64 client-linux-amd64 client-linux-arm64 client-linux-armv7

# ----------------------
# Server Linux builds (CGO enabled, server build tag)
# On macOS (Darwin), use cross-compilers similar to linux-on-darwin-with-cgo
# ----------------------
server-linux-amd64:
ifeq ($(shell uname -s),Darwin)
	GOARCH=amd64 GOOS=linux CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc go build -tags server -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-server-linux_amd64 .
else
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags server -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-server-linux_amd64 .
endif

server-linux-arm64:
ifeq ($(shell uname -s),Darwin)
	CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -tags server -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-server-linux_arm64 .
else
	CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -tags server -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-server-linux_arm64 .
endif

server-linux-armv7:
ifeq ($(shell uname -s),Darwin)
	CC=arm-linux-gnueabihf-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 go build -tags server -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-server-linux_armv7 .
else
	CC=arm-linux-gnueabihf-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 go build -tags server -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-server-linux_armv7 .
endif

server-linux-all:
ifeq ($(shell uname -s),Darwin)
	$(MAKE) macos-toolchains
endif
	$(MAKE) server-linux-amd64
	$(MAKE) server-linux-arm64
	$(MAKE) server-linux-armv7
