package api

import (
	"context"
)

func (s ServerHandler) GetVersion(ctx context.Context, request GetVersionRequestObject) (GetVersionResponseObject, error) {
	return GetVersion200JSONResponse{
		Version: s.version,
	}, nil
}
