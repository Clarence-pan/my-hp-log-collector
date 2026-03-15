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
	@if [ "$$(uname -s)" != "Linux" ]; then \
		echo "ERROR: make install only supports Ubuntu Linux. Current OS: $$(uname -s)"; exit 1; \
	fi
	@ARCH=$$(uname -m); \
	if [ "$$ARCH" = "x86_64" ]; then ARCH=amd64; \
	elif [ "$$ARCH" = "aarch64" ]; then ARCH=arm64; \
	else echo "ERROR: Unsupported architecture $$ARCH"; exit 1; fi; \
	BIN=$(DIST)/$(BINARY_NAME)-linux-$$ARCH; \
	if [ ! -f "$$BIN" ]; then echo "Building $$BIN..."; $(MAKE) build-linux-$$ARCH; fi; \
	echo "Installing binary to $(INSTALL_BIN)/$(BINARY_NAME)"; \
	sudo install -m 755 "$$BIN" "$(INSTALL_BIN)/$(BINARY_NAME)"; \
	echo "Creating config directory $(CONFIG_DIR)"; \
	sudo mkdir -p "$(CONFIG_DIR)"; \
	echo "Installing config template to $(CONFIG_DIR)/log-collector.yaml"; \
	sudo install -m 644 log-collector.example.yaml "$(CONFIG_DIR)/log-collector.yaml"; \
	if [ -f .env.example ]; then \
		if [ ! -f "$(CONFIG_DIR)/.env" ]; then \
			echo "Installing env template to $(CONFIG_DIR)/.env"; \
			sudo install -m 600 .env.example "$(CONFIG_DIR)/.env"; \
		else echo "Keeping existing $(CONFIG_DIR)/.env"; fi; \
	else echo "No .env.example found, skip .env"; fi; \
	echo "Installing systemd unit to $(SERVICE_DIR)/$(SERVICE_NAME).service"; \
	sudo install -m 644 packaging/log-collector.service "$(SERVICE_DIR)/$(SERVICE_NAME).service"; \
	sudo systemctl daemon-reload; \
	sudo systemctl enable $(SERVICE_NAME); \
	echo ""; \
	echo "Install done. Config file locations (edit these before starting the service):"; \
	echo "  - Config YAML:  $(CONFIG_DIR)/log-collector.yaml"; \
	echo "  - Env vars:     $(CONFIG_DIR)/.env"; \
	echo ""; \
	echo "Then start the service:"; \
	echo "  sudo systemctl start $(SERVICE_NAME)"; \
	echo "  sudo systemctl status $(SERVICE_NAME)"; \
	echo "  journalctl -u $(SERVICE_NAME) -f"

clean:
	rm -rf $(DIST)
