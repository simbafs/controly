
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/simbafs/controly/server/internal/hub"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebsocketAPI struct {
	hub *hub.Hub
}

func NewWebsocketAPI(hub *hub.Hub) *WebsocketAPI {
	return &WebsocketAPI{
		hub: hub,
	}
}

func (ws *WebsocketAPI) Setup(r *gin.Engine) {
	r.GET("/api/client/register", ws.RegisterClient)
	r.GET("/api/admin/ws", ws.RegisterAdmin)
}

func (ws *WebsocketAPI) RegisterClient(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	ws.hub.RegisterClient(conn)

	defer func() {
		ws.hub.UnregisterClient(conn)
		conn.Close()
	}()

	for {
		// TODO: Implement client websocket message handling (read/write messages, interact with hub)
		panic("not implemented")
	}
}

func (ws *WebsocketAPI) RegisterAdmin(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	ws.hub.RegisterAdmin(conn)

	defer func() {
		ws.hub.UnregisterAdmin(conn)
		conn.Close()
	}()

	for {
		// TODO: Implement admin websocket message handling (read/write messages, interact with hub)
		panic("not implemented")
	}
}
