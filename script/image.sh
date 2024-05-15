#!/bin/bash
IMAGE_PROJECT=holdnowby
IMAGE_NAME=gophercron
VERSION=$1

# Make full image name
IMAGE=${IMAGE_PROJECT}/${IMAGE_NAME}:${VERSION}

docker build -t ${IMAGE} .

docker push ${IMAGE_PROJECT}/${IMAGE_NAME}:${VERSION}
