# Build log-collector for Linux/macOS, arm64/amd64 into dist/

BINARY_NAME := log-collector
CMD_PATH    := ./cmd/log-collector
DIST        := dist

GOOS_ARCHES := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all clean test build-%
all: clean $(addprefix build-,$(subst /,-,$(GOOS_ARCHES)))

# build-<goos>-<goarch>, e.g. build-linux-amd64
build-%:
	$(eval GOOS   := $(word 1,$(subst -, ,$*)))
	$(eval GOARCH := $(word 2,$(subst -, ,$*)))
	@mkdir -p $(DIST)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(DIST)/$(BINARY_NAME)-$(GOOS)-$(GOARCH) $(CMD_PATH)

test:
	go test ./...

clean:
	rm -rf $(DIST)
