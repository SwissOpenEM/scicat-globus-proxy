// Interact with SciCat backend service
package scicat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

type ScicatService struct {
	Url   string
	Token string
}

// Implements ScicatService interface
var _ ScicatServiceInterface = (*ScicatService)(nil)

// Interface for calls to the SciCat backend
//
// Error() messages are intended to be returned to users.
// Errors may additionally contain GetDetails(), intended for logging, or wrapped errors.
type ScicatServiceInterface interface {
	GetUserIdentity() (User, error)
	GetDataset(datasetPid string) (ScicatDataset, error)
}

// Allows additional details to be attached to an error
type DetailedError struct {
	Message string
	Details string
	Err     error
}

var _ error = (*DetailedError)(nil)

func (e DetailedError) Error() string {
	return fmt.Sprintf("%v: %v\n%v", e.Message, e.Details, e.Err.Error())
}

func (e DetailedError) Unwrap() error {
	return e.Err
}

type HttpError struct {
	StatusCode int
	DetailedError
}

var _ error = (*HttpError)(nil)

// Get the SciCat user identity associated with the provided API key
func (scicat *ScicatService) GetUserIdentity() (User, error) {
	var user User
	userIdentityUrl, err := url.JoinPath(scicat.Url, "api", "v3", "users", "my", "identity")
	if err != nil {
		return user, DetailedError{
			Message: "server error, couldn't create request url for scicat token verification request",
			Err:     err,
		}
	}

	slog.Debug("Getting SciCat user identity", "url", userIdentityUrl)
	req, err := http.NewRequest("GET", userIdentityUrl, nil)
	if err != nil {
		return user, DetailedError{
			Message: "unable to authenticate SciCat user. Failed to create request url",
			Err:     err,
		}
	}
	req.Header.Set("Authorization", "Bearer "+scicat.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return user, DetailedError{
			Message: "unable to authenticate SciCat user. The request from SciCat failed",
			Err:     err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)

		return user, HttpError{
			resp.StatusCode,
			DetailedError{
				Details: fmt.Sprintf("status: '%d', body: '%s'", resp.StatusCode, string(body)),
				Message: "unable to authenticate SciCat user. The access token provided with the request is invalid",
			},
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return user, DetailedError{
			Message: "unable to authenticat SciCat user. Error reading SciCat response",
			Err:     err,
		}
	}

	err = json.Unmarshal(body, &user)
	if err != nil {
		return user, DetailedError{
			Message: "unnable to authenticat SciCat user. Error parsing the user identity response",
			Err:     err,
		}
	}

	user.ScicatToken = scicat.Token

	return user, nil
}

func (scicat *ScicatService) GetOrigDatablocks(scicatPid string) ([]ScicatOrigDatablock, error) {

	origDatablocksUrl, err := url.JoinPath(scicat.Url, "api", "v3", "datasets", url.QueryEscape(scicatPid), "origdatablocks")

	if err != nil {
		return []ScicatOrigDatablock{}, errors.New("couldn't create dataset request url")
	}

	origDatablocksReq, err := http.NewRequest("GET", origDatablocksUrl, nil)
	if err != nil {
		return []ScicatOrigDatablock{}, DetailedError{
			Message: "couldn't generate dataset request",
			Err:     err,
		}
	}
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJfaWQiOiI2ODgxZjc0NGFkNWQ5NGNlMzc0Y2ExYzAiLCJ1c2VybmFtZSI6InBoaWxpcHAiLCJlbWFpbCI6InBoaWxpcHAud2lzc21hbm5AZXRoei5jaCIsImF1dGhTdHJhdGVneSI6Im9pZGMiLCJfX3YiOjAsImlkIjoiNjg4MWY3NDRhZDVkOTRjZTM3NGNhMWMwIiwidXNlcklkIjoiNjg4MWY3NDRhZDVkOTRjZTM3NGNhMWMwIiwiaWF0IjoxNzYzNzE1MDA5LCJleHAiOjE3NjM3NTEwMDl9.FqqwzZG7RkEWNKLfQ3ohffOIc6TCXkWySLpe8ucWbsU"
	origDatablocksReq.Header.Set("Authorization", "Bearer "+token)

	slog.Debug("Fetching dataset from SciCat", "url", origDatablocksUrl)

	origDatablocksResp, err := http.DefaultClient.Do(origDatablocksReq)
	if err != nil {
		return []ScicatOrigDatablock{}, DetailedError{
			Message: "couldn't send dataset request to scicat backend",
			Err:     err,
		}
	}
	defer origDatablocksResp.Body.Close()

	origDatablocksRespBody, err := io.ReadAll(origDatablocksResp.Body)
	if err != nil {
		return []ScicatOrigDatablock{}, DetailedError{
			Message: "failed to read response body",
			Err:     err,
		}
	}

	var origDatablocks []ScicatOrigDatablock
	err = json.Unmarshal(origDatablocksRespBody, &origDatablocks)
	if err != nil {
		return []ScicatOrigDatablock{}, DetailedError{
			Message: "failed to unmarshal response body",
			Err:     err,
		}
	}
	return origDatablocks, nil

}

func (scicat *ScicatService) GetDataset(scicatPid string) (ScicatDataset, error) {
	// fetch related dataset
	datasetUrl, err := url.JoinPath(scicat.Url, "api", "v3", "datasets", url.QueryEscape(scicatPid))
	if err != nil {
		return ScicatDataset{}, errors.New("couldn't create dataset request url")
	}

	datasetReq, err := http.NewRequest("GET", datasetUrl, nil)
	if err != nil {
		return ScicatDataset{}, DetailedError{
			Message: "couldn't generate dataset request",
			Err:     err,
		}
	}
	datasetReq.Header.Set("Authorization", "Bearer "+scicat.Token)

	slog.Debug("Fetching dataset from SciCat", "url", datasetUrl)

	datasetResp, err := http.DefaultClient.Do(datasetReq)
	if err != nil {
		return ScicatDataset{}, DetailedError{
			Message: "couldn't send dataset request to scicat backend",
			Err:     err,
		}
	}
	defer datasetResp.Body.Close()

	if datasetResp.StatusCode != 200 {
		body, _ := io.ReadAll(datasetResp.Body)
		return ScicatDataset{}, DetailedError{
			Message: "the dataset with the given pid does not exist or you don't have access rights to it",
			Details: fmt.Sprintf("response status '%d', body '%s'", datasetResp.StatusCode, string(body)),
		}
	}

	datasetRespBody, err := io.ReadAll(datasetResp.Body)
	if err != nil {
		return ScicatDataset{}, DetailedError{
			Message: "failed to read response body",
			Err:     err,
		}
	}

	var dataset ScicatDataset
	err = json.Unmarshal(datasetRespBody, &dataset)
	if err != nil {
		return ScicatDataset{}, DetailedError{
			Message: "failed to unmarshal response body",
			Err:     err,
		}
	}
	return dataset, nil
}
