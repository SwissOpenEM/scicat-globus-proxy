# Globus Transfer Service

## Summary

This service allows for requesting [Globus](https://www.globus.org) transfers using the [Scicat](https://scicatproject.github.io) token. It complements the [Ingestor Service](https://github.com/SwissOpenEM/Ingestor/), and allows users to request globus data transfers without exposing globus credentials to the end user. It also keeps track of ongoing transfers, and their state can be polled from this service.

It relies on a single globus [service account](https://docs.globus.org/guides/recipes/automate-with-service-account/) to request and track transfers, which can be set using environment variables.

It also assumes that there's a set of possible destinations and sources. Whether it's possible to ingest from the source to the given destination is defined by the Scicat user's current set of groups. The group associations are given to this service as a config.

## Details

![Overview diagram of the Globus Transfer Service](docs/globus-transfer-service-diagram.png)

The Globus Transfer Service (GTS) is a REST API, documented in the [OpenAPI description](internal/api/openapi.yaml). This is called by the Ingestor Service (3) in when a newly created dataset is ready to be uploaded. The main role of the GTS is to validate the user's SciCat credentials and verify authorization to upload the dataset (4), then to request a globus transfer on the user's behalf (5). This allows globus to be used in environments where end users should not have direct access to globus credentials for security purposes, such as when Globus Guest Collections are not available for isolating user data.

The transfer status is tracked in a SciCat job. The jobId is returned by the `/transfer` endpoint. GTS will continually update the SciCat job with the current status, which can be queried from the scicat backend:

```sh
curl -H 'accept: application/json' '${scicatUrl}/api/v4/jobs/${jobId}' \
```

## Configuration

You can find an example of the settings at [`example-conf.yaml`](example-conf.yaml)

 - `scicatUrl` - the **base** url fo the instance of scicat to use (without the `/api/v[X]` part)
 - `facilityCollectionIDs` - a map of facility names (identifiers) to their collection id's
 - `globusScopes` - the scopes to use for the client connection. Access is required to transfer api and specific collections
 - `port` - the port at which the server should run
 - `facilitySrcGroupTemplate` - the template to use for groups (their names) that allow users to use facilities listed in `facilityCollectionIDs` as the source of their transfer requests
 - `facilityDstGroupTemplate` - same as above, but as the destination of their transfer requests
 - `destinationPathTemplate` - the template to use for determining the path at the destination of the transfer.
    The following template variables are supported:
      - Pid:           dataset `pid` property
      - PidPrefix:     prefix of the pid (before the slash)
      - PidShort:      pid with out the prefix
      - PidEncoded:    url-encoded PID
      - SourceFolder:  dataset `sourceFolder` property
      - DatasetFolder: base name of `sourceFolder`
      - Username:      username of the current scicat user
 - `task` - a set of settings for configuring the handling of transfer tasks
   - `maxConcurrency` - maximum number of transfer tasks executed in parallel
   - `queueSize` - how many tasks can be put in a queue (0 is infinite)
   - `pollInterval` - the amount of seconds to wait before a task polls Globus again to update the status of the transfer

## Environment variables

 - `GLOBUS_CLIENT_ID` - the client id for the service account (2-legged OAUTH, trusted client model)
 - `GLOBUS_CLIENT_SECRET` - the client secret for the service account (2-legged OAUTH, trusted client model)
 - `SCICAT_SERVICE_USER_USERNAME` - the username for the service user to use for creating transfer jobs in scicat
 - `SCICAT_SERVICE_USER_PASSWORD` - the above user's password

## Docker images
Docker images are built and pushed for every modification and tags added to the `master` branch