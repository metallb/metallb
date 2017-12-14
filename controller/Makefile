include ../Makefile.inc

NAME:=controller
BINARY:=$(NAME)

# PHONY because we need to rebuild for every arch.
.PHONY: controller
controller:
	CGO_ENABLED=0 $(SYSTEM) go build -o $(BINARY)
