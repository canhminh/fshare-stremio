package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

//go:embed index.html
var htmlWizardPage string

func masterHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := r.URL.Path
	parts := []string{}
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}

	var creds Credentials
	normalizedPath := path

	if len(parts) > 0 && parts[0] != "manifest.json" && !strings.HasPrefix(parts[0], "catalog") && !strings.HasPrefix(parts[0], "meta") && !strings.HasPrefix(parts[0], "stream") {
		decoded, err := base64.StdEncoding.DecodeString(parts[0])
		if err == nil {
			if json.Unmarshal(decoded, &creds) == nil {
				normalizedPath = "/" + strings.Join(parts[1:], "/")
			}
		}
	}

	normalizedPath = decodeURIComponent(normalizedPath)

	if normalizedPath == "/" || normalizedPath == "" {
		serveHTMLWizard(w, r)
		return
	}
	if normalizedPath == "/manifest.json" {
		serveManifest(w, r)
		return
	}
	if normalizedPath == "/catalog/series/fshare_manager.json" {
		serveCatalog(w, r, creds)
		return
	}
	if strings.HasPrefix(normalizedPath, "/meta/series/") {
		serveMeta(w, r, normalizedPath, creds)
		return
	}
	if strings.HasPrefix(normalizedPath, "/stream/series/") {
		serveStream(w, r, normalizedPath, creds)
		return
	}

	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "Route Not Found"})
}

func serveHTMLWizard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlWizardPage))
}

func serveManifest(w http.ResponseWriter, r *http.Request) {
	manifest := map[string]interface{}{
		"id":          "org.fsharefilemanager.local",
		"version":     "4.0.0",
		"name":        "Fshare Local Vault (Go)",
		"description": "Secure automated favorites synced entirely via your static home IP address profile",
		"resources":   []string{"catalog", "meta", "stream"},
		"types":       []string{"series"},
		"catalogs": []map[string]interface{}{
			{"type": "series", "id": "fshare_manager", "name": "Fshare Favorite Folders"},
		},
	}
	json.NewEncoder(w).Encode(manifest)
}

func serveCatalog(w http.ResponseWriter, r *http.Request, creds Credentials) {
	session, err := getActiveSession(creds, false)
	if err != nil {
		json.NewEncoder(w).Encode(CatalogResponse{Metas: []MetaItem{}, Error: err.Error()})
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", "https://www.fshare.vn/api/v3/favorites/explore?sort=-modified", nil)
	req.Header.Set("Authorization", "Bearer "+session.WebToken)
	req.Header.Set("User-Agent", BrowserUserAgent)
	req.Header.Set("Cookie", fmt.Sprintf("session_id=%s; app=%s", session.WebSessionID, session.WebToken))

	resp, err := client.Do(req)
	if err != nil {
		json.NewEncoder(w).Encode(CatalogResponse{Metas: []MetaItem{}, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		session, err = getActiveSession(creds, true)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+session.WebToken)
			req.Header.Set("Cookie", fmt.Sprintf("session_id=%s; app=%s", session.WebSessionID, session.WebToken))
			resp, _ = client.Do(req)
			defer resp.Body.Close()
		}
	}

	var rawData map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rawData)
	items, _ := rawData["items"].([]interface{})
	metas := []MetaItem{}

	for _, rawItem := range items {
		itemMap, ok := rawItem.(map[string]interface{})
		if !ok { continue }
		base := itemMap
		if f, exists := itemMap["file"].(map[string]interface{}); exists && f != nil { base = f }
		if fld, exists := itemMap["folder"].(map[string]interface{}); exists && fld != nil { base = fld }

		linkcode, _ := base["linkcode"].(string)
		if linkcode == "" { linkcode, _ = itemMap["linkcode"].(string) }
		name, _ := base["name"].(string)
		if name == "" { name, _ = itemMap["name"].(string) }

		typeVal := fmt.Sprintf("%v", base["type"])
		if typeVal == "<nil>" { typeVal = fmt.Sprintf("%v", itemMap["type"]) }
		_, hasMime := base["mimetype"]

		if linkcode != "" && (typeVal == "0" || !hasMime) {
			metas = append(metas, MetaItem{
				ID:     "fshare_folder:" + linkcode,
				Type:   "series",
				Name:   name,
				Poster: "https://cdn-icons-png.flaticon.com/512/716/716834.png",
			})
		}
	}
	json.NewEncoder(w).Encode(CatalogResponse{Metas: metas})
}

type FileItem struct {
	Linkcode string
	Name     string
}

