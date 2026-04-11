// Initialize from URL path
const instanceName = window.location.pathname.split('/').pop();

// State variables
let currentInstance = instanceName;
let autoScroll = true;
let eventSource = null;
let reconnectAttempts = 0;

// DOM elements
const logsContainer = document.getElementById('logs');
const connectionStatus = document.getElementById('connection-status');
const scrollToggle = document.getElementById('scroll-toggle');
const instanceSelect = document.getElementById('instance-select');
const backToHomeButton = document.getElementById('back-to-home');
const restartButton = document.getElementById('restart-button');

// Back to home button click handler
backToHomeButton.addEventListener('click', function() {
    window.location.href = '/';
});

// Restart button click handler
restartButton.addEventListener('click', function() {
    restartInstance(currentInstance, restartButton);
});

// Load instance selector from API
async function loadInstanceSelector() {
    try {
        const response = await fetch('/api/v1/instances');
        const data = await response.json();
        const select = document.getElementById('instance-select');

        select.innerHTML = '';

        data.instances.forEach(name => {
            const option = document.createElement('option');
            option.value = name;
            option.textContent = name;
            select.appendChild(option);
        });

        // Select current instance from URL
        if (data.instances.includes(instanceName)) {
            select.value = instanceName;
        }

        // If no instances, show message
        if (data.instances.length === 0) {
            const option = document.createElement('option');
            option.textContent = 'No instances configured';
            option.disabled = true;
            select.appendChild(option);
        }
    } catch (error) {
        console.error('Failed to load instance list:', error);
    }
}

// Select instance and reconnect
function selectInstance(instanceNameParam) {
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }

    // Clear log container
    logsContainer.innerHTML = '';

    // Update URL without reload
    window.history.pushState({}, '', '/logs/' + instanceNameParam);

    currentInstance = instanceNameParam;
    autoScroll = true;
    updateScrollButtonText();

    // Show empty state
    showEmptyState();

    // Connect to new instance
    connectSSE(instanceNameParam);
}

// Show empty state message
function showEmptyState() {
    if (logsContainer.children.length === 0) {
        logsContainer.innerHTML = '<div class="empty-state"><h2>等待日志...</h2><p>实例启动后,日志将实时显示在此处。</p></div>';
    }
}

// Update scroll button text
function updateScrollButtonText() {
    scrollToggle.textContent = autoScroll ? '暂停滚动' : '恢复滚动';
}

// Connect to SSE stream
function connectSSE(instance) {
    // Update status to connecting
    updateConnectionStatus('connecting');

    // Close existing connection if any
    if (eventSource) {
        eventSource.close();
    }

    // Create new EventSource connection
    eventSource = new EventSource('/api/v1/logs/' + instance + '/stream');

    // Connection opened
    eventSource.onopen = function() {
        updateConnectionStatus('connected');
        reconnectAttempts = 0;
    };

    // Connection error
    eventSource.onerror = function() {
        updateConnectionStatus('disconnected');
        reconnectAttempts++;

        // EventSource will automatically try to reconnect
        // After 10 failed attempts, show message
        if (reconnectAttempts >= 10) {
            appendLog('连接已断开，请刷新页面重试', 'stderr');
        }
    };

    // Listen for stdout events
    eventSource.addEventListener('stdout', function(e) {
        appendLog(e.data, 'stdout');
    });

    // Listen for stderr events
    eventSource.addEventListener('stderr', function(e) {
        appendLog(e.data, 'stderr');
    });

    // Listen for connected event
    eventSource.addEventListener('connected', function(e) {
        const data = JSON.parse(e.data);
        console.log('Connected to instance:', data.instance);
    });
}

// Append log message to container
function appendLog(message, source) {
    // Remove empty state if present
    const emptyState = logsContainer.querySelector('.empty-state');
    if (emptyState) {
        emptyState.remove();
    }

    // Strip ANSI escape codes from message
    const cleanMessage = stripAnsiCodes(message);

    // Create log line element
    const logLine = document.createElement('div');
    logLine.className = 'log-' + source;
    logLine.textContent = cleanMessage;

    // Append to container
    logsContainer.appendChild(logLine);

    // Auto-scroll if enabled
    if (autoScroll) {
        logsContainer.scrollTop = logsContainer.scrollHeight;
    }
}

