package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type App struct {
	ProjectID string
	Location  string
}

// LoggingMiddleware wraps an http.HandlerFunc and logs error requests with details
func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom ResponseWriter to capture the status code
		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)

		// Only log if there's an error (status code >= 400)
		if rw.status >= 400 {
			duration := time.Since(start)

			// Parse GET parameters
			if err := r.ParseForm(); err != nil {
				log.Printf("Error parsing form: %v", err)
			}

			// Create a map to store parameters
			params := make(map[string]interface{})

			// Add GET parameters
			for k, v := range r.Form {
				params[k] = v
			}

			// Add POST parameters for non-GET requests
			if r.Method != http.MethodGet {
				var postParams map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&postParams); err == nil {
					for k, v := range postParams {
						params[k] = v
					}
				}
			}

			// Convert params to JSON for logging
			paramsJSON, _ := json.Marshal(params)

			log.Printf(
				"ERROR: method=%s path=%s status=%d duration=%s params=%s",
				r.Method,
				r.URL.Path,
				rw.status,
				duration,
				string(paramsJSON),
			)
		}
	}
}

// Custom ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
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

	// Set up HTTP server for reranking with logging middleware
	http.HandleFunc("/v1/rerank", LoggingMiddleware(app.handleRerank))
	http.HandleFunc("/rerank", LoggingMiddleware(app.handleRerank))

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}

	log.Printf("Starting server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
