package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
)

type UserData struct {
	Sub string `json:"sub"` // This is the user_id
}

func getUserIDFromToken(token string) (string, error) {
	req, err := http.NewRequest("GET", os.Getenv("supabaseProjectURL")+"/auth/v1/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", os.Getenv("supabaseAnonKey"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", errors.New("unauthorized: " + string(body))
	}

	var user UserData
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}
	return user.Sub, nil
}
