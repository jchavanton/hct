#!/bin/bash
VERSION=0.0.0
DIR_PREFIX=`pwd`
IMAGE=freeswitch

docker ps | grep "${IMAGE}" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker kill || /usr/bin/true;
docker container ls -a | grep "${IMAGE}" | cut -d ' ' -f 1 | xargs --no-run-if-empty docker container rm || /usr/bin/true;

docker run -d --net=host \
              --privileged \
              --name=freeswitch \
              --log-driver syslog \
              --log-opt tag="{{.Name}}" \
              --restart unless-stopped \
              -v /tmp:/tmp \
              ${IMAGE}:${VERSION}
