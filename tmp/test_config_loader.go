//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test_config_loader <config_file>")
		os.Exit(1)
	}

	configPath := os.Args[1]
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("❌ 加载失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 配置加载成功\n")
	fmt.Printf("  Cron: %s\n", cfg.Cron)

	if len(cfg.Instances) > 0 {
		fmt.Printf("  模式: 多实例 (instances)\n")
		fmt.Printf("  实例数量: %d\n", len(cfg.Instances))
		for i, inst := range cfg.Instances {
			fmt.Printf("    [%d] 名称: %s, 端口: %d, 启动命令: %s\n", i+1, inst.Name, inst.Port, inst.StartCommand)
		}
	} else {
		fmt.Printf("  模式: 单实例 (legacy)\n")
		fmt.Printf("  端口: %d\n", cfg.Nanobot.Port)
		fmt.Printf("  启动超时: %v\n", cfg.Nanobot.StartupTimeout)
	}
}
