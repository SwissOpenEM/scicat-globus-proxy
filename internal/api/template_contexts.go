// Collect type-safe template types
package api

import (
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/util"
)

// Context for sourcePath and destinationPath
type facilityPathContext struct {
	DatasetFolder        string
	SourceFolder         string
	RelativeSourceFolder string
	Pid                  string
	PidShort             string
	PidPrefix            string
	PidEncoded           string
	Username             string
}

type facilityPathTemplate = util.TypedTemplate[facilityPathContext]

// Context for accessPath and accessValue
type accessPathContext struct {
	Name string
}

type accessPathTemplate = util.TypedTemplate[accessPathContext]

type scopeContext struct {
	Name string
}

type scopeTemplate = util.TypedTemplate[scopeContext]
