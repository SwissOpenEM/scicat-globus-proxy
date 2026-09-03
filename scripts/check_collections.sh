#!/bin/bash
# Check all collections in the config file(s) to see if the client credentials can get a token for them
# Required Environment Variables:
#   GLOBUS_CLI_CLIENT_ID
#   GLOBUS_CLI_CLIENT_SECRET
# Required Tools:
#   yq
#   curl
# usage: ./check_collections.sh [-v] [configs...]
set -euo pipefail

VERBOSE=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -*)
            echo "Unknown option: $1"
            exit 1
            ;;
        *)
            break
            ;;
    esac
done

FILES=("${@:-scicat-globus-proxy-config.yaml}")

: ${GLOBUS_CLI_CLIENT_ID:?GLOBUS_CLI_CLIENT_ID must be set}
: ${GLOBUS_CLI_CLIENT_SECRET:?GLOBUS_CLI_CLIENT_SECRET must be set}

globus_scope () {
    local collection="$1"
    echo "urn:globus:auth:scope:transfer.api.globus.org:all[*https://auth.globus.org/scopes/${collection}/data_access]"
}

read_collections () {
    for file in "$@"; do
        yq '.facilities[]|"\(.name) \(.collection)"' "$file"
    done
}

RETURN_CODE=0

while read -r NAME COLLECTION; do
    token=$(
        curl -s -u "${GLOBUS_CLI_CLIENT_ID}:${GLOBUS_CLI_CLIENT_SECRET}" --basic \
            -XPOST https://auth.globus.org/v2/oauth2/token   \
            --data-urlencode "grant_type=client_credentials" \
            --data-urlencode "scope=$(globus_scope "$COLLECTION")" 2>&1
    )
    err_status=$?
    if [[ $err_status -eq 0 ]] && { echo "$token" | yq -e 'has("access_token")' >/dev/null 2>&1 ; }; then
        printf "%s %-15s %-8s %s\n" "✅" "$NAME" success "$COLLECTION"
    else
        printf "%s %-15s %-8s %s\n" "❌" "$NAME" "error:$err_status" "$COLLECTION"
        RETURN_CODE=1
    fi
    if [[ $VERBOSE == true ]]; then
        echo "$token" | yq -P -I 2 .
    fi
done < <(read_collections "${FILES[@]}" | sort -u)
exit $RETURN_CODE