// Strip ANSI escape codes from text
function stripAnsiCodes(text) {
    // Match ANSI escape sequences: ESC[...m format
    // \x1b = ESC character
    // \[ = literal opening bracket
    // [0-9;]* = any digits or semicolons (color codes)
    // m = terminating character
    const ansiRegex = /\x1b\[[0-9;]*m/g;
    return text.replace(ansiRegex, '');
}

// Update connection status indicator
function updateConnectionStatus(status) {
    // Remove all status classes
    connectionStatus.classList.remove('status-connecting', 'status-connected', 'status-disconnected');

    // Add appropriate class and text
    if (status === 'connecting') {
        connectionStatus.classList.add('status-connecting');
        connectionStatus.textContent = '连接中...';
    } else if (status === 'connected') {
        connectionStatus.classList.add('status-connected');
        connectionStatus.textContent = '已连接';
    } else if (status === 'disconnected') {
        connectionStatus.classList.add('status-disconnected');
        connectionStatus.textContent = '已断开';
    }
}

// Scroll event listener for smart auto-scroll toggle
logsContainer.addEventListener('scroll', function() {
    const scrollPosition = logsContainer.scrollTop + logsContainer.clientHeight;
    const scrollHeight = logsContainer.scrollHeight;

    // If within 50px of bottom, enable auto-scroll
    if (scrollHeight - scrollPosition <= 50) {
        autoScroll = true;
    } else {
        autoScroll = false;
    }

    // Update button text
    updateScrollButtonText();
});

// Toggle button click handler
scrollToggle.addEventListener('click', function() {
    autoScroll = !autoScroll;

    // If enabling auto-scroll, scroll to bottom
    if (autoScroll) {
        logsContainer.scrollTop = logsContainer.scrollHeight;
    }

    // Update button text
    updateScrollButtonText();
});

// Instance selector change handler
instanceSelect.addEventListener('change', function(e) {
    selectInstance(e.target.value);
});

// Update scroll toggle button text
function updateScrollButtonText() {
    if (autoScroll) {
        scrollToggle.textContent = '暂停滚动';
    } else {
        scrollToggle.textContent = '恢复滚动';
    }
}

// Restart instance function
async function restartInstance(instanceName, button) {
    const originalText = button.textContent;

    try {
        // Disable button and show loading state
        button.disabled = true;
        button.classList.add('loading');
        button.textContent = '重启中...';

        // Call restart API
        const response = await fetch(`/api/v1/instances/${instanceName}/restart`, {
            method: 'POST'
        });

        const data = await response.json();

        if (response.ok && data.success) {
            button.textContent = '重启成功';

            // Reconnect SSE stream after restart
            if (eventSource) {
                eventSource.close();
                eventSource = null;
            }

            // Clear logs and reconnect
            logsContainer.innerHTML = '';
            showEmptyState();
            connectSSE(instanceName);

            // Restore button after 2 seconds
            setTimeout(() => {
                button.textContent = originalText;
                button.disabled = false;
                button.classList.remove('loading');
            }, 2000);
        } else {
            throw new Error(data.error || '重启失败');
        }
    } catch (error) {
        console.error('Failed to restart instance:', error);
        button.textContent = '重启失败';
        alert(`重启实例 ${instanceName} 失败: ${error.message}`);
        // Restore button after 2 seconds
        setTimeout(() => {
            button.textContent = originalText;
            button.disabled = false;
            button.classList.remove('loading');
        }, 2000);
    }
}

// Initialize on DOMContentLoaded
document.addEventListener('DOMContentLoaded', function() {
    // Load instance selector
    loadInstanceSelector();

    // Show empty state
    showEmptyState();

    // Connect to SSE stream
    connectSSE(instanceName);

    // Initialize button text
    updateScrollButtonText();

    // Load version into header badge
    loadHeaderVersion();
});

// Load version for header badge (no auth required)
async function loadHeaderVersion() {
    try {
        const resp = await fetch('/api/v1/version');
        if (resp.ok) {
            const data = await resp.json();
            const badge = document.getElementById('header-version');
            if (badge && data.version) {
                badge.textContent = 'v' + data.version.replace(/^v/, '');
            }
        }
    } catch (e) {
        // Non-critical, ignore
    }
}
