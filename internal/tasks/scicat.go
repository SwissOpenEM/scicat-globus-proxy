package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/SwissOpenEM/scicat-globus-proxy/internal/serviceuser"
	"github.com/SwissOpenEM/scicat-globus-proxy/jobs"
)

type scicatJobPost struct {
	Type         string         `json:"type"`
	JobParams    jobs.JobParams `json:"jobParams"`
	OwnerUser    string         `json:"ownerUser,omitempty"`
	OwnerGroup   string         `json:"ownerGroup,omitempty"`
	ContactEmail string         `json:"contactEmail,omitempty"`
}

type scicatGlobusTransferJobPatch struct {
	StatusCode      string               `json:"statusCode,omitempty"`
	StatusMessage   string               `json:"statusMessage,omitempty"`
	JobResultObject jobs.JobResultObject `json:"jobResultObject,omitempty"`
}

type scicatErrorResp struct {
	Status  string `json:"status"`
	Message string `json:"Message"`
}

type JobDeleteNotExist struct {
	scicatErrorResp
}

func (e *JobDeleteNotExist) Error() string {
	return e.Message
}

func CreateGlobusTransferScicatJob(scicatUrl string, scicatToken string, ownerGroup string, datasetPid string, globusTaskId string) (jobs.ScicatJob, error) {
	url, err := url.JoinPath(scicatUrl, "api", "v4", "jobs")
	if err != nil {
		return jobs.ScicatJob{}, err
	}

	reqBody, err := json.Marshal(scicatJobPost{
		Type:       "globus_transfer_job",
		OwnerGroup: ownerGroup,
		JobParams: jobs.JobParams{
			DatasetList: []jobs.Dataset{
				{
					Pid:   datasetPid,
					Files: []string{},
				},
			},
		},
	})
	if err != nil {
		return jobs.ScicatJob{}, err
	}

	slog.Debug("Creating globus_transfer_job on SciCat", "url", url, "body", reqBody)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return jobs.ScicatJob{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+scicatToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return jobs.ScicatJob{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return jobs.ScicatJob{}, fmt.Errorf("authentication failed: user is not logged in")
	}
	if resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return jobs.ScicatJob{}, fmt.Errorf("unknown error occured with status: '%s', body: '%s'", resp.Status, string(bodyBytes))
	}

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return jobs.ScicatJob{}, err
	}

	job := jobs.ScicatJob{}
	err = json.Unmarshal(respBodyBytes, &job)
	if err != nil {
		return job, err
	}

	return UpdateGlobusTransferScicatJob(scicatUrl, scicatToken, job.ID, "001", "started", jobs.JobResultObject{
		GlobusTaskId:     globusTaskId,
		BytesTransferred: 0,
		FilesTransferred: 0,
		FilesTotal:       0,
		Status:           jobs.Transferring,
		Error:            "",
	})
}

func UpdateGlobusTransferScicatJob(scicatUrl string, scicatToken string, jobId string, statusCode string, statusMessage string, jobStatus jobs.JobResultObject) (jobs.ScicatJob, error) {
	url, err := url.JoinPath(scicatUrl, "api", "v4", "jobs", url.QueryEscape(jobId))
	if err != nil {
		return jobs.ScicatJob{}, err
	}

	reqBody, err := json.Marshal(scicatGlobusTransferJobPatch{
		StatusCode:      statusCode,
		StatusMessage:   statusMessage,
		JobResultObject: jobStatus,
	})
	if err != nil {
		return jobs.ScicatJob{}, err
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return jobs.ScicatJob{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+scicatToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return jobs.ScicatJob{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 403:
		return jobs.ScicatJob{}, fmt.Errorf("cannot patch dataset: forbidden")
	case 400:
		return jobs.ScicatJob{}, fmt.Errorf("cannot patch dataset: invalid job id")
	case 200:
		break
	default:
		body, _ := io.ReadAll(resp.Body)
		return jobs.ScicatJob{}, fmt.Errorf("unknown status encountered: '%s', body: '%s'", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return jobs.ScicatJob{}, err
	}

	job := jobs.ScicatJob{}
	err = json.Unmarshal(body, &job)
	return job, err
}

func DeleteScicatJob(scicatUrl string, scicatToken string, jobId string) error {
	url, err := url.JoinPath(scicatUrl, "api", "v4", "jobs", url.QueryEscape(jobId))
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+scicatToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("couldn't delete job, status 400 - body reading error: %s", err.Error())
		}

		var errResp scicatErrorResp
		err = json.Unmarshal(b, &errResp)
		if err != nil {
			return fmt.Errorf("couldn't delete job, status 400 - unmarshaling error: %s", err.Error())
		}

		if strings.Contains(errResp.Message, "doesn't exist") {
			return &JobDeleteNotExist{scicatErrorResp{Message: fmt.Sprintf("status 400 - %s", errResp.Message)}}
		}
		return fmt.Errorf("status 400 - '%s'", errResp.Message)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("couldn't delete job, unknown status - status code: %d, status: %s", resp.StatusCode, resp.Status)
	}
	return nil
}

func RestoreGlobusTransferJobsFromScicat(scicatUrl string, serviceUser serviceuser.ScicatServiceUser, pool TaskPool) error {
	token, err := serviceUser.GetToken()
	if err != nil {
		return err
	}
	unfinishedJobs, err := jobs.GetJobList(scicatUrl, token, `{"where":{"type":"globus_transfer_job","jobResultObject.completed":false,"jobResultObject.error":""}}`)
	if err != nil {
		return err
	}

	for _, job := range unfinishedJobs {
		if job.JobResultObject.GlobusTaskId == "" {
			slog.Warn("job has no globus task id, so it cannot be resumed", "jobId", job.ID)
			continue
		}
		if len(job.JobParams.DatasetList) > 1 {
			slog.Warn("job has more than one associated dataset, which is not currently supported", "jobId", job.ID, "datasetCount", len(job.JobParams.DatasetList))
			continue
		}
		if len(job.JobParams.DatasetList) <= 0 {
			slog.Warn("job has no datasets associated, so it cannot be resumed", "jobId", job.ID)
		}
		pool.AddTransferTask(job.JobResultObject.GlobusTaskId, job.JobParams.DatasetList[0].Pid, job.ID)
	}

	return nil
}
