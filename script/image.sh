#!/bin/bash
IMAGE_PROJECT=holdnowby
IMAGE_NAME=gophercron

# Make full image name
IMAGE=${IMAGE_PROJECT}/${IMAGE_NAME}:v2.3.0-alpha.1

docker build -t ${IMAGE} .
