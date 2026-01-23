BINARY := ralph
INSTALL_PATH := $(HOME)/.local/bin

.PHONY: build test install clean

build:
	go build -o $(BINARY) ./cmd/ralph

test:
	go test ./...

install: build
	@mkdir -p $(INSTALL_PATH)
	@rm -f $(INSTALL_PATH)/$(BINARY)
	cp $(BINARY) $(INSTALL_PATH)/$(BINARY)
	@echo "Installed $(BINARY) to $(INSTALL_PATH)"
	@$(INSTALL_PATH)/$(BINARY) --version

clean:
	rm -f $(BINARY)
