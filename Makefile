# Build log-collector for Linux/macOS, arm64/amd64 into dist/

BINARY_NAME   := log-collector
CMD_PATH      := ./cmd/log-collector
DIST          := dist
INSTALL_BIN   := /usr/local/bin
CONFIG_DIR    := /etc/log-collector
SERVICE_DIR   := /etc/systemd/system
SERVICE_NAME  := log-collector

GOOS_ARCHES := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all clean test install build-%
all: clean $(addprefix build-,$(subst /,-,$(GOOS_ARCHES)))

# build-<goos>-<goarch>, e.g. build-linux-amd64
build-%:
	$(eval GOOS   := $(word 1,$(subst -, ,$*)))
	$(eval GOARCH := $(word 2,$(subst -, ,$*)))
	@mkdir -p $(DIST)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(DIST)/$(BINARY_NAME)-$(GOOS)-$(GOARCH) $(CMD_PATH)

test:
	go test ./...

# install: 仅支持 Ubuntu Linux，将二进制与 systemd 服务安装到系统目录（需 sudo）
install:
	@./scripts/install.sh

clean:
	rm -rf $(DIST)
