package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	ports := []int{18790, 18791, 18792}
	for _, port := range ports {
		address := fmt.Sprintf("127.0.0.1:%d", port)
		conn, err := net.DialTimeout("tcp", address, 1*time.Second)
		if err == nil {
			fmt.Printf("✓ Port %d is OPEN\n", port)
			conn.Close()
		} else {
			fmt.Printf("✗ Port %d is CLOSED (%v)\n", port, err)
		}
	}
}
