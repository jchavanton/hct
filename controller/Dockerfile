FROM golang:bullseye

ARG VERSION="0.0.0"

COPY go.mod /
COPY go.sum /
RUN go get github.com/google/uuid
RUN go get github.com/docker/docker/api/types/container
RUN go get github.com/docker/docker/client
RUN go get github.com/ory/dockertest/v3/docker/types
RUn go get github.com/rabbitmq/amqp091-go

COPY main.go /

RUN echo "building hct controller" \
 && cd / && go build main.go

COPY entry.sh /

ENTRYPOINT ["/entry.sh"]
RUN ["chmod", "+x", "/entry.sh"]
