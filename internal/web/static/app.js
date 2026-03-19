// Initialize from URL path
const instanceName = window.location.pathname.split('/').pop();

// State variables
let autoScroll = true;
let eventSource = null;
let reconnectAttempts = 0;

// DOM elements
const logsContainer = document.getElementById('logs');
const connectionStatus = document.getElementById('connection-status');
const scrollToggle = document.getElementById('scroll-toggle');
const instanceSelect = document.getElementById('instance-select');

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

    // Create log line element
    const logLine = document.createElement('div');
    logLine.className = 'log-' + source;
    logLine.textContent = message;

    // Append to container
    logsContainer.appendChild(logLine);

    // Auto-scroll if enabled
    if (autoScroll) {
        logsContainer.scrollTop = logsContainer.scrollHeight;
    }
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

// Update scroll toggle button text
function updateScrollButtonText() {
    if (autoScroll) {
        scrollToggle.textContent = '暂停滚动';
    } else {
        scrollToggle.textContent = '恢复滚动';
    }
}

// Initialize on DOMContentLoaded
document.addEventListener('DOMContentLoaded', function() {
    // Connect to SSE stream
    connectSSE(instanceName);

    // Initialize button text
    updateScrollButtonText();
});
