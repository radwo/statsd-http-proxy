GOPATH=$(CURDIR)

# build tools
IS_GCCGO_INSTALLED=$(gccgo --version 2> /dev/null)

# build version
VERSION=`git describe --tags | awk -F'-' '{print $$1}'`
BUILD_NUMBER=`git rev-parse HEAD`
BUILD_DATE=`date +%Y-%m-%d-%H:%M`

# go compiler flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildNumber=$(BUILD_NUMBER) -X main.BuildDate=$(BUILD_DATE)"
LDFLAGS_COMPRESSED=-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildNumber=$(BUILD_NUMBER) -X main.BuildDate=$(BUILD_DATE)"

#gccgo compiler flags
GCCGOFLAGS=-gccgoflags "-march=native -O3"
GCCGOFLAGS_GOLD=-gccgoflags "-march=native -O3 -fuse-ld=gold"

# default task
default: build

# install dependencies
deps:
	GOPATH=$(GOPATH) go get -d ./...

deps-gccgo:
ifndef IS_GCCGO_INSTALLED
	$(error "gccgo not installed")
endif

# build with go compiler
build: deps
	GOPATH=$(GOPATH) CGO_ENABLED=0 go build -v -x -a $(LDFLAGS) -o $(CURDIR)/bin/statsd-http-proxy


# build with go compiler and link optiomizations
build-shrink: deps
	GOPATH=$(GOPATH) CGO_ENABLED=0 go build -v -x -a $(LDFLAGS_COMPRESSED) -o $(CURDIR)/bin/statsd-http-proxy-shrink

# build with gccgo compiler
# Require to install gccgo
build-gccgo: deps deps-gccgo
	GOPATH=$(GOPATH) CGO_ENABLED=0 go build -v -x -a -compiler gccgo $(GCCGOFLAGS) -o $(CURDIR)/bin/statsd-http-proxy-gccgo

# build with gccgo compiler and gold linker
# Require to install gccgo
build-gccgo-gold: deps deps-gccgo
	GOPATH=$(GOPATH) CGO_ENABLED=0 go build -v -x -a -compiler gccgo $(GCCGOFLAGS_GOLD) -o $(CURDIR)/bin/statsd-http-proxy-gccgo-gold

# build all
build-all: build build-shrink build-gccgo build-gccgo-gold

# clean build
clean:
	rm -rf ./bin
	go clean

# to publish to docker registry we need to be logged in
docker-login:
ifdef DOCKER_REGISTRY_USERNAME
	@echo "h" $(DOCKER_REGISTRY_USERNAME) "h"
else
	docker login
endif

# build docker images
docker-build:
	docker build --tag gometric/statsd-http-proxy:latest -f ./Dockerfile.alpine .
	docker build --tag gometric/statsd-http-proxy:$(VERSION) -f ./Dockerfile.alpine .

# publish docker images to hub
docker-publish: docker-build
	docker login
	docker push gometric/statsd-http-proxy:latest
	docker push gometric/statsd-http-proxy:$(VERSION)
