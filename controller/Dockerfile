FROM golang:bullseye

ARG VERSION="0.0.0"

COPY main.go /

RUN echo "building media server" \
 	&& cd / && go build main.go

COPY entry.sh /

ENTRYPOINT ["/entry.sh"]
RUN ["chmod", "+x", "/entry.sh"]
