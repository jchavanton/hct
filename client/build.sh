#!/bin/sh
VERSION="0.0.4"
NAME="hct_client"
docker build . --no-cache -t ${NAME}
docker tag ${NAME}:latest ${NAME}:${VERSION}
