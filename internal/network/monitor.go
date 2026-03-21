package network

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"syscall"
	"time"
)

// ConnectivityState 连通性状态
type ConnectivityState struct {
	IsConnected  bool
	LastCheck    time.Time
	ErrorMessage string // 最后一次错误消息（连通时为空）
}

// NetworkMonitor 网络连通性监控器
type NetworkMonitor struct {
	targetURL  string
	interval   time.Duration
	timeout    time.Duration
	logger     *slog.Logger
	httpClient *http.Client
	state      *ConnectivityState
	mu         sync.RWMutex // 保护 state 的读写锁
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewNetworkMonitor 创建网络监控器
func NewNetworkMonitor(
	targetURL string,
	interval time.Duration,
	timeout time.Duration,
	logger *slog.Logger,
) *NetworkMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建 HTTP 客户端，禁用重定向跟随
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不跟随重定向
		},
	}

	return &NetworkMonitor{
		targetURL:  targetURL,
		interval:   interval,
		timeout:    timeout,
		logger:     logger.With("component", "network-monitor"),
		httpClient: client,
		state:      nil, // 初始状态为 nil，表示首次检查
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动网络监控循环（在独立 goroutine 中运行）
func (nm *NetworkMonitor) Start() {
	nm.logger.Info("网络监控已启动",
		"target", nm.targetURL,
		"interval", nm.interval,
		"timeout", nm.timeout)

	ticker := time.NewTicker(nm.interval)
	defer ticker.Stop()

	// 立即执行一次初始检查
	nm.checkConnectivity()

	for {
		select {
		case <-nm.ctx.Done():
			nm.logger.Info("网络监控已停止")
			return
		case <-ticker.C:
			nm.checkConnectivity()
		}
	}
}

// checkConnectivity 检查连通性并记录日志
func (nm *NetworkMonitor) checkConnectivity() {
	start := time.Now()
	isConnected, statusCode, errMsg := nm.performCheck()
	duration := time.Since(start)

	// 更新状态（加锁保护）
	now := time.Now()
	nm.mu.Lock()
	previousState := nm.state
	nm.state = &ConnectivityState{
		IsConnected:  isConnected,
		LastCheck:    now,
		ErrorMessage: errMsg, // 记录错误消息（成功时为空字符串）
	}
	nm.mu.Unlock()

	// 记录日志
	if isConnected {
		// MONITOR-03: 成功时记录 INFO
		nm.logger.Info("Google 连通性检查成功",
			"duration", duration.Milliseconds(),
			"status_code", statusCode)
	} else {
		// MONITOR-02: 失败时记录 ERROR
		nm.logger.Error("Google 连通性检查失败",
			"duration", duration.Milliseconds(),
			"error_type", errMsg)
	}

	// 记录状态变化（首次检查、状态保持、状态改变）
	if previousState == nil {
		nm.logger.Info("初始连通性状态", "is_connected", isConnected)
	} else if previousState.IsConnected != isConnected {
		if previousState.IsConnected && !isConnected {
			nm.logger.Warn("连通性状态改变: 从连通变为不连通")
		} else {
			nm.logger.Info("连通性状态改变: 从不连通变为连通")
		}
	}
}

// performCheck 执行 HTTP HEAD 请求检查连通性
// 返回: (是否连通, HTTP 状态码, 错误消息)
func (nm *NetworkMonitor) performCheck() (bool, int, string) {
	req, err := http.NewRequest(http.MethodHead, nm.targetURL, nil)
	if err != nil {
		return false, 0, fmt.Sprintf("创建请求失败: %v", err)
	}

	resp, err := nm.httpClient.Do(req)
	if err != nil {
		errMsg := nm.classifyError(err)
		return false, 0, errMsg
	}
	defer resp.Body.Close()

	// 仅 HTTP 200 OK 算成功
	if resp.StatusCode == http.StatusOK {
		return true, resp.StatusCode, ""
	}

	// 非 200 状态码视为失败
	errMsg := fmt.Sprintf("HTTP 状态码 %d (%s)", resp.StatusCode, resp.Status)
	return false, resp.StatusCode, errMsg
}

// classifyError 分类错误类型
func (nm *NetworkMonitor) classifyError(err error) string {
	// 解析 URL 错误
	if urlErr, ok := err.(*url.Error); ok {
		// 超时错误
		if urlErr.Timeout() {
			return "连接超时"
		}

		innerErr := urlErr.Err

		// DNS 解析失败
		if netErr, ok := innerErr.(*net.DNSError); ok {
			return fmt.Sprintf("DNS 解析失败: %s", netErr.Name)
		}

		// 连接拒绝
		if netErr, ok := innerErr.(*net.OpError); ok {
			if syscallErr, ok := netErr.Err.(*os.SyscallError); ok {
				if syscallErr.Err == syscall.ECONNREFUSED {
					return "连接被拒绝"
				}
			}
		}

		// TLS 握手错误
		if _, ok := innerErr.(*tls.CertificateVerificationError); ok {
			return "TLS 证书验证失败"
		}
		if _, ok := innerErr.(*x509.UnknownAuthorityError); ok {
			return "TLS 未知证书颁发机构"
		}

		return fmt.Sprintf("网络错误: %v", innerErr)
	}

	return fmt.Sprintf("未知错误: %v", err)
}

// Stop 停止网络监控
func (nm *NetworkMonitor) Stop() {
	nm.logger.Info("正在停止网络监控...")
	nm.cancel()
}

// GetState 获取当前连通性状态（供 Phase 27 使用）
func (nm *NetworkMonitor) GetState() *ConnectivityState {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.state
}
