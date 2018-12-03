BUILD_OS_TARGETS = "linux darwin freebsd windows"

test: deps
	go test ./...

deps:
	go get -d -v -t ./...
	go get github.com/golang/lint/golint
	go get golang.org/x/tools/cmd/vet
	go get golang.org/x/tools/cmd/cover
	go get github.com/mattn/goveralls

LINT_RET = .golint.txt
lint: deps
	go vet ./...
	rm -f $(LINT_RET)
	golint ./... | tee $(LINT_RET)
	test ! -s $(LINT_RET)

cover: deps
	go get github.com/axw/gocov/gocov
	goveralls

.PHONY: test deps lint cover
