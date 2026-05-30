package main

import (
	"fmt"
	"net/http"
)

func main() {
	port := 8787
	
	// Sets up our master custom HTTP handler
	http.HandleFunc("/", masterHandler)

	fmt.Printf("🚀 Your Fshare Addon is running locally in Go at http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Printf("Server failed to initialize: %v\n", err)
	}
}
