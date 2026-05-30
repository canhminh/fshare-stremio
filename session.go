package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var (
	sessionCache = make(map[string]CacheItem)
	cacheMutex   sync.RWMutex // 🟢 Thread-safe concurrency wrapper
)

type LoginResponse struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
}

func getActiveSession(creds Credentials, forceRefresh bool) (SessionData, error) {
	if creds.Email == "" || creds.Password == "" {
		return SessionData{}, errors.New("addon configuration error: email or password missing")
	}

	cacheKey := base64.StdEncoding.EncodeToString([]byte(creds.Email))
	if len(cacheKey) > 15 {
		cacheKey = cacheKey[:15]
	}

	if !forceRefresh {
		cacheMutex.RLock()
		item, exists := sessionCache[cacheKey]
		cacheMutex.RUnlock()
		if exists && time.Now().Unix() < item.ExpiresAt {
			return item.Data, nil
		}
	}

	// Step A: Fetch Web Portal Authorization profile bounds
	webData, err := executeFshareLogin(creds.Email, creds.Password, WebAppKey)
	if err != nil {
		return SessionData{}, fmt.Errorf("web portal login rejected: %w", err)
	}

	// Step B: Fetch API Downloader Authorization profile bounds
	apiData, err := executeFshareLogin(creds.Email, creds.Password, PyloadAppKey)
	if err != nil {
		return SessionData{}, fmt.Errorf("stream engine API login rejected: %w", err)
	}

	session := SessionData{
		WebToken:     webData.Token,
		WebSessionID: webData.SessionID,
		APIToken:     apiData.Token,
		APISessionID: apiData.SessionID,
	}

	cacheMutex.Lock()
	sessionCache[cacheKey] = CacheItem{
		Data:      session,
		ExpiresAt: time.Now().Unix() + (50 * 60), // Locked cache duration bounds (50 Mins)
	}
	cacheMutex.Unlock()

	return session, nil
}

func executeFshareLogin(email, password, appKey string) (LoginResponse, error) {
	payload, _ := json.Marshal(map[string]string{
		"user_email": email,
		"password":   password,
		"app_key":    appKey,
	})

	req, _ := http.NewRequest("POST", "https://api.fshare.vn/api/user/login", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", APIUserAgent)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return LoginResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return LoginResponse{}, fmt.Errorf("fshare returned network failure status: %d", resp.StatusCode)
	}

	var res LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return LoginResponse{}, err
	}

	if res.Token == "" || res.SessionID == "" {
		return LoginResponse{}, errors.New("auth tracking parameters returned blank")
	}

	return res, nil
}
