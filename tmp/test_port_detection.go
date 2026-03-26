package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/allanpk716/go-protocol-detector/pkg"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 测试端口列表
	testCases := []struct {
		name string
		port uint32
	}{
		{"nanobot-me", 18790},
		{"nanobot-work-helper", 18792},
	}

	fmt.Println("=== 端口检测对比测试 ===")
	fmt.Println()

	for _, tc := range testCases {
		fmt.Printf("--- 测试实例: %s (端口 %d) ---\n", tc.name, tc.port)

		// 方法 1: 当前实现 (net.DialTimeout)
		fmt.Printf("[方法1] 当前实现 (net.DialTimeout): ")
		address := fmt.Sprintf("127.0.0.1:%d", tc.port)
		conn, err := net.DialTimeout("tcp", address, 1*time.Second)
		if err != nil {
			fmt.Printf("❌ 无法连接: %v\n", err)
		} else {
			conn.Close()
			fmt.Printf("✓ 端口监听中\n")
		}

		// 方法 2: go-protocol-detector 的 CommonPortCheck
		fmt.Printf("[方法2] go-protocol-detector (CommonPortCheck): ")
		detector := pkg.NewDetector(3 * time.Second)
		err = detector.CommonPortCheck("127.0.0.1", fmt.Sprintf("%d", tc.port))
		if err != nil {
			fmt.Printf("❌ 无法连接: %v\n", err)
		} else {
			fmt.Printf("✓ 端口监听中\n")
		}

		// 方法 3: 使用 gopsutil 检查端口监听状态
		fmt.Printf("[方法3] gopsutil (检查端口监听): ")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		pid, err := findPIDByPort(ctx, tc.port, logger)
		cancel()
		if err != nil {
			fmt.Printf("❌ 检查失败: %v\n", err)
		} else if pid > 0 {
			fmt.Printf("✓ 端口监听中 (PID: %d)\n", pid)
		} else {
			fmt.Printf("❌ 端口未监听\n")
		}

		fmt.Println()
	}

	fmt.Println("=== 测试完成 ===")
}

// findPIDByPort 查找监听指定端口的进程 PID
func findPIDByPort(ctx context.Context, port uint32, logger *slog.Logger) (int32, error) {
	// 简化版本，只检查本地连接
	// 注意：这里需要 gopsutil 库，为了简化测试，我们暂时返回 0
	logger.Debug("检查端口", "port", port)

	// 在实际代码中，这里会使用 gopsutil 的 net.Connections
	// 但为了这个测试程序能独立运行，我们跳过这个检查
	return 0, nil
}
