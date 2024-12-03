package chat

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all connections by default
		return true
	},
}

// ReadChatMessages serves as a WebSocket endpoint to stream messages based on channel ID
func ReadChatMessages(c echo.Context) error {
	channelID := c.QueryParam("channel_id")
	if channelID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing channel_id parameter"})
	}

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	// Add the subscriber
	addSubscriber(channelID, ws)

	// Keep the connection open until the client disconnects
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}

	// Remove the subscriber when done
	removeSubscriber(channelID, ws)

	return nil
}
