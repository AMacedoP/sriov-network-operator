CURPATH=$(PWD)
TARGET_DIR=$(CURPATH)/build/_output
KUBECONFIG?=$(HOME)/.kube/config

GOLIST=go list
GOFMT=gofmt
BUILD_GOPATH=$(TARGET_DIR):$(TARGET_DIR)/vendor:$(CURPATH)/cmd

IMAGE_BUILDER_OPTS=
IMAGE_BUILDER?=imagebuilder
IMAGE_BUILD=$(IMAGE_BUILDER)
export IMAGE_TAG_CMD?=docker tag

export APP_NAME=sriov-network-operator
APP_REPO=github.com/pliurh/$(APP_NAME)
TARGET=$(TARGET_DIR)/bin/$(APP_NAME)
export IMAGE_TAG=quay.io/pliurh/$(APP_NAME):latest
MAIN_PKG=cmd/manager/main.go
export NAMESPACE?=sriov-network-operator
PKGS=$(shell go list ./... | grep -v -E '/vendor/|/test|/examples')

# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: all operator-sdk build clean fmt gendeepcopy test-unit test-e2e run

all: build #check install

operator-sdk:
	@if ! type -p operator-sdk ; \
	then if [ ! -d $(GOPATH)/src/github.com/operator-framework/operator-sdk ] ; \
	  then git clone https://github.com/operator-framework/operator-sdk --branch master $(GOPATH)/src/github.com/operator-framework/operator-sdk ; \
	  fi ; \
	  cd $(GOPATH)/src/github.com/operator-framework/operator-sdk ; \
	  make dep ; \
	  make install ; \
	fi

all: build plugins

build: _build-manager _build-sriov-network-config-daemon

_build-%:
	WHAT=$* hack/build-go.sh

run:
	hack/run-locally.sh

clean:
	@rm -rf $(TARGET_DIR)

# image: imagebuilder
# 	@if [ $${USE_IMAGE_STREAM:-false} = false ] && [ $${SKIP_BUILD:-false} = false ] ; \
# 	then $(IMAGE_BUILDER) -t $(IMAGE_TAG) . $(IMAGE_BUILDER_OPTS) ; \
# 	fi

fmt: ; $(info  running gofmt...) @ ## Run gofmt on all source files
	@ret=0 && for d in $$($(GOLIST) -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		$(GOFMT) -l -w $$d/*.go || ret=$$? ; \
	 done ; exit $$ret

gencode: operator-sdk
	@operator-sdk generate k8s
	@operator-sdk generate openapi

deploy-setup:
	@EXCLUSIONS=() hack/deploy-setup.sh sriov-network-operator

# test-unit:
# 	@go test -v $(PKGS)
test-e2e: operator-sdk
	@EXCLUSIONS=() hack/deploy-setup.sh sriov-network-operator && operator-sdk test local ./test/e2e --go-test-flags "-v" --namespace sriov-network-operator --no-setup
	@hack/undeploy.sh sriov-network-operator
undeploy:
	@hack/undeploy.sh sriov-network-operator

_plugin-%:
	@hack/build-plugins.sh $*

plugins: _plugin-intel _plugin-generic