package app

import (
	"log/slog"

	"gateway-api/config"
	"gateway-api/internal/middleware"
)

type gatewayDependencies struct {
	config         config.Config
	logger         *slog.Logger
	requestLogSink middleware.RequestLogSink
	cleanup        []func()
}

func (d *gatewayDependencies) close() {
	for i := len(d.cleanup) - 1; i >= 0; i-- {
		if d.cleanup[i] != nil {
			d.cleanup[i]()
		}
	}
}
