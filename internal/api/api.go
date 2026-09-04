package api

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=cfg.yaml openapi.yaml
import (
	"fmt"
	"sync"

	config "github.com/SwissOpenEM/scicat-globus-proxy/internal/config"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/globusauth"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/serviceuser"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/tasks"
	util "github.com/SwissOpenEM/scicat-globus-proxy/internal/util"
)

type ServerHandler struct {
	version             string
	globusClientManager *globusauth.ClientManager
	scicatUrl           string
	scicatServiceUser   serviceuser.ScicatServiceUser
	facilities          map[string]Facility
	taskPool            tasks.TaskPool
	addTaskMutex        *sync.Mutex
}

type Facility struct {
	Name            string
	Collection      string
	AccessPath      *accessPathTemplate
	AccessValue     *accessPathTemplate
	Direction       config.FacilityDirection
	SourcePath      *facilityPathTemplate
	DestinationPath *facilityPathTemplate
}

func NewFacility(config config.FacilityConfig) (*Facility, error) {
	var err error
	facility := new(Facility)
	facility.Name = config.Name
	facility.Collection = config.Collection
	facility.Direction = config.Direction
	facility.AccessPath, err = util.NewTypedTemplate[accessPathContext](*config.AccessPath)
	if err != nil {
		return nil, err
	}
	facility.AccessValue, err = util.NewTypedTemplate[accessPathContext](*config.AccessValue)
	if err != nil {
		return nil, err
	}
	facility.SourcePath, err = util.NewTypedTemplate[facilityPathContext](config.SourcePath)
	if err != nil {
		return nil, err
	}
	facility.DestinationPath, err = util.NewTypedTemplate[facilityPathContext](config.DestinationPath)
	if err != nil {
		return nil, err
	}
	return facility, nil
}

var _ StrictServerInterface = ServerHandler{}

func NewServerHandler(
	version string,
	globusClientManager *globusauth.ClientManager,
	scicatUrl string,
	scicatServiceUser serviceuser.ScicatServiceUser,
	facilities *map[string]Facility,
	taskPool tasks.TaskPool) (ServerHandler, error) {
	// create server with service client
	if globusClientManager == nil {
		return ServerHandler{}, fmt.Errorf("AUTH error: globus client manager is nil")
	}

	return ServerHandler{
		version:             version,
		globusClientManager: globusClientManager,
		scicatUrl:           scicatUrl,
		scicatServiceUser:   scicatServiceUser,
		facilities:          *facilities,
		taskPool:            taskPool,
		addTaskMutex:        &sync.Mutex{},
	}, nil
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
