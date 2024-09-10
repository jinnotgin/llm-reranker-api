package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// ErrorResponse represents the structure of our error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// LoggingMiddleware wraps an http.HandlerFunc and logs error requests with details
func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Read and store the request body
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Create a custom ResponseWriter to capture the status code and response
		rw := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
			body:           &bytes.Buffer{},
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		// Only log if there's an error (status code >= 400)
		if rw.status >= 400 {
			// Parse GET parameters
			if err := r.ParseForm(); err != nil {
				log.Printf("Error parsing form: %v", err)
			}

			// Create a map to store request details
			requestDetails := make(map[string]interface{})
			requestDetails["method"] = r.Method
			requestDetails["path"] = r.URL.Path
			requestDetails["status"] = rw.status
			requestDetails["duration"] = duration.String()
			requestDetails["query_params"] = r.URL.Query()

			// Add request body for non-GET requests
			if r.Method != http.MethodGet {
				requestDetails["body"] = string(bodyBytes)
			}

			// Try to parse the error response
			var errorResp ErrorResponse
			if err := json.Unmarshal(rw.body.Bytes(), &errorResp); err == nil {
				requestDetails["error_details"] = errorResp.Error
			} else {
				requestDetails["response"] = rw.body.String()
			}

			// Convert requestDetails to JSON for logging
			detailsJSON, _ := json.Marshal(requestDetails)

			log.Printf("ERROR: %s", string(detailsJSON))
		}
	}
}

// Custom ResponseWriter to capture status code and response body
type responseWriter struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
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
