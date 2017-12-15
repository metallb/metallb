include ../Makefile.inc

NAME:=arp-speaker
BINARY:=$(NAME)
DOCKER_IMAGE_NAME:=$(DOCKER)/$(NAME)

.PHONY: arp-speaker
arp-speaker:
	CGO_ENABLED=0 $(SYSTEM) go build -o $(BINARY)
