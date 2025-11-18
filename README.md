# SciCat Globus Proxy

## Summary

This service allows for requesting [Globus](https://www.globus.org) transfers using the [SciCat](https://scicatproject.github.io) token. It complements the [Ingestor Service](https://github.com/SwissOpenEM/Ingestor/), and allows users to request globus data transfers without exposing globus credentials to the end user. It also keeps track of ongoing transfers, and their state can be polled from this service.

It relies on a single globus [service account](https://docs.globus.org/guides/recipes/automate-with-service-account/) to request and track transfers, which can be set using environment variables.

It also assumes that there's a set of possible destinations and sources. Whether it's possible to ingest from the source to the given destination is defined by the SciCat user's current set of groups. The group associations are given to this service as a config.

## Details

![Overview diagram of the SciCat Globus Proxy](docs/SwissOpenEM/scicat-globus-proxy-diagram.png)

The SciCat Globus Proxy (GTS) is a REST API, documented in the [OpenAPI description](internal/api/openapi.yaml). This is called by the Ingestor Service (3) in when a newly created dataset is ready to be uploaded. The main role of the GTS is to validate the user's SciCat credentials and verify authorization to upload the dataset (4), then to request a globus transfer on the user's behalf (5). This allows globus to be used in environments where end users should not have direct access to globus credentials for security purposes, such as when Globus Guest Collections are not available for isolating user data.

The transfer status is tracked in a SciCat job. The jobId is returned by the `/transfer` endpoint. GTS will continually update the SciCat job with the current status, which can be queried from the scicat backend:

```sh
curl -H 'accept: application/json' '${scicatUrl}/api/v4/jobs/${jobId}' \
```

The swagger docs are accessible on running instances at `/docs/index.html`. The OpenAPI spec is available at `/openapi.yaml`.

## Configuration

The configuration file `scicat-globus-proxy.config.yaml` can be put into two locations:

1. Next to the executable (taking precedence)
2. Into `$USERCONFIGDIR/scicat-globus-proxy` where `$USERCONFIGDIR` is resolved like this:

   - Unix: `$XDG_CONFIG_HOME/scicat-globus-proxy/scicat-globus-proxy-config.yaml` if non-empty, else `$HOME/.config/scicat-globus-proxy/scicat-globus-proxy-config.yaml`
   - MacOS: `$HOME/Library/Application Support/scicat-globus-proxy/scicat-globus-proxy-config.yaml`
   - Windows: `%AppData%\scicat-globus-proxy\scicat-globus-proxy-config.yaml`

   See <https://pkg.go.dev/os#UserConfigDir/> for details.

You can find an example of the settings at [`example-conf.yaml`](example-conf.yaml)

- `scicatUrl` - the **base** url fo the instance of scicat to use (without the `/api/v[X]` part). (required)
- `port` - the port at which the server should run. (required)
- `facilities` - a list of facilities available for transfer. Facilities have the following properties:
  - `name` - a unique name for the facility, used in transfer requests (required)
  - `collection` - the globus collection ID (required)
  - `scopes` - the globus scopes to use for the client connection. Access is required to transfer api and specific collections. Default: `["urn:globus:auth:scope:transfer.api.globus.org:all[*https://auth.globus.org/scopes/{{.collection}}/data_access]"]`. Available template variables:
    - `Name`
    - `Collection`
  - `accessPath` - a path relative to the OAuth identity to find authentication information granting access to this facility. Should point to an array of strings. Default: `profile.accessGroups`
  - `accessValue` - a required string within the identy object pointed to by `accessPath` (eg a group name). Default: `{{.Name}}`. Available template variables:
    - `Name`
  - `direction` - valid transfer directions for this facility.
    - `SRC` - Only valid as a source
    - `DST` - Only valid as a destination
    - `BOTH` - Valid as either source or destination (default)
  - `sourcePath` - path *relative to the globus endpoint root* for datasets when this facility is used as the source for transfers. Default: `/{{ .RelativeSourceFolder }}`. Available template variables:
    - `Pid`:                  dataset `pid` property
    - `PidPrefix`:            prefix of the pid (before the slash)
    - `PidShort`:             pid with out the prefix
    - `PidEncoded`:           url-encoded PID
    - `SourceFolder`:         dataset `sourceFolder` property
    - `RelativeSourceFolder`: sourceFolder after stripping the `collectionRootPath` prefix.
    - `DatasetFolder`:        base name of `sourceFolder`
    - `Username`:             username of the current scicat user
  - `destinationPath` - path *relative to the globus endpoint root* for datasets when this facility is used as the destination for transfers. Default: `/{{ .RelativeSourceFolder }}`. Available template variables are the same as `sourcePath`.
  - `collectionRootPath` - Path of the globus root collection. All datasets are required to be contained within this directory for the source facility.
- `task` - a set of settings for configuring the handling of transfer tasks. (optional)
  - `maxConcurrency` - maximum number of transfer tasks executed in parallel. (default: 10)
  - `queueSize` - how many tasks can be put in a queue (0 is infinite). (default: 0)
  - `pollInterval` - the amount of seconds to wait before a task polls Globus again to update the status of the transfer. (default: 10)

## Environment variables

- `GLOBUS_CLIENT_ID` - the client id for the service account (2-legged OAUTH, trusted client model)
- `GLOBUS_CLIENT_SECRET` - the client secret for the service account (2-legged OAUTH, trusted client model)
- `SCICAT_SERVICE_USER_USERNAME` - the username for the service user to use for creating transfer jobs in scicat
- `SCICAT_SERVICE_USER_PASSWORD` - the above user's password

## Docker images

Docker images are built and pushed for every modification and tags added to the `main`
branch.
