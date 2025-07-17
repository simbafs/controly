package infrastructure_test

import (
	"github.com/simbafs/controly/server/internal/application"
	"github.com/simbafs/controly/server/internal/infrastructure"
)

var (
	_ application.DisplayRepository    = (*infrastructure.InMemoryDisplayRepository)(nil)
	_ application.ControllerRepository = (*infrastructure.InMemoryControllerRepository)(nil)
)
