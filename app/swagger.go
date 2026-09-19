//go:build swagger

package app

import (
	"net/http"

	"github.com/womat/s0meter/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

const PathSwagger = "/swagger/"

func (app *App) registerSwaggerRoute(mux *http.ServeMux) {
	// The generated spec carries no version of its own; fill it from the build-time
	// value so the UI reports the same version as /version and --about.
	docs.SwaggerInfo.Version = VERSION

	mux.Handle("GET "+PathSwagger, httpSwagger.Handler(
		httpSwagger.PersistAuthorization(true),
	))
}
