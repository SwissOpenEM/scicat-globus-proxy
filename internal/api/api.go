package api

import (
	"fmt"
	"sync"
	"text/template"

	"github.com/SwissOpenEM/globus"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/serviceuser"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/tasks"
)

//go:generate oapi-codegen --config=cfg.yaml openapi.yaml

type ServerHandler struct {
	globusClient          globus.GlobusClient
	scicatUrl             string
	scicatServiceUser     serviceuser.ScicatServiceUser
	facilityCollectionIDs map[string]string
	srcGroupTemplate      *template.Template
	dstGroupTemplate      *template.Template
	dstPathTemplate       DestinationTemplate
	taskPool              tasks.TaskPool
	addTaskMutex          *sync.Mutex
}

type ScicatDataset struct {
	OwnerGroup   string `json:"ownerGroup"`
	SourceFolder string `json:"sourceFolder"`
}

var _ StrictServerInterface = ServerHandler{}

func NewServerHandler(globusClient globus.GlobusClient, scopes []string, scicatUrl string, scicatServiceUser serviceuser.ScicatServiceUser, facilityCollectionIDs map[string]string, srcGroupTemplateBody string, dstGroupTemplateBody string, dstPathTemplateBody string, taskPool tasks.TaskPool) (ServerHandler, error) {
	// create server with service client
	var err error
	if !globusClient.IsClientSet() {
		return ServerHandler{}, fmt.Errorf("AUTH error: Client is nil")
	}

	srcGroupTemplate, err := template.New("source group template").Parse(srcGroupTemplateBody)
	if err != nil {
		return ServerHandler{}, err
	}

	dstGroupTemplate, err := template.New("destination group template").Parse(dstGroupTemplateBody)
	if err != nil {
		return ServerHandler{}, err
	}

	dstPathTemplate, err := NewDestinationTemplate(dstPathTemplateBody)
	if err != nil {
		return ServerHandler{}, err
	}

	return ServerHandler{
		scicatUrl:             scicatUrl,
		scicatServiceUser:     scicatServiceUser,
		globusClient:          globusClient,
		facilityCollectionIDs: facilityCollectionIDs,
		srcGroupTemplate:      srcGroupTemplate,
		dstGroupTemplate:      dstGroupTemplate,
		dstPathTemplate:       dstPathTemplate,
		taskPool:              taskPool,
		addTaskMutex:          &sync.Mutex{},
	}, err
}

func getPointerOrNil[T comparable](v T) *T {
	var a T
	if a == v {
		return nil
	} else {
		return &v
	}
}
