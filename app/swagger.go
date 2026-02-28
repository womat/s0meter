//go:build swagger

package app

import (
	"net/http"

	_ "s0meter/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

func (app *App) registerSwaggerRoute(mux *http.ServeMux) {
	mux.Handle(PathSwagger, NewRouteMap(map[string]http.Handler{
		"GET": httpSwagger.Handler(httpSwagger.PersistAuthorization(true)),
	}))
}
