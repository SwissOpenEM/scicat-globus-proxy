package api

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=cfg.yaml openapi.yaml
import (
	"fmt"
	"sync"

	"github.com/SwissOpenEM/globus"
	config "github.com/SwissOpenEM/scicat-globus-proxy/internal/config"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/serviceuser"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/tasks"
	util "github.com/SwissOpenEM/scicat-globus-proxy/internal/util"
)

type ServerHandler struct {
	version           string
	globusClient      globus.GlobusClient
	scicatUrl         string
	scicatServiceUser serviceuser.ScicatServiceUser
	facilities        map[string]Facility
	taskPool          tasks.TaskPool
	addTaskMutex      *sync.Mutex
}

type Facility struct {
	Name               string
	Collection         string
	AccessPath         *accessPathTemplate
	AccessValue        *accessPathTemplate
	Direction          config.FacilityDirection
	SourcePath         *facilityPathTemplate
	DestinationPath    *facilityPathTemplate
	CollectionRootPath *scopeTemplate
}

func NewFacility(config config.FacilityConfig) (*Facility, error) {
	var err error
	f := new(Facility)
	f.Name = config.Name
	f.Collection = config.Collection
	f.Direction = config.Direction
	f.AccessPath, err = util.NewTypedTemplate[accessPathContext](config.AccessPath)
	if err != nil {
		return nil, err
	}
	f.AccessValue, err = util.NewTypedTemplate[accessPathContext](config.AccessValue)
	if err != nil {
		return nil, err
	}
	f.SourcePath, err = util.NewTypedTemplate[facilityPathContext](config.SourcePath)
	if err != nil {
		return nil, err
	}
	f.DestinationPath, err = util.NewTypedTemplate[facilityPathContext](config.DestinationPath)
	if err != nil {
		return nil, err
	}
	if config.CollectionRootPath == "" {
		f.CollectionRootPath = nil
	} else {
		f.CollectionRootPath, err = util.NewTypedTemplate[scopeContext](config.CollectionRootPath)
		if err != nil {
			return nil, err
		}
	}
	return f, nil
}

var _ StrictServerInterface = ServerHandler{}

func NewServerHandler(
	version string,
	globusClient globus.GlobusClient,
	scicatUrl string,
	scicatServiceUser serviceuser.ScicatServiceUser,
	facilities *map[string]Facility,
	taskPool tasks.TaskPool) (ServerHandler, error) {
	// create server with service client
	var err error
	if !globusClient.IsClientSet() {
		return ServerHandler{}, fmt.Errorf("AUTH error: Client is nil")
	}

	return ServerHandler{
		version:           version,
		globusClient:      globusClient,
		scicatUrl:         scicatUrl,
		scicatServiceUser: scicatServiceUser,
		facilities:        *facilities,
		taskPool:          taskPool,
		addTaskMutex:      &sync.Mutex{},
	}, err
}

// Helper to get a pointer to a literal value
func getPointerOrNil[T comparable](v T) *T {
	var a T
	if a == v {
		return nil
	} else {
		return &v
	}
}
