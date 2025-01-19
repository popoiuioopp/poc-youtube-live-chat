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

type AuthService struct {
	config        *oauth2.Config
	tokenFilePath string
}

func NewAuthService(clientSecretPath, tokenFilePath, redirectURL string) (*AuthService, error) {
	b, err := os.ReadFile(clientSecretPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, youtube.YoutubeScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file: %v", err)
	}

	config.RedirectURL = redirectURL

	return &AuthService{
		config:        config,
		tokenFilePath: tokenFilePath,
	}, nil
}

func (s *AuthService) HandleAuth(c echo.Context) error {
	authURL := s.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	return c.Redirect(http.StatusFound, authURL)
}

func (s *AuthService) OAuthCallback(c echo.Context) error {
	code := c.QueryParam("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing authorization code"})
	}

	token, err := s.config.Exchange(context.Background(), code)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to exchange token"})
	}

	if err := yt.SaveTokenToFile(token, s.tokenFilePath); err != nil {
		fmt.Println("WARNING: failed to save token to file:", err)
	}

	if err := yt.InitYoutubeClient(s.config, token, s.tokenFilePath); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Authorization successful"})
}

func (s *AuthService) GetConfig() *oauth2.Config {
	return s.config
}

func (s *AuthService) TokenFilePath() string {
	return s.tokenFilePath
}
