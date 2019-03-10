FROM alpine:latest

LABEL maintainer="Fixer <letsjustfixit@users.noreply.github.com>"
#The maximum number of sequential tries of an operation without success. Possible values: 1-100 (default 5)
ENV max_retries  5
#Number of connections to use when downloading single file. Possible values: 1-100 (default 4)
ENV N  4
#Output directory (optional, default value is the current working directory)
#ENV O "."
#Number of files to download in parallel when mirroring directories. Possible values: 1-10 (default 1)
ENV P 3
#Specify a port number for JSON-RPC server to listen to. Possible values: 1024-65535 (default 7800)
ENV rpc_listen_port 7800
#Set RPC secret authorization token (required)
ENV SECRET "SECRET"
#Script to run after successful download
ENV SCRIPT "/usr/bin/script.sh"
#URL of the LFTP instance
ENV LFTP_URL ""
#URL of the FelTamadas instance
ENV FELTAMHU_URL ""

VOLUME ["/app"]

RUN apk --no-cache add lftp lame curl; \
  echo "set ftp:ssl-allow no" > ~/.lftprc;\
  lftp --version

ADD ./lftp-server /usr/bin/lftp-server
ADD ./start.sh /usr/bin/start.sh
ADD ./script.sh /usr/bin/script.sh

#ENTRYPOINT [ "/usr/bin/lftp" ]

WORKDIR "/app"

#CMD ["sh","/usr/bin/lftp-server -max-retries ${max_retries} -n ${N} -o ${O} -p ${P} -rpc-listen-port ${rpc_listen_port} -rpc-secret ${SECRET} -s ${SCRIPT}"]
#ENTRYPOINT ["/usr/bin/lftp-server -rpc-secret SECRET"]
#CMD ["lftp-server", "-rpc-secret SECRET"]
ENTRYPOINT ["/usr/bin/start.sh"]
