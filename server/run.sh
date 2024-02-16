#!/bin/sh
DIR_PREFIX=`pwd`
NAME="hct_client"
IMAGE=${NAME}:latest
CONTAINER=${NAME}

# docker ps | grep '${CONTAINER_NAME}' | cut -d ' ' -f 1 | xargs --no-run-if-empty docker kill || /usr/bin/true;
# docker container ls -a | grep ${CONTAINER_NAME} | cut -d ' ' -f 1 | xargs --no-run-if-empty docker container rm || /usr/bin/true;

docker stop ${CONTAINER}
docker rm ${CONTAINER}
docker run -d --net=host --name=${CONTAINER} \
	-v ${PWD}/xml:/xml \
	${IMAGE}
