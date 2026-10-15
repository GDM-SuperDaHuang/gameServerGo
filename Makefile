SHELL := /bin/sh

.PHONY: help check-version check-gate-version check-room-version check-serverlist-version check-home-version build-gate build-room build-serverlist build-home image-gate image-room image-serverlist image-home push-gate push-room push-serverlist push-home gate room serverlist home

TCR_REGISTRY ?= ccr.ccs.tencentyun.com
TCR_NAMESPACE ?= jpzw
IMAGE_PLATFORM ?= linux/amd64

GATE_VERSION_FILE := app/gate/version
ROOM_VERSION_FILE := app/room/version
SERVERLIST_VERSION_FILE := app/serverlist/version
HOME_VERSION_FILE := app/home/version
BACKGROUND_VERSION_FILE := app/background/version


GATE_VERSION := $(strip $(shell test -f $(GATE_VERSION_FILE) && cat $(GATE_VERSION_FILE)))
ROOM_VERSION := $(strip $(shell test -f $(ROOM_VERSION_FILE) && cat $(ROOM_VERSION_FILE)))
SERVERLIST_VERSION := $(strip $(shell test -f $(SERVERLIST_VERSION_FILE) && cat $(SERVERLIST_VERSION_FILE)))
HOME_VERSION := $(strip $(shell test -f $(HOME_VERSION_FILE) && cat $(HOME_VERSION_FILE)))
BACKGROUND_VERSION := $(strip $(shell test -f $(BACKGROUND_VERSION_FILE) && cat $(BACKGROUND_VERSION_FILE)))

GATE_TAG := $(GATE_VERSION).0.0
ROOM_TAG := $(ROOM_VERSION).0.0
SERVERLIST_TAG := $(SERVERLIST_VERSION).0.0
HOME_TAG := $(HOME_VERSION).0.0
BACKGROUND_TAG := $(BACKGROUND_VERSION).0.0

GATE_IMAGE := $(TCR_REGISTRY)/$(TCR_NAMESPACE)/gate
ROOM_IMAGE := $(TCR_REGISTRY)/$(TCR_NAMESPACE)/room
SERVERLIST_IMAGE := $(TCR_REGISTRY)/$(TCR_NAMESPACE)/serverlist
HOME_IMAGE := $(TCR_REGISTRY)/$(TCR_NAMESPACE)/home
BACKGROUND_IMAGE := $(TCR_REGISTRY)/$(TCR_NAMESPACE)/background

help:
	@echo "用法:"
	@echo "  make check-version          # 校验 gate + room 版本文件"
	@echo "  make check-serverlist-version / make check-home-version"
	@echo "  make gate             # 构建 gate 镜像并推送"
	@echo "  make room             # 构建 room 镜像并推送"
	@echo "  make serverlist       # 构建 serverlist 镜像并推送"
	@echo "  make home             # 构建 home 镜像并推送"
	@echo ""
	@echo "可覆盖变量:"
	@echo "  TCR_REGISTRY=ccr.ccs.tencentyun.com"
	@echo "  TCR_NAMESPACE=你的命名空间"
	@echo "  IMAGE_PLATFORM=linux/amd64"
	@echo ""
	@echo "示例:"
	@echo "  make TCR_NAMESPACE=myteam"

check-gate-version:
	@test -n "$(GATE_VERSION)" || (echo "错误: $(GATE_VERSION_FILE) 不存在或为空"; exit 1)
	@echo "$(GATE_VERSION)" | grep -Eq '^[0-9]+(\.[0-9]+)*$$' || (echo "错误: $(GATE_VERSION_FILE) 格式非法，应为数字或点分版本，例如 1 或 1.2.3"; exit 1)

check-room-version:
	@test -n "$(ROOM_VERSION)" || (echo "错误: $(ROOM_VERSION_FILE) 不存在或为空"; exit 1)
	@echo "$(ROOM_VERSION)" | grep -Eq '^[0-9]+(\.[0-9]+)*$$' || (echo "错误: $(ROOM_VERSION_FILE) 格式非法，应为数字或点分版本，例如 1 或 1.2.3"; exit 1)

check-serverlist-version:
	@test -n "$(SERVERLIST_VERSION)" || (echo "错误: $(SERVERLIST_VERSION_FILE) 不存在或为空"; exit 1)
	@echo "$(SERVERLIST_VERSION)" | grep -Eq '^[0-9]+(\.[0-9]+)*$$' || (echo "错误: $(SERVERLIST_VERSION_FILE) 格式非法，应为数字或点分版本，例如 1 或 1.2.3"; exit 1)

check-home-version:
	@test -n "$(HOME_VERSION)" || (echo "错误: $(HOME_VERSION_FILE) 不存在或为空"; exit 1)
	@echo "$(HOME_VERSION)" | grep -Eq '^[0-9]+(\.[0-9]+)*$$' || (echo "错误: $(HOME_VERSION_FILE) 格式非法，应为数字或点分版本，例如 1 或 1.2.3"; exit 1)

