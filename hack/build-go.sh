#!/usr/bin/env bash

set -eu

REPO=github.com/pliurh/sriov-network-operator
WHAT=${WHAT:-sriov-network-operator}
GOFLAGS=${GOFLAGS:-}
GLDFLAGS=${GLDFLAGS:-}

# eval $(go env | grep -e "GOHOSTOS" -e "GOHOSTARCH")

# : "${GOOS:=${GOHOSTOS}}"
# : "${GOARCH:=${GOHOSTARCH}}"

# Go to the root of the repo
GOOS=linux
GOARCH=amd64

cdup="$(git rev-parse --show-cdup)" && test -n "$cdup" && cd "$cdup"

if [ -z ${VERSION_OVERRIDE+a} ]; then
	echo "Using version from git..."
	VERSION_OVERRIDE=$(git describe --abbrev=8 --dirty --always)
fi

GLDFLAGS+="-X ${REPO}/pkg/version.Raw=${VERSION_OVERRIDE}"

# eval $(go env)

if [ -z ${BIN_PATH+a} ]; then
	export BIN_PATH=build/_output/${GOOS}/${GOARCH}
fi

mkdir -p ${BIN_PATH}

CGO_ENABLED=1

echo "Building ${REPO}/cmd/${WHAT} (${VERSION_OVERRIDE})"
CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS} -ldflags "${GLDFLAGS} -s -w" -tags no_openssl -o ${BIN_PATH}/${WHAT} ${REPO}/cmd/${WHAT}
