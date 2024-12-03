package main

import (
	"log"
	"youtube-echo-service/services"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	e.GET("/auth", services.HandleAuth)
	e.GET("/oauth2callback", services.OAuthCallback)
	e.GET("/chat", services.ReadChatMessages)

	// Start server
	if err := e.Start(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
