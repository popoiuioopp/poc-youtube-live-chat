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

	e.GET("/auth", auth.HandleAuth)
	e.GET("/oauth2callback", auth.OAuthCallback)
	e.GET("/chat", chat.ReadChatMessages)

	cfg, err := auth.GetOAuthConfig()
	if err == nil {
		token, err := yt.LoadTokenFromFile(auth.TokenFilePath)
		if err == nil {
			// Token file exists, let's try to init
			err := yt.InitYoutubeClient(cfg, token, auth.TokenFilePath)
			if err != nil {
				fmt.Println("WARNING: token from file was invalid or something:", err)
			} else {
				fmt.Println("Successfully loaded existing token, no /auth route needed.")
			}
		}
	}

	// Start server
	if err := e.Start(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
