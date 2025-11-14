package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"reflect"
	"slices"

	"github.com/SwissOpenEM/globus"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/scicat"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/tasks"
	"github.com/SwissOpenEM/scicat-globus-proxy/jobs"
	"github.com/gin-gonic/gin"
)

type GroupTemplateData struct {
	FacilityName string
}

func (s ServerHandler) PostTransferTask(ctx context.Context, request PostTransferTaskRequestObject) (PostTransferTaskResponseObject, error) {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		return PostTransferTask500JSONResponse{
			Message: getPointerOrNil("context error"),
		}, nil
	}

	// check facility id's and fetch collection id's
	sourceCollectionID, ok := s.facilityCollectionIDs[request.Params.SourceFacility]
	if !ok {
		return PostTransferTask403JSONResponse{
			Message: getPointerOrNil("invalid source facility"),
		}, nil
	}
	destCollectionID, ok := s.facilityCollectionIDs[request.Params.DestFacility]
	if !ok {
		return PostTransferTask403JSONResponse{
			Message: getPointerOrNil("invalid destination facility"),
		}, nil
	}

	// User should have been cached in the context from the auth middleware
	u, ok := ginCtx.Get("scicatUser")
	if !ok {
		return PostTransferTask500JSONResponse{
			Message: getPointerOrNil("no user was found"),
		}, nil
	}

	// fetch scicat user
	scicatUser, ok := u.(scicat.User)
	if !ok {
		return PostTransferTask500JSONResponse{
			Message: getPointerOrNil("invalid user in context"),
			Details: getPointerOrNil(fmt.Sprintf("type found: '%s'", reflect.TypeOf(u))),
		}, nil
	}

	scicatService := scicat.ScicatService{
		Url:   s.scicatUrl,
		Token: scicatUser.ScicatToken,
	}

	dataset, err := scicatService.GetDataset(request.Params.ScicatPid)
	if err != nil {
		slog.Error("error fetching dataset from scicat", "error", err)

		var httpErr *scicat.HttpError
		if errors.As(err, &httpErr) {
			switch httpErr.StatusCode {
			case 400, 401, 403:
				return PostTransferTask400JSONResponse{
					GeneralErrorResponseJSONResponse{
						Message: getPointerOrNil(httpErr.Message),
						Details: getPointerOrNil(httpErr.Details),
					},
				}, nil
			default:
				return PostTransferTask500JSONResponse{
					Message: getPointerOrNil(httpErr.Message),
					Details: getPointerOrNil(httpErr.Details),
				}, nil
			}
		}
		var detailedResponse scicat.DetailedError
		if errors.As(err, &detailedResponse) {
			return PostTransferTask500JSONResponse{
				Message: getPointerOrNil(detailedResponse.Error()),
				Details: getPointerOrNil(detailedResponse.Details),
			}, nil
		}
		return PostTransferTask500JSONResponse{
			Message: getPointerOrNil("unable to fetch dataset " + request.Params.ScicatPid),
		}, nil
	}

	// check for required group membership.
	// facilitySrcGroupTemplate and facilityDstGroupTemplate are checked against the
	// user's access groups (from Profile.AccessGroups in their user token)
	var srcGroupBuf bytes.Buffer
	err = s.srcGroupTemplate.Execute(&srcGroupBuf, GroupTemplateData{FacilityName: request.Params.SourceFacility})
	if err != nil {
		return PostTransferTask500JSONResponse{
			Message: getPointerOrNil("group templating failed with source facility"),
			Details: getPointerOrNil(err.Error()),
		}, nil
	}

	var dstGroupBuf bytes.Buffer
	err = s.dstGroupTemplate.Execute(&dstGroupBuf, GroupTemplateData{FacilityName: request.Params.DestFacility})
	if err != nil {
		return PostTransferTask500JSONResponse{
			Message: getPointerOrNil("group templating failed with destination facility"),
			Details: getPointerOrNil(err.Error()),
		}, nil
	}

	requiredGroups := []string{srcGroupBuf.String(), dstGroupBuf.String(), dataset.OwnerGroup}
	missingGroups := []string{}
	for _, group := range requiredGroups {
		if !slices.Contains(scicatUser.Profile.AccessGroups, group) {
			missingGroups = append(missingGroups, group)
		}
	}

	if len(missingGroups) > 0 {
		return PostTransferTask401JSONResponse{
			Message: getPointerOrNil("you don't have the required access groups to request this transfer"),
			Details: getPointerOrNil(fmt.Sprintf("missing groups: '%v'", missingGroups)),
		}, nil
	}

	// request the transfer
	if s.taskPool.IsQueueSizeLimited() {
		s.addTaskMutex.Lock()
		defer s.addTaskMutex.Unlock()
		if !s.taskPool.CanSubmitJob() {
			return PostTransferTask503JSONResponse{
				Message: getPointerOrNil("the task queue is currently full, try again later..."),
			}, nil
		}
	}

	params := destPathParams{
		DatasetFolder: path.Base(dataset.SourceFolder),
		SourceFolder:  dataset.SourceFolder,
		Pid:           request.Params.ScicatPid,
		PidShort:      path.Base(request.Params.ScicatPid),
		PidPrefix:     path.Dir(request.Params.ScicatPid),
		PidEncoded:    url.PathEscape(request.Params.ScicatPid),
		Username:      scicatUser.Profile.Username,
	}

	sourcePath := dataset.SourceFolder
	destPath, err := s.dstPathTemplate.Execute(params)
	if err != nil {
		return PostTransferTask500JSONResponse{
			Message: getPointerOrNil("couldn't template destination folder for the transfer"),
			Details: getPointerOrNil(err.Error()),
		}, nil
	}

	var globusResult globus.TransferResult
	if request.Body == nil {
		return PostTransferTask400JSONResponse{
			GeneralErrorResponseJSONResponse: GeneralErrorResponseJSONResponse{
				Message: getPointerOrNil("no body was sent with the request"),
			},
		}, nil
	}

	if request.Body.FileList != nil {
		// use filelist
		paths := make([]string, len(*request.Body.FileList))
		isSymlinks := make([]bool, len(*request.Body.FileList))
		for i, file := range *request.Body.FileList {
			paths[i] = file.Path
			isSymlinks[i] = file.IsSymlink
		}
		globusResult, err = s.globusClient.TransferFileList(sourceCollectionID, sourcePath, destCollectionID, destPath, paths, isSymlinks, false)
	} else {
		// sync folders through globus
		globusResult, err = s.globusClient.TransferFolderSync(sourceCollectionID, sourcePath, destCollectionID, destPath, false)
	}
	if err != nil {
		return PostTransferTask400JSONResponse{
			GeneralErrorResponseJSONResponse: GeneralErrorResponseJSONResponse{
				Message: getPointerOrNil("can't request globus transfer"),
				Details: getPointerOrNil(err.Error()),
			},
		}, nil
	}

	serviceUserToken, err := s.scicatServiceUser.GetToken()
	if err != nil {
		_, _ = s.globusClient.TransferCancelTaskByID(globusResult.TaskId) // attempt to cancel transfer
		return PostTransferTask500JSONResponse{
			Message: getPointerOrNil("service user login failed"),
			Details: getPointerOrNil(err.Error()),
		}, nil
	}

	// TODO: replace the service user token with the current user's token if it becomes possible to create the scicatJob as one's own user
	//   , which will happen once the required changes are merged into BE SciCat. If the changes will still not allow this, just
	//   remove this TODO.
	scicatJob, err := tasks.CreateGlobusTransferScicatJob(s.scicatUrl, serviceUserToken, dataset.OwnerGroup, params.Pid, "")
	if err != nil {
		_, _ = s.globusClient.TransferCancelTaskByID(globusResult.TaskId) // attempt to cancel transfer
		return PostTransferTask500JSONResponse{
			Message: getPointerOrNil("failed creating transfer job in SciCat"),
			Details: getPointerOrNil(err.Error()),
		}, nil
	}

	s.taskPool.AddTransferTask(globusResult.TaskId, request.Params.ScicatPid, scicatJob.ID)

	// return response
	return PostTransferTask200JSONResponse{
		JobId: scicatJob.ID,
	}, nil
}

