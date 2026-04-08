# Troubleshooting

> This content was extracted from README.md for better organization.

## 故障排除

### 常见问题

#### 1. 更新失败：找不到 uv 命令

**症状**: 日志显示 `uv: command not found` 或类似错误

**解决方案**:
```bash
# 检查 uv 是否已安装
uv --version

# 如果未安装，使用以下命令安装
powershell -ExecutionPolicy ByPass -c "irm https://astral.sh/uv/install.ps1 | iex"
```

#### 2. 无法停止 Nanobot 进程

**症状**: 日志显示停止超时或进程仍在运行

**解决方案**:
```powershell
# 手动查找并停止 Nanobot 进程
tasklist | findstr nanobot
taskkill /F /IM nanobot.exe

# 检查端口占用
netstat -ano | findstr :18790
```

#### 3. Pushover 通知未收到

**症状**: 更新成功但没有收到通知

**排查步骤**:
1. 检查 `config.yaml` 中的 `api_token` 和 `user_key` 是否正确
2. 确认 Pushover 账户是否有效
3. 检查日志中是否有通知发送错误

#### 4. 守护进程模式无日志输出

**症状**: 使用 `--update-now` 后看不到日志

**解决方案**:
守护进程的日志会重定向到 `logs/daemon.log` 文件：
```bash
# 查看守护进程日志
type logs\daemon.log

# 或禁用守护进程模式进行调试
$env:NO_DAEMON = "1"
./nanobot-auto-updater.exe --update-now
```

#### 5. 更新挂起或超时

**症状**: 更新命令执行很久没有响应

**诊断**:
```bash
# 查看实时日志
Get-Content logs\app-2026-03-01.log -Wait

# 检查更新心跳日志（每 10 秒输出一次）
# 应该看到类似以下的日志：
# [INFO] Update heartbeat: still running... (30s elapsed)
```

**可能原因**:
- 网络连接问题
- GitHub/PyPI 访问受限
- uv 命令挂起

#### 6. API 认证失败

**症状**: 返回 `401 Unauthorized`

**解决方案**:
```bash
# 检查 Bearer Token 是否正确配置
# 确保配置文件中的 token 与请求中的 token 一致
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer your-exact-token-here"

# 确认 token 长度至少 32 个字符
```

#### 7. 更新锁冲突

**症状**: 返回 `429 Too Many Requests` 或 `Update already in progress`

**原因**: 另一个更新正在进行中

**解决方案**:
```bash
# 等待当前更新完成（通常几分钟内）
# 查看日志确认更新进度
Get-Content logs\app-2026-03-16.log -Wait

# 如果确认没有更新在运行，可能是锁文件残留
# 锁文件位置: logs/update.lock（重启程序会自动清理）
```

#### 8. 监控服务未启动

**症状**: API 可用但监控不工作

**排查步骤**:
1. 检查 `config.yaml` 中的 `monitor` 配置段是否存在
2. 确认 `monitor.interval` 配置正确（至少 1 分钟）
3. 查看日志中是否有监控服务启动记录
4. 确认未使用 `--skip-monitor` 参数

#### 9. Bearer Token 配置错误

**症状**: 配置加载失败，提示 token 长度不足

**解决方案**:
```yaml
# 确保 bearer_token 至少 32 个字符
api:
  bearer_token: "this-token-must-be-at-least-32-characters-long"
```

### 获取帮助

如果遇到无法解决的问题：

1. **查看日志**: 检查 `logs/` 目录中的最新日志文件
2. **提交 Issue**: [GitHub Issues](https://github.com/HQGroup/nanobot-auto-updater/issues)
3. **提供信息**: 包括日志片段、配置文件（去除敏感信息）和系统环境
