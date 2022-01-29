DOCKER ?= docker
DOCKER_REPO ?= omnibrian
DOCKER_IMAGE_NAME ?= podman-exporter
DOCKER_IMAGE_TAG ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))
DOCKER_MAJOR_VERSION_TAG = $(firstword $(subst ., ,$(shell cat VERSION)))
DOCKERFILE_PATH ?= ./Dockerfile
DOCKERBUILD_CONTEXT ?= ./

GO ?= go
GO_OPTS ?=
GOFMT ?= $(GO)fmt
GOLINT ?= golangci-lint
GOLINT_OPTS ?=

PKGS = ./...
MAIN = podman_exporter

all: check-fmt lint build

.PHONY: check-fmt
check-fmt:
	@echo ">> checking code style"
	@fmtOutput=$$($(GOFMT) -d .); \
	if [ -n "$${fmtOutput}" ]; then \
		echo "failed $(GOFMT) check"; \
		echo "$${fmtOutput}"; \
		exit 1; \
	fi

fmt:
	@echo ">> formatting code"
	$(GO) fmt $(pkgs)

.PHONY: vet
vet:
	@echo ">> vetting code"
	$(GO) vet $(GO_OPTS) $(pkgs)

.PHONY: lint
lint: check-fmt vet
	@echo ">> running $(GOLINT)"
	$(GOLINT) run $(GOLINT_OPTS) $(pkgs)

start: fmt vet
	@echo ">> starting dev server"
	$(GO) run ./$(MAIN).go

build: check-fmt vet
	@echo ">> building binary"
	$(GO) build -o bin/$(MAIN) ./$(MAIN).go

test: check-fmt vet
	@echo ">> running all tests"
	$(GO) test $(GO_OPTS) $(pkgs)

.PHONY: docker-build
docker-build: build
	@echo ">> building docker image"
	$(DOCKER) build -t "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" \
		-f $(DOCKERFILE_PATH) \
		$(DOCKERBUILD_CONTEXT)

.PHONY: docker-push
docker-push: docker-build
	@echo ">> pushing docker image"
	$(DOCKER) push "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)"

.PHONY: docker-latest
docker-latest: docker-push
	@echo ">> adding version and latest tags to docker image"
	$(DOCKER) tag "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):latest"
	$(DOCKER) push "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):latest"
	$(DOCKER) tag "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):v$(DOCKER_MAJOR_VERSION_TAG)"
	$(DOCKER) push "$(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):$(DOCKER_MAJOR_VERSION_TAG)"
