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

GITCOMMIT=$(shell git describe --dirty --always --match '')
GITBRANCH=$(shell git rev-parse --abbrev-ref HEAD)

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

MANIFESTFILE=manifests/metallb.yaml
.PHONY: manifest
manifest:
	cat manifests/namespace.yaml >$(MANIFESTFILE)
	cd helm-chart && helm template --namespace metallb-system --set controller.resources.limits.cpu=100m,controller.resources.limits.memory=100Mi,speaker.resources.limits.cpu=100m,speaker.resources.limits.memory=100Mi,prometheus.scrapeAnnotations=true,config.name=config . >>../$(MANIFESTFILE)
	sed -i '/heritage: /d' $(MANIFESTFILE)
	sed -i '/release: /d' $(MANIFESTFILE)
	sed -i '/chart: /d' $(MANIFESTFILE)
	sed -i '/^# /d' $(MANIFESTFILE)
	sed -i 's/RELEASE-NAME-metallb-//g' $(MANIFESTFILE)
	sed -i 's/RELEASE-NAME-metallb:/metallb-system:/g' $(MANIFESTFILE)
	perl -p0i -e 's/metadata:\n  name: (?!metallb-system)/metadata:\n  namespace: metallb-system\n  name: /gs' $(MANIFESTFILE)

.PHONY: build
build:
	$(GOCMD) install -v -ldflags="-X go.universe.tf/metallb/internal/version.gitCommit=$(GITCOMMIT) -X go.universe.tf/metallb/internal/version.gitBranch=$(GITBRANCH)" ./controller ./speaker ./test-bgp-router

.PHONY: start-minikube
start-minikube:
	minikube start --bootstrapper=kubeadm
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

.PHONY: push-images
push-images: gen-image-targets
	+make -f $(MK_IMAGE_TARGETS) $(foreach bin,$(BINARIES),$(bin)/$(ARCH))

.PHONY: push
push: push-images
ifeq ($(findstring localhost,$(REGISTRY)),localhost)
	kubectl set image -n metallb-system deploy/controller controller=$(shell kubectl get svc -n kube-system registry -o go-template='{{.spec.clusterIP}}'):80/controller:$(TAG)-$(ARCH)
	kubectl set image -n metallb-system ds/speaker speaker=$(shell kubectl get svc -n kube-system registry -o go-template='{{.spec.clusterIP}}'):80/speaker:$(TAG)-$(ARCH)
	kubectl set image -n metallb-system deploy/test-bgp-router test-bgp-router=$(shell kubectl get svc -n kube-system registry -o go-template='{{.spec.clusterIP}}'):80/test-bgp-router:$(TAG)-$(ARCH)
else
	kubectl set image -n metallb-system deploy/controller controller=$(REGISTRY)/controller:$(TAG)-$(ARCH)
	kubectl set image -n metallb-system ds/speaker speaker=$(REGISTRY)/speaker:$(TAG)-$(ARCH)
	kubectl set image -n metallb-system deploy/test-bgp-router test-bgp-router=$(REGISTRY)/test-bgp-router:$(TAG)-$(ARCH)
endif

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
	printf "\n" >$(MK_IMAGE_TARGETS)
	for binary in $(BINARIES); do \
		for arch in $(ALL_ARCH); do \
			printf ".PHONY: $$binary/$$arch\n" >>$(MK_IMAGE_TARGETS) ;\
			printf "$$binary/$$arch:\n" >>$(MK_IMAGE_TARGETS) ;\
			printf "\t+make -f Makefile.inc push BINARY=$$binary GOARCH=$$arch TAG=$(TAG)-$$arch GOCMD=$(GOCMD) DOCKER_SUDO=$(DOCKER_SUDO) REGISTRY=$(REGISTRY) GITCOMMIT=$(GITCOMMIT) GITBRANCH=$(GITBRANCH)\n" >>$(MK_IMAGE_TARGETS) ;\
			printf "\n" >>$(MK_IMAGE_TARGETS) ;\
		done ;\
		printf ".PHONY: $$binary\n" >>$(MK_IMAGE_TARGETS) ;\
		printf "$$binary: " >>$(MK_IMAGE_TARGETS) ;\
		for arch in $(ALL_ARCH); do \
			printf  "$$binary/$$arch " >>$(MK_IMAGE_TARGETS) ;\
		done ;\
		printf "\n" >>$(MK_IMAGE_TARGETS) ;\
		printf "\tmanifest-tool push from-args --platforms $(PLATFORMS) --template $(REGISTRY)/$${binary}:$(TAG)-ARCH --target $(REGISTRY)/$${binary}:$(TAG)" >>$(MK_IMAGE_TARGETS) ;\
		if [ "$(TAG)" = "master" ]; then \
			printf "\tmanifest-tool push from-args --platforms $(PLATFORMS) --template $(REGISTRY)/$${binary}:$(TAG)-ARCH --target $(REGISTRY)/$${binary}:latest" >>$(MK_IMAGE_TARGETS) ;\
		fi ;\
		printf "\n" >>$(MK_IMAGE_TARGETS) ;\
	done
	printf "all: " >>$(MK_IMAGE_TARGETS)
	for binary in $(BINARIES); do \
		printf "$$binary " >>$(MK_IMAGE_TARGETS) ;\
	done
	printf "\n" >>$(MK_IMAGE_TARGETS)

################################
## e2e tests
##

