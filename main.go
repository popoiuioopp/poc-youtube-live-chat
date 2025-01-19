package main

import (
	"fmt"
	"log"
	"youtube-echo-service/auth"
	"youtube-echo-service/chat"

	yt "youtube-echo-service/youtube"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	authService, err := auth.NewAuthService("client_secret.json", "token/youtubeToken.json", "http://localhost:8080/oauth2callback")
	if err != nil {
		log.Fatalf("Failed to create AuthService: %v", err)
	}

	e.GET("/auth", authService.HandleAuth)
	e.GET("/oauth2callback", authService.OAuthCallback)
	e.GET("/chat", chat.ReadChatMessages)

	token, err := yt.LoadTokenFromFile(authService.TokenFilePath())
	if err == nil {
		cfg := authService.GetConfig()
		err := yt.InitYoutubeClient(cfg, token, authService.TokenFilePath())
		if err != nil {
			fmt.Println("WARNING: token from file was invalid or something:", err)
		} else {
			fmt.Println("Successfully loaded existing token, no /auth route needed.")
		}
	}

	// Start server
	if err := e.Start(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
