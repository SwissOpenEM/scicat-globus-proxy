FROM golang:1.24 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./internal ./internal
COPY ./jobs ./jobs
COPY ./cmd ./cmd

RUN CGO_ENABLED=0 GOOS=linux go build -o ./build/globus_transfer_service ./cmd/api-server/

FROM alpine:3 AS release

WORKDIR /service

COPY --from=build-stage /app/build/globus_transfer_service ./globus_transfer_service
COPY ./example-conf.yaml /service/globus-transfer-service-conf.yaml

EXPOSE 8080

ENTRYPOINT [ "/service/globus_transfer_service" ]
