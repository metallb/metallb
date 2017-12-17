# -*- mode: makefile-gmake -*-

## Customizable options. If you want to change these for your local
## checkout, write the custom values to Makefile.defaults.

# The architecture to use for `make push`.
ARCH:=amd64
# The registry to push images to. The default works for Minikube.
REGISTRY:=localhost:5000
ifeq ($(shell uname -s),Darwin)
	REGISTRY:=docker.for.mac.localhost:5000
endif
# The tag to use when building images. The default is a running
# timestamp, so that every build is a different image.
TAG:=$(shell date +"%s.%N")
# The command to use to build Go binaries.
GOCMD:=go
# If non-empty, invoke all docker commands with `sudo`.
DOCKER_SUDO:=

## End of customizable options.

# Local customizations to the above.
ifneq ($(wildcard Makefile.defaults),)
include Makefile.defaults
endif

# Magical rubbish to teach make what commas and spaces are.
EMPTY :=
SPACE := $(EMPTY) $(EMPTY)
COMMA := $(EMPTY),$(EMPTY)

ALL_ARCH:=amd64 arm arm64 ppc64le s390x
BINARIES:=controller speaker test-bgp-router
PLATFORMS:=$(subst $(SPACE),$(COMMA),$(foreach arch,$(ALL_ARCH),linux/$(arch)))
MK_IMAGE_TARGETS:=Makefile.image-targets
IN_CLUSTER_REGISTRY:=$(REGISTRY)
ifeq ($(findstring localhost,$(REGISTRY)),localhost)
	IN_CLUSTER_REGISTRY:=$(shell kubectl get svc -n kube-system registry -o go-template='{{.spec.clusterIP}}'):80
endif

all:
	$(error Please request a specific thing, there is no default target)

################################
## Iteration during development
##
## Leave `make proxy-to-registry` running in a terminal if you're
## using minikube.
##
## `make push` builds timestamped images, pushes them to REGISTRY, and
## updates your currently active cluster to pull them.

.PHONY: start-minikube
start-minikube:
	minikube start
	minikube addons enable registry
	kubectl apply -f manifests/metallb.yaml
	kubectl apply -f manifests/test-bgp-router.yaml
	kubectl apply -f manifests/tutorial-1.yaml

.PHONY: proxy-to-registry
proxy-to-registry:
	( \
		pod=$(shell kubectl get pod -n kube-system -l kubernetes.io/minikube-addons=registry -o go-template='{{(index .items 0).metadata.name}}') &&\
		kubectl port-forward -n kube-system $$pod 5000:5000 ;\
	)

.PHONY: push-manifests
push-manifests:
	kubectl apply -f manifests/metallb.yaml,manifests/test-bgp-router.yaml,manifests/tutorial-1.yaml

.PHONY: push
push: gen-image-targets
	+make -f $(MK_IMAGE_TARGETS) $(foreach bin,$(BINARIES),$(bin)/$(ARCH))
	kubectl set image -n metallb-system deploy/controller controller=$(IN_CLUSTER_REGISTRY)/controller:$(TAG)-$(ARCH)
	kubectl set image -n metallb-system ds/speaker speaker=$(IN_CLUSTER_REGISTRY)/speaker:$(TAG)-$(ARCH)
	kubectl set image -n metallb-system deploy/test-bgp-router test-bgp-router=$(IN_CLUSTER_REGISTRY)/test-bgp-router:$(TAG)-$(ARCH)

################################
## Building full images
##
## `make all-arch-images` builds and pushes images for all
## architectures, tagged as TAG-ARCH, then creates a multi-arch
## manifest under TAG that links to all of them.

.PHONY: all-arch-images
all-arch-images: gen-image-targets
	+make -f $(MK_IMAGE_TARGETS) all

