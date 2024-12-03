package main

import (
	"log"
	"youtube-echo-service/auth"
	"youtube-echo-service/chat"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	e.GET("/auth", auth.HandleAuth)
	e.GET("/oauth2callback", auth.OAuthCallback)
	e.GET("/chat", chat.ReadChatMessages)

	// Start server
	if err := e.Start(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
