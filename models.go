package main

const (
	WebAppKey        = "L2S7R6ZMagggC5wWkQhX2+aDi467PPuftWUMRFSn"
	PyloadAppKey     = "dMnqMMZMUnN5YpvKENaEhdQQ5jxDqddt"
	APIUserAgent     = "pyLoad-B1RS5N"
	BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
)

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SessionData struct {
	WebToken     string `json:"webToken"`
	WebSessionID string `json:"webSessionId"`
	APIToken     string `json:"apiToken"`
	APISessionID string `json:"apiSessionId"`
}

type CacheItem struct {
	Data      SessionData
	ExpiresAt int64
}

// Stremio core exchange profiles
type MetaItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Poster string `json:"poster,omitempty"`
}

type CatalogResponse struct {
	Metas []MetaItem `json:"metas"`
	Error string     `json:"error,omitempty"`
}

type VideoItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Season  int    `json:"season"`
	Episode int    `json:"episode"`
}

type MetaDetails struct {
	ID     string      `json:"id"`
	Type   string      `json:"type"`
	Name   string      `json:"name"`
	Genres []string    `json:"genres"`
	Videos []VideoItem `json:"videos"`
}

type MetaResponse struct {
	Meta  *MetaDetails `json:"meta"`
	Error string       `json:"error,omitempty"`
}

type StreamItem struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type StreamResponse struct {
	Streams []StreamItem `json:"streams"`
}
