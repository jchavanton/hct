#!/bin/bash
DIR_PREFIX=`pwd`
CONTAINER=hct_controller
VERSION="0.0.0"
IMAGE=${CONTAINER}:${VERSION}

HCT_DIR="$(dirname "${DIR_PREFIX}")"
echo "${HCT_DIR}"

docker stop ${CONTAINER}
docker rm ${CONTAINER}
docker run -d --net=host \
              --name=${CONTAINER} \
              -v ${HCT_DIR}/client/xml/:/xml \
              -v ${DIR_PREFIX}/public:/go/public \
              -v /var/run/docker.sock:/var/run/docker.sock \
              ${IMAGE}
              # tail -f /dev/null
