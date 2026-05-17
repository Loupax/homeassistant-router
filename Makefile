BINARY     := homeassistant
INSTALL_DIR := $(HOME)/.local/bin
SERVICE_DIR := $(HOME)/.config/systemd/user

.PHONY: build install test clean \
        setup-listener \
        install-services enable disable start stop restart \
        logs logs-listener \
        update-hotwords install-hotwords-timer logs-hotwords

build:
	go build -o $(BINARY) .

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)

setup-listener:
	./setup-listener.sh

install-services:
	mkdir -p $(SERVICE_DIR)
	cp homeassistant-router.service homeassistant-listener.service $(SERVICE_DIR)/
	systemctl --user daemon-reload

enable: install-services
	systemctl --user enable --now homeassistant-router homeassistant-listener

disable:
	systemctl --user disable --now homeassistant-router homeassistant-listener

start:
	systemctl --user start homeassistant-router homeassistant-listener

stop:
	systemctl --user stop homeassistant-router homeassistant-listener

restart:
	systemctl --user restart homeassistant-router homeassistant-listener

logs:
	journalctl --user -f -u homeassistant-router

logs-listener:
	journalctl --user -f -u homeassistant-listener

update-hotwords:
	python3 update_hotwords.py

install-hotwords-timer:
	mkdir -p $(SERVICE_DIR)
	cp homeassistant-hotwords.service homeassistant-hotwords.timer $(SERVICE_DIR)/
	systemctl --user daemon-reload
	systemctl --user enable --now homeassistant-hotwords.timer

logs-hotwords:
	journalctl --user -f -u homeassistant-hotwords
