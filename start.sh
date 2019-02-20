#!/bin/sh
cd /app
/usr/bin/lftp-server -max-retries ${max_retries} -n ${N} -o ${O} -p ${P} -rpc-listen-port ${rpc_listen_port} -rpc-secret ${SECRET} -s ${SCRIPT}
