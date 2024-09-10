package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// New reranking-related structs
type RankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            *int     `json:"top_n,omitempty"`
	RankFields      []string `json:"rank_fields,omitempty"`
	ReturnDocuments *bool    `json:"return_documents,omitempty"`
	MaxChunksPerDoc *int     `json:"max_chunks_per_doc,omitempty"`
}

type Document struct {
	Text string                 `json:"text"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

type RankResponseItem struct {
	Document       *Document `json:"document,omitempty"`
	Index          int       `json:"index"`
	RelevanceScore float64   `json:"relevance_score"`
}

type RankResponseMeta struct {
	APIVersion  map[string]interface{} `json:"api_version"`
	BilledUnits map[string]int         `json:"billed_units"`
	Tokens      map[string]int         `json:"tokens"`
	Warnings    []string               `json:"warnings"`
}

type RankResponse struct {
	ID      string             `json:"id"`
	Results []RankResponseItem `json:"results"`
	Meta    RankResponseMeta   `json:"meta"`
}

type Passage struct {
	ID      int
	Content string
}

func (app *App) apeerRerank(query string, documents []Document) ([]int, error) {
	passages := make([]Passage, len(documents))
	for i, doc := range documents {
		passages[i] = Passage{ID: i, Content: doc.Text}
	}

	prompt := app.constructAPEERPrompt(query, passages)

	response, err := app.callGeminiAPI(context.Background(), "gemini-1.5-flash", prompt)
	if err != nil {
		return nil, fmt.Errorf("error calling LLM: %v", err)
	}

	rankedIDs, err := app.parseAPEERResponse(response)
	if err != nil {
		return nil, fmt.Errorf("error parsing LLM response: %v", err)
	}

	return rankedIDs, nil
}

func (app *App) constructAPEERPrompt(query string, passages []Passage) string {
	var sb strings.Builder

	sb.WriteString("You are RankGPT, an intelligent assistant that can rank passages based on their relevancy and accuracy to the query.\n\n")
	sb.WriteString(fmt.Sprintf("Query: [querystart] %s [queryend]\n\n", query))
	sb.WriteString("Rank the following passages based on their relevance and accuracy to the query. Prioritize passages that directly address the query and provide detailed, correct answers. Ignore factors such as length, complexity, or writing style unless they seriously hinder readability.\n\n")

	for _, p := range passages {
		sb.WriteString(fmt.Sprintf("[%d] %s\n\n", p.ID, p.Content))
	}

	sb.WriteString("Produce a succinct and clear ranking of all passages, from most to least relevant, using their identifiers. The format should be [rankstart] [most relevant passage ID] > [next most relevant passage ID] > ... > [least relevant passage ID] [rankend]. Refrain from including any additional commentary or explanations in your ranking.")

	return sb.String()
}

func (app *App) parseAPEERResponse(response string) ([]int, error) {
	start := strings.Index(response, "[rankstart]")
	end := strings.Index(response, "[rankend]")
	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("invalid response format")
	}

	rankingStr := response[start+11 : end]
	rankingStr = strings.TrimSpace(rankingStr)

	idStrs := strings.Split(rankingStr, ">")
	var rankedIDs []int

	for _, idStr := range idStrs {
		idStr = strings.TrimSpace(idStr)
		idStr = strings.Trim(idStr, "[]")
		id := 0
		_, err := fmt.Sscanf(idStr, "%d", &id)
		if err != nil {
			return nil, fmt.Errorf("error parsing ID: %v", err)
		}
		rankedIDs = append(rankedIDs, id)
	}

	return rankedIDs, nil
}

func (app *App) handleRerank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RankRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convert string documents to Document structs
	documents := make([]Document, len(req.Documents))
	for i, text := range req.Documents {
		documents[i] = Document{Text: text}
	}

	rankedIDs, err := app.apeerRerank(req.Query, documents)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error during reranking: %v", err), http.StatusInternalServerError)
		return
	}

	results := make([]RankResponseItem, len(rankedIDs))
	for i, id := range rankedIDs {
		item := RankResponseItem{
			Index:          id,
			RelevanceScore: float64(len(rankedIDs)-i) / float64(len(rankedIDs)), // Simple score calculation
		}
		if req.ReturnDocuments == nil || *req.ReturnDocuments {
			item.Document = &Document{Text: req.Documents[id]}
		}
		results[i] = item
	}

	if req.TopN != nil && *req.TopN < len(results) {
		results = results[:*req.TopN]
	}

	response := RankResponse{
		ID:      uuid.New().String(),
		Results: results,
		Meta: RankResponseMeta{
			APIVersion: map[string]interface{}{
				"version":         "1.0",
				"is_deprecated":   false,
				"is_experimental": false,
			},
			BilledUnits: map[string]int{
				"input_tokens":    0,
				"output_tokens":   0,
				"search_units":    0,
				"classifications": 0,
			},
			Tokens: map[string]int{
				"input_tokens":  0,
				"output_tokens": 0,
			},
			Warnings: []string{},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
