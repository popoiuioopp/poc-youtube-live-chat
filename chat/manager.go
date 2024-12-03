package chat

import (
	"fmt"
	"sync"
	"youtube-echo-service/models"

	"github.com/gorilla/websocket"
)

var (
	channelStates   = make(map[string]*models.ChannelState)
	channelStatesMu sync.Mutex
)

func startChannelMonitoring(channelID string) {
	if _, exists := channelStates[channelID]; exists {
		channelStatesMu.Unlock()
		return
	}

	state := &models.ChannelState{
		IsLive:      false,
		LiveChatID:  "",
		Messages:    make(chan *models.ChatMessage, 100),
		Subscribers: make(map[*websocket.Conn]struct{}),
		StopChan:    make(chan struct{}),
	}
	channelStates[channelID] = state

	go MonitorChannel(channelID, state)
	go BroadcastMessages(channelID, state)
	fmt.Println("START MONITORING CHANNELID:", channelID)
}

func addSubscriber(channelID string, ws *websocket.Conn) {
	channelStatesMu.Lock()
	state, exists := channelStates[channelID]
	if !exists {
		startChannelMonitoring(channelID)
		state = channelStates[channelID]
	}
	channelStatesMu.Unlock()

	state.SubscribersMu.Lock()
	state.Subscribers[ws] = struct{}{}
	state.SubscribersMu.Unlock()
}

func removeSubscriber(channelID string, ws *websocket.Conn) {
	channelStatesMu.Lock()
	state, exists := channelStates[channelID]
	if exists {
		state.SubscribersMu.Lock()
		delete(state.Subscribers, ws)
		subscribersLeft := len(state.Subscribers)
		state.SubscribersMu.Unlock()

		if subscribersLeft == 0 {
			close(state.StopChan)
			delete(channelStates, channelID)
		}
	}
	channelStatesMu.Unlock()
}
