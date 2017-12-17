# -*- mode: makefile-gmake -*-

# Inputs. Invoke from the parent makefile with these options set.
GOARCH:=
BINARY:=
TAG:=
REGISTRY:=
GOCMD:=go
DOCKER_SUDO:=

ifeq ($(GOARCH),)
	$(error GOARCH not specified)
endif
ifeq ($(BINARY),)
	$(error BINARY not specified)
endif
ifeq ($(TAG),)
	$(error TAG not specified)
endif
ifeq ($(REGISTRY),)
	$(error REGISTRY not specified)
endif

# Other variables.
GITCOMMIT:=$(shell git describe --dirty --always)
BUILD_DIR:=build/$(GOARCH)$(GOARM)/$(BINARY)
DOCKERCMD:=docker
ifneq ($(DOCKER_SUDO),)
	DOCKERCMD:=sudo docker
endif
DOCKERFILE_BASE:=alpine:latest
ifneq ($(GOARCH),amd64)
	DOCKERFILE_BASE:=$(GOARCH)/alpine:latest
endif
ifeq ($(GOARCH),arm)
	DOCKERFILE_BASE:=arm32v6/alpine:latest
endif
ifeq ($(GOARCH),arm64)
	DOCKERFILE_BASE:=arm64v8/alpine:latest
endif

.PHONY: push
push: image
	$(DOCKERCMD) push $(REGISTRY)/$(BINARY):$(TAG)

.PHONY: image
image: dockerfile binary
	$(DOCKERCMD) build -t $(REGISTRY)/$(BINARY):$(TAG) $(BUILD_DIR)

.PHONY: dockerfile
dockerfile: build-dir
	perl -pe "s#alpine:latest#$(DOCKERFILE_BASE)#g" ./$(BINARY)/Dockerfile >$(BUILD_DIR)/Dockerfile
ifneq ($(GOARCH),amd64)
	perl -pi -e "s/^RUN.*//g" $(BUILD_DIR)/Dockerfile
endif

.PHONY: binary
binary: build-dir
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) GOARM=6 \
		$(GOCMD) build -o $(BUILD_DIR)/$(BINARY) go.universe.tf/metallb/$(BINARY)

.PHONY: build-dir
build-dir:
	mkdir -p $(BUILD_DIR)
