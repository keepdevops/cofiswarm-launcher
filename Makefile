ROLE := launcher
.PHONY: build test test-standalone-layout
build:
	go build -o bin/cofiswarm-launcher ./cmd/cofiswarm-launcher
test: build test-standalone-layout
test-standalone-layout:
	./test/scripts/assert-layout.sh $(ROLE)
