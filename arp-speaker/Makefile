SYSTEM:=
DOCKER:=miek
NAME:=arp-speaker
DOCKER_IMAGE_NAME:=$(DOCKER)/$(NAME)
LINUX_ARCH:=amd64 arm arm64 ppc64le s390x

ifeq (, $(shell which manifest-tool))
    $(error "No manifest-tool in $$PATH, install with: go get github.com/estesp/manifest-tool")
endif

.PHONY: build
build:
	for arch in $(LINUX_ARCH); do \
	    mkdir -p build/linux/$$arch && $(MAKE) arp-speaker BINARY=build/linux/$$arch/$(NAME) SYSTEM="GOOS=linux GOARCH=$$arch";\
	done

arp-speaker: main.go
	CGO_ENABLED=0 $(SYSTEM) go build -o arp-speaker

.PHONY: docker-build
docker-build: build
	# 1. Assume binary is build in build/docker/linux/<arch>s
	# 2. Copy Dockerfile to build/docker/linux/<arch>
	# 3. Replace base image from alpine:latest to <arch>/alpine:latest
	# <arch>:
	# arm: arm32v6
	# arm64: arm64v8
	for arch in $(LINUX_ARCH); do \
	    cp Dockerfile build/docker/linux/$$arch ;\
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
	        sed -e "s/alpine:latest/$$BASEIMAGE\\/alpine:latest/" -i build/docker/linux/$$arch/Dockerfile ;\
	    fi ;\
	    docker build -t coredns build/docker/linux/$$arch ;\
	    docker tag coredns $(DOCKER_IMAGE_NAME):coredns-$$arch ;\
	done

.PHONY: docker-upload
docker-upload:
	@echo Pushing: $(VERSION)
	for arch in $(LINUX_ARCH); do \
	    docker push $(DOCKER_IMAGE_NAME):coredns-$$arch ;\
	done
	manifest-tool push from-args --platforms $(PLATFORMS) --template $(DOCKER_IMAGE_NAME):coredns-ARCH --target $(DOCKER_IMAGE_NAME):$(VERSION)
	manifest-tool push from-args --platforms $(PLATFORMS) --template $(DOCKER_IMAGE_NAME):coredns-ARCH --target $(DOCKER_IMAGE_NAME):latest
