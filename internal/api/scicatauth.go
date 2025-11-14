package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/SwissOpenEM/scicat-globus-proxy/internal/scicat"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gin-gonic/gin"
	ginmiddleware "github.com/oapi-codegen/gin-middleware"
)

// HTTP header to specify SciCat API key
const SCICAT_AUTH_HEADER = "SciCat-API-Key"

func ScicatTokenAuthMiddleware(scicatUrl string) openapi3filter.AuthenticationFunc {
	return func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
		// Get scicat API key from the header
		ginCtx, ok := ctx.Value(ginmiddleware.GinContextKey).(*gin.Context)
		if !ok {
			return errors.New("server error, can't get gin context")
		}
		scicatApiKey := ginCtx.Request.Header.Get(SCICAT_AUTH_HEADER)
		if scicatApiKey == "" {
			return fmt.Errorf("SciCat authentication is required. Specify a SciCat token in the '%s' header", SCICAT_AUTH_HEADER)
		}

		scicat := scicat.ScicatService{
			Url:   scicatUrl,
			Token: scicatApiKey,
		}

		user, err := scicat.GetUserIdentity()
		if err != nil {
			return err
		}

		ginCtx.Set("scicatUser", user)
		return nil
	}
}
