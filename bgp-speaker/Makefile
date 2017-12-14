include ../Makefile.inc

NAME:=bgp-speaker
BINARY:=$(NAME)

.PHONY: bgp-speaker
bgp-speaker:
	CGO_ENABLED=0 $(SYSTEM) go build -o $(BINARY)
