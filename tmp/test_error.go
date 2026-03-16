package main

import (
    "fmt"
    "time"
    
    "github.com/HQGroup/nanobot-auto-updater/internal/config"
)

func main() {
    cfg := config.New()
    cfg.API.Port = 70000         // Invalid port
    cfg.API.BearerToken = "short" // Invalid token
    cfg.Monitor.Interval = 30 * time.Second // Invalid interval

    err := cfg.Validate()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    }
}
