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
		if [ "$1" == "HCT_CLIENT" ] ;then
			scp -r client/* $i:$INSTALL_DIR/client
			scp -r controller/* $i:$INSTALL_DIR/controller
		fi
		if [ "$1" == "HCT_SERVER" ] ;then
			scp -r server/* $i:$INSTALL_DIR/server
			ssh $i "sudo mkdir -p $INSTALL_DIR/freeswitch && sudo chmod -R 777 $INSTALL_DIR/freeswitch"
			scp -r freeswitch/* $i:$INSTALL_DIR/freeswitch
			ssh $i "sudo mkdir -p $INSTALL_DIR/kamailio && sudo chmod -R 777 $INSTALL_DIR/kamailio"
			scp -r kamailio/* $i:$INSTALL_DIR/kamailio
		fi
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
