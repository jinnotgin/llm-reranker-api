package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

type App struct {
	ProjectID string
	Location  string
}

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	app := &App{
		ProjectID: os.Getenv("PROJECT_ID"),
		Location:  os.Getenv("LOCATION"),
	}

	// Example of using the LLM
	prompt := "Tell me a short joke"
	response, err := app.callGeminiAPI(context.Background(), "gemini-1.5-flash", prompt)
	if err != nil {
		log.Fatalf("Error calling Gemini API: %v", err)
	}
	fmt.Println("Gemini response:", response)

	// Set up HTTP server for reranking
	http.HandleFunc("/v1/rerank", app.handleRerank)
	http.HandleFunc("/rerank", app.handleRerank)

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}

	log.Printf("Starting server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
