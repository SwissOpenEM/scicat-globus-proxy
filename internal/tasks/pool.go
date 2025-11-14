package tasks

import (
	"fmt"
	"sync"
	"time"

	"github.com/SwissOpenEM/globus"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/serviceuser"
	"github.com/alitto/pond/v2"
)

type TaskPool struct {
	scicatUrl         string
	globusClient      globus.GlobusClient
	scicatServiceUser serviceuser.ScicatServiceUser
	pool              pond.Pool
	taskPollInterval  time.Duration
	cancelTask        map[string]chan struct{}
	cancelMutex       *sync.Mutex
}

type JobNotExistError struct {
	msg string
}

func (e *JobNotExistError) Error() string {
	return e.msg
}

func CreateTaskPool(scicatUrl string, globusClient globus.GlobusClient, scicatServiceUser serviceuser.ScicatServiceUser, maxConcurrency int, queueSize int, taskPollInterval uint) TaskPool {
	return TaskPool{
		scicatUrl:         scicatUrl,
		globusClient:      globusClient,
		scicatServiceUser: scicatServiceUser,
		pool:              pond.NewPool(maxConcurrency, pond.WithQueueSize(queueSize)),
		taskPollInterval:  time.Duration(taskPollInterval) * time.Second,
		cancelTask:        map[string]chan struct{}{},
		cancelMutex:       &sync.Mutex{},
	}
}

func (tp TaskPool) AddTransferTask(globusTaskId string, datasetPid string, scicatJobId string) pond.Task {
	tp.cancelTask[scicatJobId] = make(chan struct{})
	task := transferTask{
		scicatUrl:         &tp.scicatUrl,
		globusClient:      tp.globusClient,
		scicatServiceUser: tp.scicatServiceUser,
		globusTaskId:      globusTaskId,
		datasetPid:        datasetPid,
		scicatJobId:       scicatJobId,
		taskPollInterval:  tp.taskPollInterval,
		cancel:            tp.cancelTask[scicatJobId],
		cleanup: func() {
			tp.cancelMutex.Lock()
			defer tp.cancelMutex.Unlock()
			delete(tp.cancelTask, scicatJobId)
		},
	}

	return tp.pool.Submit(task.execute)
}

func (tp TaskPool) CancelTransferTask(scicatJobId string) error {
	tp.cancelMutex.Lock()
	defer tp.cancelMutex.Unlock()
	if cancelChannel, ok := tp.cancelTask[scicatJobId]; ok {
		cancelChannel <- struct{}{}
		return nil
	}
	return &JobNotExistError{fmt.Sprintf("job with ID '%s' does not exist or is already cancelled/removed", scicatJobId)}
}

func (tp TaskPool) DeleteTransferTask(scicatJobId string) error {
	_ = tp.CancelTransferTask(scicatJobId)
	token, err := tp.scicatServiceUser.GetToken()
	if err != nil {
		return err
	}
	return DeleteScicatJob(tp.scicatUrl, token, scicatJobId)
}

func (tp TaskPool) CanSubmitJob() bool {
	if tp.pool.QueueSize() == 0 {
		return true
	}
	return tp.pool.WaitingTasks() < uint64(tp.pool.QueueSize())
}

func (tp TaskPool) IsQueueSizeLimited() bool {
	return tp.pool.QueueSize() > 0
}
