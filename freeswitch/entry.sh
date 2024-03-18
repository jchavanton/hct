#!/bin/bash -e

ulimit -c unlimited
COREDIR=/tmp
if [ -d $COREDIR ]; then
         echo "ERROR: core dump directory $COREDIR does not exist."
fi
echo "$COREDIR/core.%e.sig%s.%p" > /proc/sys/kernel/core_pattern

LOCAL_IPV4=$(hostname -I | cut -d' ' -f1)
export LOCAL_IPV4=${LOCAL_IPV4}
# CMD="tail -f /dev/null"
if [ "$1" = "" ]; then
	CMD="stdbuf -i0 -o0 -e0 /usr/local/freeswitch/bin/freeswitch -c -nonat"
else
	CMD="$*"
fi
echo "Running [$CMD]"
exec $CMD
echo "exiting ..."
