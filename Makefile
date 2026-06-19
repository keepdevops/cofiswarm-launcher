ROLE := launcher
.PHONY: build test test-standalone-layout
build:
	go build -o bin/cofiswarm-launcher ./cmd/cofiswarm-launcher
	go build -o bin/cofiswarm-configure ./cmd/cofiswarm-configure
test: build test-standalone-layout test-configure-gate
test-standalone-layout:
	./test/scripts/assert-layout.sh $(ROLE)
test-configure-gate:
	./test/scripts/test-configure-gate.sh
