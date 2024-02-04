#!/bin/bash
DIR_PREFIX=`pwd`
CONTAINER=hct_controller
VERSION="0.0.0"
IMAGE=${CONTAINER}:${VERSION}
docker stop ${CONTAINER}
docker rm ${CONTAINER}
docker run -d --net=host \
              --name=${CONTAINER} \
              -v ${DIR_PREFIX}/upload:/go/upload \
              -v ${DIR_PREFIX}/public:/go/public \
              ${IMAGE}
              # tail -f /dev/null
