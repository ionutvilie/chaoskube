BINARY = chaoskube
GOARCH = amd64

VERSION?=?
COMMIT=$(shell git rev-parse HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
GITURL=$(shell git config --get remote.origin.url | sed "s/git@//g;s/\.git//g;s/:/\//g")

CURRENT_DIR=$(shell pwd)
BUILD_DIR_LINK=$(shell readlink ${BUILD_DIR})

DOCKER_IMAGE_NAME       ?= ${BINARY}
DOCKER_IMAGE_TAG        ?= latest

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS = -ldflags "-w -s -X main.VERSION=${VERSION} -X main.COMMIT=${COMMIT} -X main.BRANCH=${BRANCH}"

# Build the project
all: linux docker

SHELL := /bin/bash

clean: 
	go clean -n
	rm -f ${CURRENT_DIR}/${BINARY}-windows-${GOARCH}.exe
	rm -f ${CURRENT_DIR}/${BINARY}-linux-${GOARCH}
	rm -f ${CURRENT_DIR}/${BINARY}-mac-${GOARCH}

dep:
	dep ensure -vendor-only

linux:
	@echo ">> building linux binary"
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BINARY}-linux-${GOARCH} . ;

windows:
	@echo ">> building windows binary"
	GOOS=windows GOARCH=amd64 go build -o ${BINARY}-windows-${GOARCH}.exe . ;

mac:
	@echo ">> building mac os binary"
	GOOS="darwin" GOARCH=amd64 go build -o ${BINARY}-mac-${GOARCH}.exe . ;

docker: 
	@echo ">> building docker image"
	docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .
	@echo ">> docker run -d -p 8080:8080 $(DOCKER_IMAGE_NAME)" ;

build_tag_push:
	@[ "${DOCKER_REGISTRY_URL}" ] || ( echo ">> DOCKER_REGISTRY_URL is not set"; exit 1 );
	@echo "Prepare linux build...";
	make linux;
	mv ${BINARY}-linux-${GOARCH} build/${BINARY}-linux-${GOARCH};
	@echo "Build docker image";
	docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" -f build/chaosDockerFile ./build;
	@echo "Prepare image to be pushed";
	docker tag $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) $(DOCKER_REGISTRY_URL)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG);
	@echo "Push image to registry";
	docker push $(DOCKER_REGISTRY_URL)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG);

# works on mac and should work on linux
simulate-pipeline-build:
	@echo ">> simulate pipeline build"
	@echo ">> $(GITURL)"
	rm -rf /tmp/workspace
	@echo ">> be careful git regex is more permisive than rsync"
	rsync -rupE --filter=':- .gitignore' $(CURRENT_DIR)/ /tmp/workspace
	# rsync -rupE --exclude={vendor,web-ui/node_modules} $(CURRENT_DIR)/ /tmp/workspace
	docker run --rm -v /tmp/workspace:/mnt/workspace -w /mnt/workspace golang:1.9 ./pipeline-build.sh ;

release: linux docker

.PHONY: release all linux windows docker