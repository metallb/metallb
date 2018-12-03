# generate go structures from proto files
define generate_sources
	$(if $(shell command -v protoc --gogo_out=. 2> /dev/null),$(info gogo/protobuf is installed),$(error gogo/protobuf missing, please install it with go get github.com/gogo/protobuf))
	@echo "# generating sources"
	@cd airport && go generate -v
endef

all: generate build

generate:
	$(call generate_sources)
	@echo "# done"

clean:
	rm -f simple/simple
	rm -f simple/simple-linux
	rm -f airport/airport
	rm -f airport/airport-linux

build:
	cd simple && go build -i
	cd airport && go build -i

linux: ## build for linux
	@echo "building template (linux)"
	cd simple && env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -i -o simple-linux
	cd airport && env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -i -o airport-linux
