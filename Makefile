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
	GOARCH=amd64 GOOS=linux CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-linux_amd64 .

linux-on-darwin-with-cgo-with-tests: lint security-tests
	GOARCH=amd64 GOOS=linux CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chissl-linux_amd64 .


make-linux-in-docker-on-darwin: lint
	@docker run --rm -v "$$PWD":/src -w /src golang:1.22-bookworm bash -lc "set -euo pipefail; apt-get update; apt-get install -y gcc g++ make pkg-config; make linux"

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

.PHONY: all freebsd linux windows docker dep lint test security-tests linux-on-darwin-with-cgo-with-tests release clean

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
