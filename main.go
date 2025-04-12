package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

type GenerateRequest struct {
	Prompt string `json:"prompt"`
	IP     string `json:"ip"`
}

type GenerateResponse struct {
	ImageBase64 string `json:"image_base64"`
	Error       string `json:"error,omitempty"`
}

func main() {
	mux := http.NewServeMux()

	// Your handlers
	mux.HandleFunc("/generate", AuthMiddleware(generateHandler))

	// Enable CORS for frontend
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}).Handler(mux)

	log.Println("Backend listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))

	// http.HandleFunc("/generate", handleGenerate)
	// log.Println("Server running on http://localhost:8080")
	// http.ListenAndServe(":8080", nil)
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	// 1. Get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}
	token := extractBearerToken(authHeader)

	// 2. Parse prompt + IP from JSON body
	var reqData GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 3. Get user_id from Supabase token
	userID, err := getUserIDFromToken(token)
	if err != nil {
		http.Error(w, "Invalid user token", http.StatusUnauthorized)
		return
	}

	// 4. Check + update usage limit
	if err := checkAndUpdateUsage(userID, reqData.IP); err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	// 5. Generate image
	imgBytes, err := generateImage(reqData.Prompt)
	if err != nil {
		log.Println("Image generation error:", err)
		resp := GenerateResponse{Error: "Image generation failed"}
		json.NewEncoder(w).Encode(resp)
		return
	}

	// 6. Return image as base64
	encoded := base64.StdEncoding.EncodeToString(imgBytes)
	resp := GenerateResponse{ImageBase64: encoded}
	json.NewEncoder(w).Encode(resp)
}

func extractBearerToken(header string) string {
	if len(header) > 7 && header[:7] == "Bearer " {
		return header[7:]
	}
	return header
}

func generateImage(prompt string) ([]byte, error) {
	requestBody, _ := json.Marshal(map[string]string{
		"inputs": prompt,
	})

	req, _ := http.NewRequest("POST", os.Getenv("hfURL"), bytes.NewBuffer(requestBody))
	req.Header.Add("Authorization", os.Getenv("BEARER_TOKEN"))
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Request error:", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Retry if response is not image (sometimes Hugging Face returns startup JSON)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "image/png" && contentType != "image/jpeg" {
		body, _ := io.ReadAll(resp.Body)
		log.Println("Non-image response:", string(body))
		return nil, err
	}

	return io.ReadAll(resp.Body)
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")

		// Optional: verify JWT with Supabase keys
		// For now we can skip verifying and just trust the token
		ctx := context.WithValue(r.Context(), "userToken", token)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func generateHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string `json:"prompt"`
		Style  string `json:"style"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	fullPrompt := fmt.Sprintf("In %s style: %s", req.Style, req.Prompt)

	imageData, err := generateImage(fullPrompt)
	if err != nil {
		http.Error(w, "Image generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(imageData)
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}
