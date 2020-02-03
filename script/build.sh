#!/bin/sh
BUILD_DIR=${PWD}/_build

GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o ${BUILD_DIR}/gophercron ./cmd/

mkdir ${BUILD_DIR}/config
mkdir ${BUILD_DIR}/view
cp ./cmd/service/conf/config-default.toml ${BUILD_DIR}/config/service-config-default.toml
cp ./cmd/client/conf/config-default.toml ${BUILD_DIR}/config/client-config-default.toml
cp -r ./dist/view/* ${BUILD_DIR}/view
