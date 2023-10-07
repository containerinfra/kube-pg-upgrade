BINARY = kube-pg-upgrade
GOARCH = amd64

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

fmt:
	@go fmt ${PKG_LIST};

docker:
	docker build -t ${IMAGE}:dev .

clean:
	-rm -f bin/${BINARY}-* bin/${BINARY}

.PHONY: link linux darwin windows test fmt clean