func (s ServerHandler) DeleteTransferTask(ctx context.Context, req DeleteTransferTaskRequestObject) (DeleteTransferTaskResponseObject, error) {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		return DeleteTransferTask500JSONResponse{
			Message: getPointerOrNil("context error"),
		}, nil
	}

	// fetch scicat user
	u, ok := ginCtx.Get("scicatUser")
	if !ok {
		return DeleteTransferTask500JSONResponse{
			Message: getPointerOrNil("no user was found"),
		}, nil
	}

	scicatUser, ok := u.(scicat.User)
	if !ok {
		return DeleteTransferTask500JSONResponse{
			Message: getPointerOrNil("invalid user in context"),
			Details: getPointerOrNil(fmt.Sprintf("type found: '%s'", reflect.TypeOf(u))),
		}, nil
	}

	serviceToken, err := s.scicatServiceUser.GetToken()
	if err != nil {
		return DeleteTransferTask500JSONResponse{
			Message: getPointerOrNil("couldn't access SciCat"),
			Details: getPointerOrNil(fmt.Sprintf("SciCat token renewal failed: %s", err.Error())),
		}, nil
	}

	job, err := jobs.GetJobById(s.scicatUrl, serviceToken, req.ScicatJobId)
	if err != nil {
		return DeleteTransferTask400JSONResponse{
			GeneralErrorResponseJSONResponse: GeneralErrorResponseJSONResponse{
				Message: getPointerOrNil("failed to request job from SciCat"),
				Details: getPointerOrNil(err.Error()),
			},
		}, nil
	}

	if job.OwnerUser != scicatUser.Profile.Username && !slices.Contains(scicatUser.Profile.AccessGroups, job.OwnerGroup) {
		return DeleteTransferTask403JSONResponse{
			Message: getPointerOrNil("you don't have the right to cancel or delete this job"),
		}, nil
	}

	if req.Params.Delete != nil && *req.Params.Delete {
		err = s.taskPool.DeleteTransferTask(req.ScicatJobId)
	} else {
		err = s.taskPool.CancelTransferTask(req.ScicatJobId)
	}

	if err != nil {
		JobNotExistErr := &tasks.JobNotExistError{}
		JobDelete400Err := &tasks.JobDeleteNotExist{}
		JobNotFoundErr := &jobs.JobNotFoundErr{}
		if errors.As(err, &JobNotExistErr) || errors.As(err, &JobDelete400Err) || errors.As(err, &JobNotFoundErr) {
			return DeleteTransferTask400JSONResponse{GeneralErrorResponseJSONResponse{
				Message: getPointerOrNil("the requested job does not exist or is already finished or cancelled"),
			}}, nil
		}
		return DeleteTransferTask500JSONResponse{
			Message: getPointerOrNil("an error occured when attempting to delete and/or cancel your task"),
			Details: getPointerOrNil(err.Error()),
		}, nil
	}

	return DeleteTransferTask200Response{}, nil
}
