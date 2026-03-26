@echo off
REM 测试 nanobot 进程是否能正常启动

echo === 测试 nanobot 进程启动 ===
echo.

echo [1] 检查 nanobot 命令是否存在
where nanobot
if %ERRORLEVEL% NEQ 0 (
    echo ❌ nanobot 命令不存在
    pause
    exit /b 1
)
echo ✓ nanobot 命令存在
echo.

echo [2] 测试启动 nanobot-me (端口 18790)
echo 命令: nanobot gateway --port 18790
echo 按任意键启动...
pause
start /B nanobot gateway --port 18790
timeout /t 5 /nobreak >nul

echo.
echo [3] 检查端口 18790 是否监听
netstat -ano | findstr :18790
if %ERRORLEVEL% EQU 0 (
    echo ✓ 端口 18790 正在监听
) else (
    echo ❌ 端口 18790 未监听
)

echo.
echo [4] 检查 nanobot 进程
tasklist | findstr nanobot
if %ERRORLEVEL% EQU 0 (
    echo ✓ nanobot 进程正在运行
) else (
    echo ❌ nanobot 进程未运行
)

echo.
echo [5] 测试启动 nanobot-work-helper (端口 18792)
echo 命令: nanobot gateway --config C:/Users/allan716/.nanobot-work-helper/config.json --port 18792
echo 按任意键启动...
pause
start /B nanobot gateway --config C:/Users/allan716/.nanobot-work-helper/config.json --port 18792
timeout /t 5 /nobreak >nul

echo.
echo [6] 检查端口 18792 是否监听
netstat -ano | findstr :18792
if %ERRORLEVEL% EQU 0 (
    echo ✓ 端口 18792 正在监听
) else (
    echo ❌ 端口 18792 未监听
)

echo.
echo [7] 再次检查所有 nanobot 进程
tasklist | findstr nanobot

echo.
echo === 测试完成 ===
pause
