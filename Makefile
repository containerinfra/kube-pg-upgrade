BINARY = kube-pg-upgrade
GOARCH = amd64
E2E_KIND_CLUSTER_NAME ?= kube-pg-upgrade-e2e

COMMIT=$(shell git rev-parse HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

BUILD=0
VERSION=0.0.0
IMAGE=containerinfra/kube-pg-upgrade

ifneq (${BRANCH}, release)
	BRANCH := -${BRANCH}
else
	BRANCH :=
endif

PKG_LIST := $(shell go list ./... | grep -v /vendor/)
LDFLAGS = -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Branch=${BRANCH}"

all: link clean linux darwin

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o bin/${BINARY}-linux-${GOARCH} . ;

darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=${GOARCH} go build ${LDFLAGS} -o bin/${BINARY}-darwin-${GOARCH} . ;

windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=${GOARCH} go build ${LDFLAGS} -o bin/${BINARY}-windows-${GOARCH}.exe . ;

build:
	CGO_ENABLED=0 go build ${LDFLAGS} -o bin/${BINARY} . ;
	chmod +x bin/${BINARY};

test: ## Run unittests
	@go test -short ${PKG_LIST}

e2e-kind-up: ## Create (or reuse) an amd64 kind cluster for E2E tests
	@chmod +x scripts/e2e-kind.sh
	@E2E_KIND_CLUSTER_NAME="$(E2E_KIND_CLUSTER_NAME)" ./scripts/e2e-kind.sh up

e2e-kind-down: ## Delete the E2E kind cluster
	@chmod +x scripts/e2e-kind.sh
	@E2E_KIND_CLUSTER_NAME="$(E2E_KIND_CLUSTER_NAME)" ./scripts/e2e-kind.sh down

test-e2e: build e2e-kind-up ## Run E2E tests against kind (requires docker, kind, kubectl)
	@E2E_KIND_CLUSTER_NAME="$(E2E_KIND_CLUSTER_NAME)" ./scripts/e2e-kind.sh kubeconfig > /tmp/kube-pg-upgrade-e2e.kubeconfig
	@KUBECONFIG=/tmp/kube-pg-upgrade-e2e.kubeconfig \
		KUBE_PG_UPGRADE_BIN="$(CURDIR)/bin/${BINARY}" \
		E2E_KIND_CLUSTER_NAME="$(E2E_KIND_CLUSTER_NAME)" \
		go test -tags=e2e -count=1 -timeout 90m -v ./test/e2e/...

fmt:
	@go fmt ${PKG_LIST};

docker:
	docker build -t ${IMAGE}:dev .

clean:
	-rm -f bin/${BINARY}-* bin/${BINARY}

.PHONY: link linux darwin windows test e2e-kind-up e2e-kind-down test-e2e fmt clean
