package main

import (
	"log"
	"youtube-echo-service/services"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	e.GET("/auth", services.HandleAuth)          // Authentication endpoint
	e.POST("/register", services.RegisterStream) // Register a live stream
	e.GET("/chat", services.ReadChatMessages)    // Get live chat messages
	e.GET("/oauth2callback", services.OAuthCallback)

	// Start server
	if err := e.Start(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
