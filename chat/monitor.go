package chat

import (
	"fmt"
	"time"
	"youtube-echo-service/models"
	yt "youtube-echo-service/youtube"
)

func MonitorChannel(channelID string, state *models.ChannelState) {
	for {
		select {
		case <-state.StopChan:
			fmt.Printf("Stopping monitoring for channel ID: %s\n", channelID)
			return
		default:
			liveVideoID, err := yt.FetchLiveVideoID(channelID)
			if err != nil {
				fmt.Printf("Error fetching live video ID: %v\n", err)
				time.Sleep(30 * time.Second)
				continue
			}

			if liveVideoID != "" {
				if !state.IsLive {
					state.IsLive = true
					liveChatID, err := yt.FetchLiveChatIDByVideoID(liveVideoID)
					if err != nil {
						fmt.Printf("Error fetching live chat ID: %v\n", err)
						time.Sleep(30 * time.Second)
						continue
					}
					state.LiveChatID = liveChatID
					go PollLiveChatMessages(channelID, state)
				}
			} else {
				if state.IsLive {
					state.IsLive = false
					state.StopChan <- struct{}{} // Stop polling live chat messages
					close(state.Messages)
					state.Messages = make(chan *models.ChatMessage, 100)
				}
			}

			time.Sleep(30 * time.Second)
		}
	}
}

func PollLiveChatMessages(channelID string, state *models.ChannelState) {
	fmt.Printf("Starting to poll live chat messages for channel ID: %s\n", channelID)
	var nextPageToken string

	for {
		select {
		case <-state.StopChan:
			fmt.Printf("Stopping live chat polling for channel ID: %s\n", channelID)
			return
		default:
			call := yt.YouTubeService.LiveChatMessages.List(state.LiveChatID, []string{"snippet", "authorDetails"})
			if nextPageToken != "" {
				call = call.PageToken(nextPageToken)
			}
			response, err := call.Do()
			if err != nil {
				fmt.Printf("Error fetching live chat messages: %v\n", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for _, item := range response.Items {
				publishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
				if err != nil {
					fmt.Printf("Error parsing time: %v\n", err)
					publishedAt = time.Now()
				}

				message := &models.ChatMessage{
					MessageID:   item.Id,
					DisplayName: item.AuthorDetails.DisplayName,
					Message:     item.Snippet.DisplayMessage,
					Timestamp:   publishedAt,
				}
				state.Messages <- message
			}
			nextPageToken = response.NextPageToken
			time.Sleep(time.Duration(response.PollingIntervalMillis) * time.Millisecond)
		}
	}
}

func BroadcastMessages(channelID string, state *models.ChannelState) {
	for message := range state.Messages {
		state.SubscribersMu.Lock()
		for ws := range state.Subscribers {
			err := ws.WriteJSON(message)
			if err != nil {
				fmt.Printf("Error sending message to subscriber: %v\n", err)
				ws.Close()
				delete(state.Subscribers, ws)
			}
		}
		state.SubscribersMu.Unlock()
	}
}
