FROM golang:1.24 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./internal ./internal
COPY ./jobs ./jobs
COPY ./cmd ./cmd

ARG VERSION=DEVELOPMENT_VERSION
RUN CGO_ENABLED=0 GOOS=linux go build -C ./cmd/api-server/ -o /app/build/scicat_globus_proxy  -ldflags="-s -w  -X 'main.version=${VERSION}'"

FROM alpine:3 AS release

WORKDIR /service

COPY --from=build-stage /app/build/scicat_globus_proxy ./scicat_globus_proxy

# Mount the config file at /service/scicat-globus-proxy-conf.yaml

EXPOSE 8080

ENTRYPOINT [ "/service/scicat_globus_proxy" ]
