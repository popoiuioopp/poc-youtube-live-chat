package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"

	yt "youtube-echo-service/youtube"
)

var TokenFilePath = "token/youtubeToken.json"

// HandleAuth initializes OAuth flow
func HandleAuth(c echo.Context) error {
	config, err := GetOAuthConfig()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot get oauth config"})
	}

	// Generate the OAuth2 URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	// Redirect the user to the authorization URL
	return c.Redirect(http.StatusFound, authURL)
}

func OAuthCallback(c echo.Context) error {
	config, err := GetOAuthConfig()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot get oauth config"})
	}

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

	if err := yt.SaveTokenToFile(token, TokenFilePath); err != nil {
		fmt.Println("WARNING: failed to save token to file:", err)
	}

	// Initialize YouTube client with the new token
	if err := yt.InitYoutubeClient(config, token, TokenFilePath); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Authorization successful"})
}

func GetOAuthConfig() (*oauth2.Config, error) {
	// Read the client secrets
	b, err := os.ReadFile("client_secret.json")
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	// Set up OAuth2 configuration
	config, err := google.ConfigFromJSON(b, youtube.YoutubeScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file: %v", err)
	}

	// Set redirect URI to handle the authorization code
	config.RedirectURL = "http://localhost:8080/oauth2callback"

	return config, nil
}
