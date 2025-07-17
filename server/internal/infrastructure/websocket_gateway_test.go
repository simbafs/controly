package infrastructure_test

import (
	"github.com/simbafs/controly/server/internal/application"
	"github.com/simbafs/controly/server/internal/infrastructure"
)

var _ application.WebSocketConnectionManager = (*infrastructure.GorillaWebSocketGateway)(nil)

var _ application.WebSocketMessenger = (*infrastructure.GorillaWebSocketGateway)(nil)
