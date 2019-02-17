# -*- mode: makefile-gmake -*-

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
	perl -pi -e "s#/google/metallb/blob/master#/google/metallb/blob/v$(VERSION)#g" website/content/*.md website/content/*/*.md

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
