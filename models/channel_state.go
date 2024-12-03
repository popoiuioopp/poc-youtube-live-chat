package models

import (
	"sync"

	"github.com/gorilla/websocket"
)

type ChannelState struct {
	IsLive        bool
	LiveChatID    string
	Messages      chan *ChatMessage
	Subscribers   map[*websocket.Conn]struct{}
	SubscribersMu sync.Mutex
	StopChan      chan struct{}
}
