package main

import (
    "fmt"
    
    "github.com/HQGroup/nanobot-auto-updater/internal/config"
)

func main() {
    // Test individual validation
    ac := config.APIConfig{
        Port:        70000,
        BearerToken: "short",
    }
    
    err := ac.Validate()
    if err != nil {
        fmt.Printf("API Error: %v\n", err)
    }
}
