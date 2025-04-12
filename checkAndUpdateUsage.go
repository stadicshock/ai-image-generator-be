package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"
)

type UsageEntry struct {
	ID        string `json:"id,omitempty"`
	UserID    string `json:"user_id"`
	IPAddress string `json:"ip_address"`
	Date      string `json:"date"`
	Count     int    `json:"count"`
}

func checkAndUpdateUsage(userID, ip string) error {
	today := time.Now().Format("2006-01-02")

	// 1. Check existing usage
	req, _ := http.NewRequest("GET", os.Getenv("supabaseProjectURL")+"/rest/v1/image_usage?user_id=eq."+userID+"&date=eq."+today, nil)
	req.Header.Set("apikey", os.Getenv("supabaseAnonKey"))
	req.Header.Set("Authorization", "Bearer "+os.Getenv("supabaseAnonKey"))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var entries []UsageEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return err
	}

	if len(entries) > 0 {
		entry := entries[0]
		if entry.Count >= 5 {
			return errors.New("daily image generation limit reached")
		}

		// 2. Update count
		newCount := entry.Count + 1
		body, _ := json.Marshal(map[string]int{"count": newCount})
		patchReq, _ := http.NewRequest("PATCH", os.Getenv("supabaseDBURL")+"?id=eq."+entry.ID, bytes.NewBuffer(body))
		patchReq.Header.Set("apikey", os.Getenv("supabaseAnonKey"))
		patchReq.Header.Set("Authorization", "Bearer "+os.Getenv("supabaseAnonKey"))
		patchReq.Header.Set("Content-Type", "application/json")

		_, err := client.Do(patchReq)
		return err
	}

	// 3. No entry — insert new
	newEntry := UsageEntry{
		UserID:    userID,
		IPAddress: ip,
		Date:      today,
		Count:     1,
	}
	entryBody, _ := json.Marshal(newEntry)
	postReq, _ := http.NewRequest("POST", os.Getenv("supabaseDBURL"), bytes.NewBuffer(entryBody))
	postReq.Header.Set("apikey", os.Getenv("supabaseAnonKey"))
	postReq.Header.Set("Authorization", "Bearer "+os.Getenv("supabaseAnonKey"))
	postReq.Header.Set("Content-Type", "application/json")

	_, err = client.Do(postReq)
	return err
}
