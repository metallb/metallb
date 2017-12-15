DOCKER:=miek

SYSTEM:=
NAME:=
BINARY:=
DOCKER_IMAGE_NAME:=

VERSION:=$(shell git tag | head -1 | tr -d 'v')
EMPTY :=
SPACE := $(EMPTY) $(EMPTY)
COMMA := $(EMPTY),$(EMPTY)

LINUX_ARCH:=amd64 arm arm64 ppc64le s390x
PLATFORMS:=$(subst $(SPACE),$(COMMA),$(foreach arch,$(LINUX_ARCH),linux/$(arch)))

ifeq (, $(shell which manifest-tool))
    $(error "No manifest-tool in $$PATH, install with: go get github.com/estesp/manifest-tool")
endif

.PHONY: build
build:
	for arch in $(LINUX_ARCH); do \
	    mkdir -p build/linux/$$arch && $(MAKE) $(NAME) BINARY=build/linux/$$arch/$(NAME) SYSTEM="GOOS=linux GOARCH=$$arch";\
	done

.PHONY: docker-build
docker-build:
	# 1. Assume binary is build in build/linux/<arch>
	# 2. Copy Dockerfile to build/linux/<arch>
	# 3. Replace base image from alpine:latest to <arch>/alpine:latest
	#
	# <arch>:
	# arm: arm32v6
	# arm64: arm64v8
	for arch in $(LINUX_ARCH); do \
	    cp Dockerfile build/linux/$$arch ;\
	    if [ $$arch != amd64 ]; then \
		case $$arch in \
	        arm) \
	            BASEIMAGE=arm32v6 ;\
	            ;; \
	        arm64) \
	            BASEIMAGE=arm64v8 ;\
	            ;; \
	        *) \
	            BASEIMAGE=$$arch ;\
	            ;; \
	        esac ;\
	        sed -e "s/alpine:latest/$$BASEIMAGE\\/alpine:latest/" -i build/linux/$$arch/Dockerfile ;\
	    fi ;\
	    docker build -t $(NAME) build/linux/$$arch ;\
	    docker tag $(NAME) $(DOCKER_IMAGE_NAME):$(NAME)-$$arch ;\
	done

.PHONY: docker-upload
docker-upload:
	@echo Pushing: $(VERSION)
	for arch in $(LINUX_ARCH); do \
	    docker push $(DOCKER_IMAGE_NAME):$(NAME)-$$arch ;\
	done
	manifest-tool push from-args --platforms $(PLATFORMS) --template $(DOCKER_IMAGE_NAME):$(NAME)-ARCH --target $(DOCKER_IMAGE_NAME):$(VERSION)
	manifest-tool push from-args --platforms $(PLATFORMS) --template $(DOCKER_IMAGE_NAME):$(NAME)-ARCH --target $(DOCKER_IMAGE_NAME):latest
