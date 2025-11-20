FROM golang:1.24 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./internal ./internal
COPY ./jobs ./jobs
COPY ./cmd ./cmd

RUN CGO_ENABLED=0 GOOS=linux go build -o ./build/scicat_globus_proxy ./cmd/api-server/

FROM alpine:3 AS release

WORKDIR /service

COPY --from=build-stage /app/build/scicat_globus_proxy ./scicat_globus_proxy

# Mount the config file at /service/scicat-globus-proxy-conf.yaml

EXPOSE 8080

ENTRYPOINT [ "/service/scicat_globus_proxy" ]
