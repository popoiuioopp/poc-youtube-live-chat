package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
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
	youtubeService *youtube.Service
	liveChatMap    sync.Map
)

type ChatMessage struct {
	MessageID   string    `json:"message_id"`
	DisplayName string    `json:"display_name"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

type LiveChat struct {
	ChatID        string
	Messages      chan *ChatMessage
	StopChan      chan struct{}
	Subscribers   map[*websocket.Conn]struct{}
	SubscribersMu sync.Mutex
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

// RegisterStream registers a live stream and fetches its chat ID
func RegisterStream(c echo.Context) error {
	streamURL := c.FormValue("url")
	chatID, err := fetchLiveChatID(streamURL)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	liveChat := &LiveChat{
		ChatID:      chatID,
		Messages:    make(chan *ChatMessage, 100), // Buffered channel
		StopChan:    make(chan struct{}),
		Subscribers: make(map[*websocket.Conn]struct{}),
	}

	// Start polling messages
	go pollLiveChatMessages(liveChat)

	// Start broadcasting messages to subscribers
	go broadcastMessages(liveChat)

	// Store liveChat in the map
	liveChatMap.Store(chatID, liveChat)

	return c.JSON(http.StatusOK, map[string]string{"chat_id": chatID, "message": "Stream registered successfully"})
}

func broadcastMessages(liveChat *LiveChat) {
	for message := range liveChat.Messages {
		liveChat.SubscribersMu.Lock()
		for ws := range liveChat.Subscribers {
			err := ws.WriteJSON(message)
			if err != nil {
				fmt.Printf("Error sending message to subscriber: %v\n", err)
				ws.Close()
				delete(liveChat.Subscribers, ws)
			}
		}
		liveChat.SubscribersMu.Unlock()
	}
}

func pollLiveChatMessages(liveChat *LiveChat) {
	defer close(liveChat.Messages)
	var nextPageToken string

	for {
		select {
		case <-liveChat.StopChan:
			fmt.Printf("Stopping polling for chat ID: %s\n", liveChat.ChatID)
			return
		default:
			call := youtubeService.LiveChatMessages.List(liveChat.ChatID, []string{"snippet", "authorDetails"})
			if nextPageToken != "" {
				call = call.PageToken(nextPageToken)
			}
			response, err := call.Do()
			if err != nil {
				fmt.Printf("Error fetching live chat messages: %v\n", err)
				time.Sleep(5 * time.Second) // Wait before retrying
				continue
			}

			for _, item := range response.Items {

				publishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
				if err != nil {
					fmt.Printf("Error parsing time: %v\n", err)
					// You can choose to skip this message or set a default time
					publishedAt = time.Now() // Or handle it as per your requirement
				}

				// Convert item to ChatMessage
				message := &ChatMessage{
					MessageID:   item.Id,
					DisplayName: item.AuthorDetails.DisplayName,
					Message:     item.Snippet.DisplayMessage,
					Timestamp:   publishedAt,
				}
				liveChat.Messages <- message
			}
			nextPageToken = response.NextPageToken
			time.Sleep(time.Duration(response.PollingIntervalMillis) * time.Millisecond)
		}
	}
}

// ReadChatMessages fetches live chat messages
func ReadChatMessages(c echo.Context) error {
	chatID := c.QueryParam("chat_id")
	if chatID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing chat_id parameter"})
	}

	value, ok := liveChatMap.Load(chatID)
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Chat ID not found"})
	}
	liveChat := value.(*LiveChat)

	// Upgrade the HTTP connection to a WebSocket connection
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	// Add the WebSocket connection to the subscribers
	liveChat.SubscribersMu.Lock()
	liveChat.Subscribers[ws] = struct{}{}
	liveChat.SubscribersMu.Unlock()

	// Listen for close messages from the client
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}

	// Remove the subscriber when done
	liveChat.SubscribersMu.Lock()
	delete(liveChat.Subscribers, ws)
	liveChat.SubscribersMu.Unlock()

	return nil
}

func UnregisterStream(c echo.Context) error {
	chatID := c.FormValue("chat_id")

	value, ok := liveChatMap.Load(chatID)
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Chat ID not found"})
	}

	liveChat := value.(*LiveChat)
	close(liveChat.StopChan)
	liveChatMap.Delete(chatID)

	return c.JSON(http.StatusOK, map[string]string{"message": "Stream unregistered successfully"})
}

// fetchLiveChatID retrieves the chat ID of a live stream
func fetchLiveChatID(streamURL string) (string, error) {
	if youtubeService == nil {
		return "", fmt.Errorf("YouTube service not initialized")
	}

	// Extract the video ID from the URL
	videoID, err := extractVideoID(streamURL)
	if err != nil {
		return "", fmt.Errorf("Failed to extract video ID: %v", err)
	}

	// Retrieve the video details
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

func extractVideoID(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	queryParams := parsedURL.Query()
	if v, ok := queryParams["v"]; ok && len(v) > 0 {
		return v[0], nil
	}

	// Handle short URLs like youtu.be/<videoID>
	if parsedURL.Host == "youtu.be" {
		return strings.TrimLeft(parsedURL.Path, "/"), nil
	}

	return "", fmt.Errorf("Could not extract video ID from URL")
}

// fetchLiveChatMessages retrieves live chat messages from a chat ID
func fetchLiveChatMessages(chatID string) ([]string, error) {
	if youtubeService == nil {
		return nil, fmt.Errorf("YouTube service not initialized")
	}

	call := youtubeService.LiveChatMessages.List(chatID, []string{"snippet", "authorDetails"})
	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("Error fetching live chat messages: %v", err)
	}

	messages := []string{}
	for _, item := range response.Items {
		messages = append(messages, item.Snippet.DisplayMessage)
	}

	return messages, nil
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
