#!/bin/bash

INSTALL_PREFIX="/opt/"

declare -a hct_hosts=("HCT_CLIENT" "HCT_SERVER")

deploy_hct_config() {
	ROLE="hct"
	INSTALL_DIR="${INSTALL_PREFIX}/${ROLE}"
	for i in "${hct_hosts[@]}"
	do
		if [ "$1" != "all" ] && [ "$1" != "$i" ] ; then continue; fi
		printf "\nuploading to [$i]\n"
		ssh $i sudo apt install -y sqlite3 docker.io
		ssh $i "sudo mkdir -p $INSTALL_DIR && sudo chmod -R 777 $INSTALL_DIR \
		        && sudo mkdir -p $INSTALL_DIR/hct \
		        && sudo chmod -R 777 $INSTALL_DIR"
		scp -r * $i:$INSTALL_DIR
		ssh $i "sudo chown -R root.root $INSTALL_DIR"
		done
}

instruction() {
	printf  "\nYou can specify a host name :\n\n"
	for i in "${hct_hosts[@]}"
	do
		echo "./deploy.sh $i"
	done
}

TARGET=$1
if [ "${TARGET}" == "" ]
then
	instruction
	deploy_hct_config 
	exit
fi

deploy_hct_config $1
