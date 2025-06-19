package api

import (
	otel "junjo-server/api/otel"

	"github.com/labstack/echo/v4"
)

func InitRoutes(e *echo.Echo) {
	e.GET("/otel/span-service-names", otel.GetDistinctServiceNames)
	e.GET("/workflow-spans-e2e/:serviceName", otel.GetWorkflowE2E)
	e.GET("/otel/node-exceptions", otel.GetNodeExceptions)
}
