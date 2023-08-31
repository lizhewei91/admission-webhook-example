TARGET_PLATFORMS ?= linux/amd64,linux/arm64
BASE_IMAGE ?= golang
BASE_IMAGE_VERSION ?= alpine3.17
IMAGE_REPO ?= ${DOCKER_HUB_REPO}/admission-webhook-exmaple
IMAGE_VERSION ?= v0.0.1
DOCKER_HUB_REPO ?= hub.xxx.cn
DOCKER_HUB_USERNAME ?= xxx
DOCKER_HUB_PASSWORD ?= xxx

.PHONY: all
all: docker-hub-login build images

docker-hub-login:
	docker logout
	docker login ${DOCKER_HUB_REPO} -u ${DOCKER_HUB_USERNAME} -p ${DOCKER_HUB_PASSWORD}

build:
	go build -o /go/src/buildx-demo/bin/buildx-demo main.go

images:
	docker buildx build \
		--build-arg BASE_IMAGE=$(BASE_IMAGE) \
		--build-arg BASE_IMAGE_VERSION=$(BASE_IMAGE_VERSION) \
		--platform $(TARGET_PLATFORMS) \
		-t $(IMAGE_REPO):$(IMAGE_VERSION) \
		-f ./Dockfile --push .