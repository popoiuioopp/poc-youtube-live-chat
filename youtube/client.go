package youtube

import (
	"context"
	"fmt"
	"io/ioutil"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var YouTubeService *youtube.Service

// Initialize YouTube client
func InitYouTubeClient(token *oauth2.Token) error {
	ctx := context.Background()

	// Read client secrets
	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		return fmt.Errorf("unable to read client secret file: %v", err)
	}

	// Parse the client secret
	config, err := google.ConfigFromJSON(b, youtube.YoutubeScope)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file: %v", err)
	}

	// Create the client with the token
	client := config.Client(ctx, token)

	// Initialize the YouTube service
	YouTubeService, err = youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("error creating youTube client: %v", err)
	}

	return nil
}
