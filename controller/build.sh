#!/bin/bash
CURRENT_COMMIT=$1
CONTAINER="hct_controller"
VERSION="0.0.0"


docker build . -f Dockerfile -t ${CONTAINER}:${VERSION}
