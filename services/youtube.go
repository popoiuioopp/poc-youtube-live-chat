package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var (
	youtubeService  *youtube.Service
	channelStates   = make(map[string]*ChannelState)
	channelStatesMu sync.Mutex
)

type ChatMessage struct {
	MessageID   string    `json:"message_id"`
	DisplayName string    `json:"display_name"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

type ChannelState struct {
	IsLive        bool
	LiveChatID    string
	Messages      chan *ChatMessage
	Subscribers   map[*websocket.Conn]struct{}
	SubscribersMu sync.Mutex
	StopChan      chan struct{}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all connections by default
		return true
	},
}

// Initialize YouTube client
func initYouTubeClient(token *oauth2.Token) error {
	ctx := context.Background()

	// Read client secrets
	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		return fmt.Errorf("Unable to read client secret file: %v", err)
	}

	// Parse the client secret
	config, err := google.ConfigFromJSON(b, youtube.YoutubeScope)
	if err != nil {
		return fmt.Errorf("Unable to parse client secret file: %v", err)
	}

	// Create the client with the token
	client := config.Client(ctx, token)

	// Initialize the YouTube service
	youtubeService, err = youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("Error creating YouTube client: %v", err)
	}

	return nil
}

// HandleAuth initializes OAuth flow
func HandleAuth(c echo.Context) error {
	// Read the client secrets
	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Unable to read client secret file"})
	}

	// Set up OAuth2 configuration
	config, err := google.ConfigFromJSON(b, youtube.YoutubeScope)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Unable to parse client secret file"})
	}

	// Set redirect URI to handle the authorization code
	config.RedirectURL = "http://localhost:8080/oauth2callback"

	// Generate the OAuth2 URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	// Redirect the user to the authorization URL
	return c.Redirect(http.StatusFound, authURL)
}

func OAuthCallback(c echo.Context) error {
	// Read the client secrets
	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Unable to read client secret file"})
	}

	// Set up OAuth2 configuration
	config, err := google.ConfigFromJSON(b, youtube.YoutubeScope)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Unable to parse client secret file"})
	}

	// Ensure the redirect URL matches
	config.RedirectURL = "http://localhost:8080/oauth2callback"

	// Retrieve the authorization code from the query parameter
	code := c.QueryParam("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing authorization code"})
	}

	// Exchange the authorization code for a token
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to exchange token"})
	}

	// Initialize YouTube client with the new token
	if err := initYouTubeClient(token); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Authorization successful"})
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

func startChannelMonitoring(channelID string) {
	if _, exists := channelStates[channelID]; exists {
		channelStatesMu.Unlock()
		return
	}

	state := &ChannelState{
		IsLive:      false,
		LiveChatID:  "",
		Messages:    make(chan *ChatMessage, 100),
		Subscribers: make(map[*websocket.Conn]struct{}),
		StopChan:    make(chan struct{}),
	}
	channelStates[channelID] = state

	go monitorChannel(channelID, state)
	go broadcastMessages(channelID, state)
	fmt.Println("START MONITORING CHANNELID:", channelID)
}

func monitorChannel(channelID string, state *ChannelState) {
	for {
		select {
		case <-state.StopChan:
			fmt.Printf("Stopping monitoring for channel ID: %s\n", channelID)
			return
		default:
			liveVideoID, err := fetchLiveVideoID(channelID)
			if err != nil {
				fmt.Printf("Error fetching live video ID: %v\n", err)
				time.Sleep(30 * time.Second)
				continue
			}

			if liveVideoID != "" {
				if !state.IsLive {
					state.IsLive = true
					liveChatID, err := fetchLiveChatIDByVideoID(liveVideoID)
					if err != nil {
						fmt.Printf("Error fetching live chat ID: %v\n", err)
						time.Sleep(30 * time.Second)
						continue
					}
					state.LiveChatID = liveChatID
					go pollLiveChatMessages(channelID, state)
				}
			} else {
				if state.IsLive {
					state.IsLive = false
					state.StopChan <- struct{}{} // Stop polling live chat messages
					close(state.Messages)
					state.Messages = make(chan *ChatMessage, 100)
				}
			}

			time.Sleep(30 * time.Second)
		}
	}
}

func pollLiveChatMessages(channelID string, state *ChannelState) {
	fmt.Printf("Starting to poll live chat messages for channel ID: %s\n", channelID)
	var nextPageToken string

	for {
		select {
		case <-state.StopChan:
			fmt.Printf("Stopping live chat polling for channel ID: %s\n", channelID)
			return
		default:
			call := youtubeService.LiveChatMessages.List(state.LiveChatID, []string{"snippet", "authorDetails"})
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

				message := &ChatMessage{
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

func broadcastMessages(channelID string, state *ChannelState) {
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

// fetchLiveVideoID fetches the live video ID from a given channel ID
func fetchLiveVideoID(channelID string) (string, error) {
	if youtubeService == nil {
		return "", fmt.Errorf("YouTube service not initialized")
	}

	call := youtubeService.Search.List([]string{"id"}).
		ChannelId(channelID).
		EventType("live").
		Type("video").
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("Error fetching live video ID: %v", err)
	}

	if len(response.Items) == 0 {
		return "", nil // No live video found
	}

	videoID := response.Items[0].Id.VideoId
	fmt.Printf("FOUND live stream for %s : %s", channelID, videoID)
	return videoID, nil
}

// fetchLiveChatIDByVideoID retrieves the chat ID of a live stream given a video ID
func fetchLiveChatIDByVideoID(videoID string) (string, error) {
	if youtubeService == nil {
		return "", fmt.Errorf("YouTube service not initialized")
	}

	call := youtubeService.Videos.List([]string{"liveStreamingDetails"}).Id(videoID)
	response, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("Error retrieving video details: %v", err)
	}

	if len(response.Items) == 0 {
		return "", fmt.Errorf("No video found with ID: %s", videoID)
	}

	liveStreamingDetails := response.Items[0].LiveStreamingDetails
	if liveStreamingDetails == nil || liveStreamingDetails.ActiveLiveChatId == "" {
		return "", fmt.Errorf("Live chat not available for this video")
	}

	return liveStreamingDetails.ActiveLiveChatId, nil
}