GCP_PROJECT:=metallb-e2e-testing
CLUSTER_PREFIX:=test
PROTOCOL:=ipv4
NETWORK_ADDON:=flannel
CLUSTER_NAME:=$(CLUSTER_PREFIX)-$(PROTOCOL)-$(NETWORK_ADDON)

.PHONY: e2e-up-cluster
e2e-up-cluster:
	(cd e2etest/terraform && terraform apply -state=$(CLUSTER_NAME).tfstate -backup=- -auto-approve -no-color -var=cluster_name=$(CLUSTER_NAME) -var=protocol=$(PROTOCOL) -var=network_addon=$(NETWORK_ADDON) -var=gcp_project=$(GCP_PROJECT))

.PHONY: e2e-down-cluster
e2e-down-cluster:
	(cd e2etest/terraform && terraform destroy -state=$(CLUSTER_NAME).tfstate -backup=- -force -no-color -var=gcp_project=$(GCP_PROJECT))

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

.PHONY: ci-config
ci-config:
	(cd .circleci && go run gen-config.go >config.yml)

.PHONY: ci-prepare
ci-prepare:
	curl https://raw.githubusercontent.com/kubernetes/helm/master/scripts/get | bash
	$(GOCMD) get github.com/Masterminds/glide
	$(GOCMD) get github.com/golang/lint/golint
	$(GOCMD) get github.com/estesp/manifest-tool

.PHONY: ci
ci: ci-prepare build test lint

.PHONY: test
test:
	$(GOCMD) test $$(glide novendor)
	$(GOCMD) test -race $$(glide novendor)

.PHONY: lint
lint:
	$(GOCMD) get -u github.com/alecthomas/gometalinter
	gometalinter --install golint
	gometalinter --deadline=1m --disable-all --enable=gofmt --enable=golint --enable=vet --enable=vetshadow --enable=structcheck --enable=unconvert --vendor ./...
	+make manifest MANIFESTFILE=manifests/metallb.yaml.new
	diff -u manifests/metallb.yaml manifests/metallb.yaml.new
	rm manifests/metallb.yaml.new

################################
## Release
##
## `make release VERSION=1.2.3` creates/updates the release branch,
## and tags the new release.

VERSION:=
SKIPRELNOTES:=
ifneq ($(VERSION),)
	MAJOR=$(shell echo $(VERSION) | cut -f1 -d'.')
	MINOR=$(shell echo $(VERSION) | cut -f2 -d'.')
	PATCH=$(shell echo $(VERSION) | cut -f3 -d'.')
endif
ifeq ($(PATCH),0)
	PREV_VERSION=master
else
	PREV_VERSION=v$(MAJOR).$(MINOR).$(shell expr $(PATCH) - 1)
endif

.PHONY: release
release:
ifeq ($(VERSION),)
	$(error VERSION is required)
endif
ifneq ($(shell git status --porcelain),)
	$(error git working directory is not clean, cannot prepare release)
endif
	git checkout master
ifeq ($(shell grep "\#\# Version $(VERSION)" website/content/release-notes/_index.md),)
ifeq ($(SKIPRELNOTES),)
	$(error no release notes for $(VERSION))
endif
endif
ifeq ($(PATCH),0)
	git checkout -b v$(MAJOR).$(MINOR)
else
	git checkout v$(MAJOR).$(MINOR)
endif
	git checkout master -- website/content/release-notes/_index.md

	perl -pi -e "s#/google/metallb/$(PREV_VERSION)#/google/metallb/v$(VERSION)#g" website/content/*.md website/content/*/*.md
	perl -pi -e "s#/google/metallb/tree/$(PREV_VERSION)#/google/metallb/tree/v$(VERSION)#g" website/content/*.md website/content/*/*.md
	perl -pi -e "s#/google/metallb/blob/$(PREV_VERSION)#/google/metallb/tree/v$(VERSION)#g" website/content/*.md website/content/*/*.md
	perl -pi -e "s#/google/metallb/master#/google/metallb/v$(VERSION)#g" website/content/*.md website/content/*/*.md
	perl -pi -e "s#/google/metallb/tree/master#/google/metallb/tree/v$(VERSION)#g" website/content/*.md website/content/*/*.md
	perl -pi -e "s#/google/metallb/blob/master#/google/metallb/tree/v$(VERSION)#g" website/content/*.md website/content/*/*.md

	perl -pi -e 's#image: metallb/(.*):.*#image: metallb/$$1:v$(VERSION)#g' manifests/*.yaml

	perl -pi -e 's/appVersion: .*/appVersion: $(VERSION)/g' helm-chart/Chart.yaml
	perl -pi -e 's/tag: .*/tag: v$(VERSION)/g' helm-chart/values.yaml
	perl -pi -e 's/pullPolicy: .*/pullPolicy: IfNotPresent/g' helm-chart/values.yaml

	+make manifest
	perl -pi -e 's/MetalLB .*/MetalLB v$(VERSION)/g' website/content/_header.md
	perl -pi -e 's/version\s+=.*/version = "$(VERSION)"/g' internal/version/version.go
	gofmt -w internal/version/version.go
	git commit -a -m "Update documentation for release $(VERSION)"
	git tag v$(VERSION) -m "See the release notes for details:\n\nhttps://metallb.universe.tf/release-notes/#version-$(MAJOR)-$(MINOR)-$(PATCH)"
	git checkout master
