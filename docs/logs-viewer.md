# Real-time Log Viewer

> This content was extracted from README.md for better organization.

## 实时日志查看 (v0.4+)

v0.4 版本新增实时日志查看功能，可以通过 Web UI 或 SSE API 查看 Nanobot 实例的实时输出。

### 方式 1：Web UI（推荐）

访问浏览器查看实时日志：

```
http://localhost:8080/logs/{实例名称}
```

**示例：**

如果你的配置是：
```yaml
instances:
  - name: "nanobot-gateway"
    port: 18790
  - name: "nanobot-work-helper"
    port: 18792
```

则可以访问：
- http://localhost:8080/logs/nanobot-gateway
- http://localhost:8080/logs/nanobot-work-helper

**Web UI 功能：**
- **实时滚动** - 类似 `tail -f`，自动滚动到最新日志
- **实例选择器** - 下拉菜单快速切换实例
- **暂停/恢复** - 按钮控制自动滚动
- **颜色区分** - Stdout（蓝色）/ Stderr（红色）
- **连接状态** - 实时显示 SSE 连接状态
- **历史日志** - 保留最近 5000 行日志
- **单文件部署** - 静态资源嵌入二进制，无需外部文件

**使用截图示例：**

打开浏览器访问 `http://localhost:8080/logs/nanobot-gateway`：
1. 页面顶部显示实例选择下拉菜单
2. 左上角显示连接状态（连接中/已连接/已断开）
3. 右上角有暂停/恢复按钮
4. 日志实时滚动，蓝色为 stdout，红色为 stderr
5. 手动滚动时自动暂停，滚动到底部时自动恢复

### 方式 2：SSE API（程序化访问）

通过 Server-Sent Events API 实时接收日志流：

```bash
# 实时流式日志（Ctrl+C 退出）
curl -N http://localhost:8080/api/v1/logs/nanobot-gateway/stream

# 返回 SSE 格式：
# event: stdout
# data: {"timestamp":"2026-03-20T10:30:00Z","content":"Starting nanobot..."}

# event: stderr
# data: {"timestamp":"2026-03-20T10:30:01Z","content":"Warning: ..."}
```

**SSE 事件格式：**

```
event: stdout|stderr
data: {"timestamp":"RFC3339时间戳","content":"日志内容"}
```

**特性：**
- **历史回放** - 连接时自动发送最近 5000 行历史日志
- **实时推送** - 新日志实时推送到客户端
- **心跳保活** - 每 30 秒发送心跳注释防止超时
- **自动重连** - 客户端断开后可自动重连

### 方式 3：浏览器 EventSource API

在 JavaScript 中使用 EventSource API 接收日志：

```html
<script>
const eventSource = new EventSource('/api/v1/logs/nanobot-gateway/stream');

eventSource.addEventListener('stdout', (e) => {
  const data = JSON.parse(e.data);
  console.log(`[OUT] ${data.timestamp}: ${data.content}`);
});

eventSource.addEventListener('stderr', (e) => {
  const data = JSON.parse(e.data);
  console.error(`[ERR] ${data.timestamp}: ${data.content}`);
});

eventSource.onerror = (e) => {
  console.error('SSE connection error');
  // EventSource 会自动重连
};
</script>
```

### 技术细节

**架构：**
- **LogBuffer** - 环形缓冲区，固定 5000 行容量，FIFO 自动覆盖
- **Log Capture** - os.Pipe() 捕获 stdout/stderr，并发 goroutine 读取
- **SSE Streaming** - Server-Sent Events 协议，30 秒心跳保活
- **Instance Isolation** - 每个实例独立缓冲，互不影响

**性能：**
- **内存占用** - 每个实例约 1MB（5000 行 x 200 字节/行）
- **写入性能** - O(1) 非阻塞写入
- **并发安全** - sync.RWMutex 保护，支持多客户端同时访问
- **优雅降级** - 慢客户端自动丢弃日志，不影响主流程
