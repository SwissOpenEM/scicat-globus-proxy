package api

import (
	"embed"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gin-contrib/slog"
	gin "github.com/gin-gonic/gin"
	middleware "github.com/oapi-codegen/gin-middleware"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

//go:embed openapi.yaml
var swaggerYAML embed.FS

func NewServer(api *ServerHandler, port uint, scicatUrl string) (*http.Server, error) {
	swagger, err := GetSwagger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading swagger spec\n: %s", err)
		os.Exit(1)
	}

	// Clear out the servers array in the swagger spec, that skips validating
	// that server names match.
	swagger.Servers = nil

	// Create gin router
	r := gin.New()

	r.Use(
		slog.SetLogger(
			slog.WithSkipPath([]string{"/version", "/health"}),
			slog.WithRequestHeader(false),
		))

	r.Use(gin.Recovery())

	r.GET("/openapi.yaml", func(c *gin.Context) {
		http.FileServer(http.FS(swaggerYAML)).ServeHTTP(c.Writer, c.Request)
	})

	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerfiles.Handler, ginSwagger.URL("/openapi.yaml")))

	r.Use(
		middleware.OapiRequestValidatorWithOptions(swagger, &middleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: ScicatTokenAuthMiddleware(scicatUrl),
			},
		}),
	)

	RegisterHandlers(r, NewStrictHandler(api, []StrictMiddlewareFunc{}))

	return &http.Server{
		Handler: r,
		Addr:    net.JoinHostPort("0.0.0.0", fmt.Sprint(port)),
	}, nil
}
