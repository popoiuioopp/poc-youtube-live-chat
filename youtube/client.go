package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var YouTubeService *youtube.Service

type savingTokenSource struct {
	baseTS       oauth2.TokenSource
	currentToken *oauth2.Token
	savePath     string
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	newToken, err := s.baseTS.Token()
	if err != nil {
		return nil, err
	}
	// Compare with our currently known token to see if changed
	if s.tokenChanged(s.currentToken, newToken) {
		s.currentToken = newToken
		// Re-save the token
		err := SaveTokenToFile(newToken, s.savePath)
		if err != nil {
			fmt.Printf("WARNING: could not save refreshed token: %v\n", err)
		} else {
			fmt.Println("Refreshed token saved to", s.savePath)
		}
	}
	return newToken, nil
}

func (s *savingTokenSource) tokenChanged(old, new *oauth2.Token) bool {
	if old == nil || new == nil {
		return true
	}
	if old.AccessToken != new.AccessToken || old.RefreshToken != new.RefreshToken {
		return true
	}
	return false
}

func InitYoutubeClient(cfg *oauth2.Config, initialToken *oauth2.Token, tokenFilePath string) error {
	base := cfg.TokenSource(context.Background(), initialToken)

	reusableTS := oauth2.ReuseTokenSource(initialToken, base)

	savingTS := &savingTokenSource{
		baseTS:       reusableTS,
		currentToken: initialToken,
		savePath:     tokenFilePath,
	}

	httpClient := oauth2.NewClient(context.Background(), savingTS)

	srv, err := youtube.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return fmt.Errorf("failed to create YouTube service: %v", err)
	}

	YouTubeService = srv
	return nil
}

func SaveTokenToFile(token *oauth2.Token, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create token file: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("unable to encode token to file: %v", err)
	}

	return nil
}

func LoadTokenFromFile(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var token oauth2.Token
	if err = json.NewDecoder(f).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}