.PHONY: gen-image-targets
gen-image-targets:
	echo "" >$(MK_IMAGE_TARGETS)
	for binary in $(BINARIES); do \
		for arch in $(ALL_ARCH); do \
			/bin/echo ".PHONY: $$binary/$$arch" >>$(MK_IMAGE_TARGETS) ;\
			/bin/echo "$$binary/$$arch:" >>$(MK_IMAGE_TARGETS) ;\
			/bin/echo -e "\t+make -f Makefile.inc push BINARY=$$binary GOARCH=$$arch TAG=$(TAG)-$$arch GOCMD=$(GOCMD) DOCKER_SUDO=$(DOCKER_SUDO) REGISTRY=$(REGISTRY)" >>$(MK_IMAGE_TARGETS) ;\
			/bin/echo "" >>$(MK_IMAGE_TARGETS) ;\
		done ;\
		/bin/echo ".PHONY: $$binary" >>$(MK_IMAGE_TARGETS) ;\
		/bin/echo -n "$$binary: " >>$(MK_IMAGE_TARGETS) ;\
		for arch in $(ALL_ARCH); do \
			/bin/echo -n "$$binary/$$arch " >>$(MK_IMAGE_TARGETS) ;\
		done ;\
		/bin/echo "" >>$(MK_IMAGE_TARGETS) ;\
		/bin/echo -e "\tmanifest-tool push from-args --platforms $(PLATFORMS) --template $(REGISTRY)/$${binary}:$(TAG)-ARCH --target $(REGISTRY)/$${binary}:$(TAG)\n" >>$(MK_IMAGE_TARGETS) ;\
	done
	/bin/echo -n "all: " >>$(MK_IMAGE_TARGETS)
	for binary in $(BINARIES); do \
		/bin/echo -ne "$$binary " >>$(MK_IMAGE_TARGETS) ;\
	done
	/bin/echo "" >>$(MK_IMAGE_TARGETS)

################################
## For CircleCI
##
## CircleCI doesn't yet support parameterized jobs on their 2.0
## platform, so we need to fully replicate the build instructions for
## each version of Go we want to test on.
##
## To make the repetition less verbose, we bundle all the stages of
## execution into this one make command, so that the job specs on
## circleci are all simple.

.PHONY: ci-prepare
ci-prepare:
	go get github.com/Masterminds/glide
	go get github.com/golang/lint/golint
	glide install

.PHONY: ci-build
ci-build:
	go install .v ./controller ./speaker ./test-bgp-router

.PHONY: ci-test
ci-test:
	go test $(glide novendor)
	go test -race $(glide novendor)

.PHONY: ci-lint
ci-lint:
	go vet $(glide novendor)
	glide novendor | xargs -n1 golint

################################
## Release
##
## `make release VERSION=1.2.3` creates/updates the release branch,
## and tags the new release.

VERSION:=
ifneq ($(VERSION),)
	MAJOR=$(shell echo $(VERSION) | cut -f1 -d'.')
	MINOR=$(shell echo $(VERSION) | cut -f2 -d'.')
	PATCH=$(shell echo $(VERSION) | cut -f3 -d'.')
endif

release:
ifeq ($(VERSION),)
	$(error VERSION is required)
endif
ifneq ($(shell git status --porcelain),)
	$(error git working directory is not clean, cannot prepare release)
endif
	git checkout master
ifeq ($(shell grep "\#\# Version $(VERSION)" website/content/release-notes.md),)
	$(error no release notes for $(VERSION))
endif
ifeq ($(PATCH),0)
	git ckeckout -b v$(MAJOR).$(MINOR)
	perl -pi -e 's#/google/metallb/master#/google/metallb/v$(VERSION)#g' website/content/*.md
	perl -pi -e 's/:latest/:v$(VERSION)/g' manifests/*.yaml
else
	git checkout v$(MAJOR).$(MINOR)
	perl -pi -e "s#/google/metallb/v$(MAJOR).$(MINOR).$$(($(PATCH)-1))#/google/metallb/v$(VERSION)#g" website/content/*.md
	perl -pi -e "s#:v$(MAJOR).$(MINOR).$$(($(PATCH)-1))#:v$(VERSION)#g" manifests/*.yaml
endif
	git checkout master -- website/content/release-notes.md
	perl -pi -e 's/version = .*/version = "v$(VERSION)"/g' website/config.toml
	git commit -a -m "Update documentation for release $(VERSION)"
	git tag v$(VERSION) -m "See the release notes for details:\n\nhttps://metallb.universe.tf/release-notes/#version-$(MAJOR)-$(MINOR)-$(PATCH)"
	git checkout master