func serveMeta(w http.ResponseWriter, r *http.Request, normPath string, creds Credentials) {
	folderLinkcode := strings.TrimSuffix(strings.TrimPrefix(normPath, "/meta/series/"), ".json")
	folderLinkcode = strings.TrimPrefix(folderLinkcode, "fshare_folder:")

	session, err := getActiveSession(creds, false)
	if err != nil {
		json.NewEncoder(w).Encode(MetaResponse{Meta: nil, Error: err.Error()})
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", "https://www.fshare.vn/api/v3/files/folder?linkcode="+folderLinkcode+"&per-page=150", nil)
	req.Header.Set("Authorization", "Bearer "+session.WebToken)
	req.Header.Set("User-Agent", BrowserUserAgent)
	req.Header.Set("Cookie", fmt.Sprintf("session_id=%s", session.WebSessionID))

	resp, err := client.Do(req)
	if err != nil {
		json.NewEncoder(w).Encode(MetaResponse{Meta: nil, Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	var rawData map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rawData)
	items, _ := rawData["items"].([]interface{})
	current, _ := rawData["current"].(map[string]interface{})
	folderName := "Folder Contents"
	if current != nil {
		if n, ok := current["name"].(string); ok { folderName = n }
	}

	files := []FileItem{}
	for _, rawItem := range items {
		itemMap, ok := rawItem.(map[string]interface{})
		if !ok { continue }
		if fmt.Sprintf("%v", itemMap["type"]) == "1" {
			lc, _ := itemMap["linkcode"].(string)
			nm, _ := itemMap["name"].(string)
			if lc != "" && nm != "" { files = append(files, FileItem{Linkcode: lc, Name: nm}) }
		}
	}

	// Natural Sorting implementation (replicates numeric: true)
	sort.Slice(files, func(i, j int) bool {
		return compareNumeric(files[i].Name, files[j].Name)
	})

	videos := []VideoItem{}
	for idx, f := range files {
		videos = append(videos, VideoItem{
			ID:      "fshare_file:" + f.Linkcode,
			Title:   f.Name,
			Season:  1,
			Episode: idx + 1,
		})
	}

	json.NewEncoder(w).Encode(MetaResponse{
		Meta: &MetaDetails{
			ID:     "fshare_folder:" + folderLinkcode,
			Type:   "series",
			Name:   folderName,
			Genres: []string{"Fshare Cloud"},
			Videos: videos,
		},
	})
}

func serveStream(w http.ResponseWriter, r *http.Request, normPath string, creds Credentials) {
	fileLinkcode := strings.TrimSuffix(strings.TrimPrefix(normPath, "/stream/series/"), ".json")
	fileLinkcode = strings.TrimPrefix(fileLinkcode, "fshare_file:")

	session, err := getActiveSession(creds, false)
	if err != nil {
		json.NewEncoder(w).Encode(StreamResponse{Streams: []StreamItem{}})
		return
	}

	executeDownload := func(sess SessionData) (string, int, error) {
		payload, _ := json.Marshal(map[string]string{
			"token":      sess.APIToken,
			"session_id": sess.APISessionID,
			"url":        "https://www.fshare.vn/file/" + fileLinkcode,
		})
		req, _ := http.NewRequest("POST", "https://api.fshare.vn/api/session/download", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", APIUserAgent)
		req.Header.Set("Cookie", "session_id="+sess.APISessionID)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil { return "", 0, err }
		defer resp.Body.Close()

		var resData map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&resData)

		if codeVal, exists := resData["code"]; exists && fmt.Sprintf("%v", codeVal) == "201" {
			return "", http.StatusUnauthorized, nil
		}
		if loc, ok := resData["location"].(string); ok && loc != "" { return loc, http.StatusOK, nil }
		if dlUrl, ok := resData["downloadurl"].(string); ok && dlUrl != "" { return dlUrl, http.StatusOK, nil }

		return "", http.StatusBadRequest, errors.New("resolution failed")
	}

	dlUrl, status, err := executeDownload(session)
	if status == http.StatusUnauthorized || err != nil {
		session, err = getActiveSession(creds, true)
		if err == nil { dlUrl, _, _ = executeDownload(session) }
	}

	streams := []StreamItem{}
	if dlUrl != "" {
		streams = append(streams, StreamItem{Title: "Stream Direct via Local Home Edge Node (Go)", URL: dlUrl})
	}
	json.NewEncoder(w).Encode(StreamResponse{Streams: streams})
}

// Global parsing utility helpers
func decodeURIComponent(str string) string {
	str = strings.ReplaceAll(str, "%3A", ":")
	str = strings.ReplaceAll(str, "%3a", ":")
	return str
}

func compareNumeric(s1, s2 string) bool {
	i, j := 0, 0
	for i < len(s1) && j < len(s2) {
		r1, r2 := rune(s1[i]), rune(s2[j])
		if unicode.IsDigit(r1) && unicode.IsDigit(r2) {
			nStr1, nStr2 := "", ""
			for i < len(s1) && unicode.IsDigit(rune(s1[i])) { nStr1 += string(s1[i]); i++ }
			for j < len(s2) && unicode.IsDigit(rune(s2[j])) { nStr2 += string(s2[j]); j++ }
			n1, _ := strconv.Atoi(nStr1)
			n2, _ := strconv.Atoi(nStr2)
			if n1 != n2 { return n1 < n2 }
			continue
		}
		if r1 != r2 { return r1 < r2 }
		i++; j++
	}
	return len(s1) < len(s2)
}
