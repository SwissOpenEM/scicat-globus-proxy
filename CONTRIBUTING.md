# Contributor Guide

## Building

```sh
cd cmd/api-server
go build .
```

## Linting

Ensure that `go.mod` and `go.sum` are up-to-date:

```sh
go mod tidy -diff
```

Run linter:

```sh
golangci-lint run
```

## Testing

```sh
go test ./...
```

## Generating API

The API specification is defined in `internal/api/openapi.yaml`. Routes are
automatically generated using
[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen).

After making changes to `openapi.yaml`, regenerate the api bindings.

First, install the tool (first time only):

```sh
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
```

Next, generate the api:

```sh
cd internal/api
go tool oapi-codegen -config cfg.yaml openapi.yaml
```

This should update api.gen.go. Commit any changes.
