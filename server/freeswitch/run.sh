#!/bin/bash
VERSION=0.0.1
DIR_PREFIX=`pwd`

if [ -z "${1}" ]; then
  echo >&2 "No tag was provided to run. Usage: ./run.sh GIT_COMMIT_ID_GOES_HERE"
  exit 2
fi

IMAGE="848116219031.dkr.ecr.us-west-1.amazonaws.com/com.connectingpointpro.webrtc-gw-freeswitch:${1}"

## For local-dev stop things. This is already done by CodeDeploy but its safe to re-run
docker ps | grep 'freeswitch' | cut -d ' ' -f 1 | xargs --no-run-if-empty docker kill || /usr/bin/true;
docker container ls -a | grep freeswitch | cut -d ' ' -f 1 | xargs --no-run-if-empty docker container rm || /usr/bin/true;

docker run -d --net=host \
              --privileged \
              --name=freeswitch \
              --log-driver syslog \
              --log-opt tag="{{.Name}}" \
              --restart unless-stopped \
              -v /tmp:/tmp \
              ${IMAGE}