check-background-version:
	@test -n "$(BACKGROUND_VERSION)" || (echo "错误: $(BACKGROUND_VERSION_FILE) 不存在或为空"; exit 1)
	@echo "$(BACKGROUND_VERSION)" | grep -Eq '^[0-9]+(\.[0-9]+)*$$' || (echo "错误: $(BACKGROUND_VERSION_FILE) 格式非法，应为数字或点分版本，例如 1 或 1.2.3"; exit 1)

check-version: check-gate-version check-room-version check-background-version

build-gate: check-gate-version
	@echo "构建 gate, version=$(GATE_VERSION)"
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(GATE_VERSION)" -o bin/gate ./app/gate

build-room: check-room-version
	@echo "构建 room, version=$(ROOM_VERSION)"
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(ROOM_VERSION)" -o bin/room ./app/room

build-serverlist: check-serverlist-version
	@echo "构建 serverlist, version=$(SERVERLIST_VERSION)"
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(SERVERLIST_VERSION)" -o bin/serverlist ./app/serverlist

build-home: check-home-version
	@echo "构建 home, version=$(HOME_VERSION)"
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(HOME_VERSION)" -o bin/home ./app/home

build-background: check-background-version
	@echo "构建 background, version=$(BACKGROUND_VERSION)"
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(BACKGROUND_VERSION)" -o bin/background ./app/background


image-gate: check-gate-version
	@echo "构建 gate 镜像: $(GATE_IMAGE):$(GATE_TAG)"
	docker build --platform $(IMAGE_PLATFORM) --build-arg VERSION=$(GATE_VERSION) -f app/gate/Dockerfile -t $(GATE_IMAGE):$(GATE_TAG) .

image-room: build-room
	@echo "构建 room 镜像: $(ROOM_IMAGE):$(ROOM_TAG)"
	docker build --platform $(IMAGE_PLATFORM) -f app/room/Dockerfile -t $(ROOM_IMAGE):$(ROOM_TAG) .

image-serverlist: build-serverlist
	@echo "构建 serverlist 镜像: $(SERVERLIST_IMAGE):$(SERVERLIST_TAG)"
	docker build --platform $(IMAGE_PLATFORM) -f app/serverlist/Dockerfile -t $(SERVERLIST_IMAGE):$(SERVERLIST_TAG) .

image-home: build-home
	@echo "构建 home 镜像: $(HOME_IMAGE):$(HOME_TAG)"
	docker build --platform $(IMAGE_PLATFORM) -f app/home/Dockerfile -t $(HOME_IMAGE):$(HOME_TAG) .

image-background: build-background
	@echo "构建 background 镜像: $(BACKGROUND_IMAGE):$(BACKGROUND_TAG)"
	docker build --platform $(IMAGE_PLATFORM) -f app/background/Dockerfile -t $(BACKGROUND_IMAGE):$(BACKGROUND_TAG) .


push-gate:
	@echo "推送 gate 镜像: $(GATE_IMAGE):$(GATE_TAG)"
	docker push $(GATE_IMAGE):$(GATE_TAG)

push-room:
	@echo "推送 room 镜像: $(ROOM_IMAGE):$(ROOM_TAG)"
	docker push $(ROOM_IMAGE):$(ROOM_TAG)

push-serverlist:
	@echo "推送 serverlist 镜像: $(SERVERLIST_IMAGE):$(SERVERLIST_TAG)"
	docker push $(SERVERLIST_IMAGE):$(SERVERLIST_TAG)

push-home:
	@echo "推送 home 镜像: $(HOME_IMAGE):$(HOME_TAG)"
	docker push $(HOME_IMAGE):$(HOME_TAG)


push-background:
	@echo "推送 background 镜像: $(BACKGROUND_IMAGE):$(BACKGROUND_TAG)"
	docker push $(BACKGROUND_IMAGE):$(BACKGROUND_TAG)


rm-bin:
	rm -rf bin > /dev/null

rm-images:
	docker rmi $(GATE_IMAGE):$(GATE_TAG) > /dev/null
	docker rmi $(ROOM_IMAGE):$(ROOM_TAG) > /dev/null
	docker rmi $(SERVERLIST_IMAGE):$(SERVERLIST_TAG) > /dev/null
	docker rmi $(HOME_IMAGE):$(HOME_TAG) > /dev/null
	docker rmi $(BACKGROUND_IMAGE):$(BACKGROUND_TAG) > /dev/null

gate: check-gate-version image-gate push-gate rm-images

room: check-room-version build-room image-room push-room rm-bin rm-images

serverlist: check-serverlist-version build-serverlist image-serverlist push-serverlist rm-bin rm-images

home: check-home-version build-home image-home push-home rm-bin rm-images

background: check-background-version build-background image-background push-background rm-bin rm-images
