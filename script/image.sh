#!/bin/bash
IMAGE_PROJECT=holdnowby
IMAGE_NAME=gophercron

# Make full image name
IMAGE=${IMAGE_PROJECT}/${IMAGE_NAME}:v2.4.1

docker build -t ${IMAGE} .
