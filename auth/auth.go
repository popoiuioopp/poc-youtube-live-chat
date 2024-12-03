package auth

import (
	"context"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"

	yt "youtube-echo-service/youtube"
)

// HandleAuth initializes OAuth flow
func HandleAuth(c echo.Context) error {
	// Read the client secrets
	b, err := os.ReadFile("client_secret.json")
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
	b, err := os.ReadFile("client_secret.json")
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
	if err := yt.InitYouTubeClient(token); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Authorization successful"})
}
