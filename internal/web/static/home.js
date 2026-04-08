// DOM elements
const instancesGrid = document.getElementById('instances-grid');

// Load instance status from API
async function loadInstances() {
    try {
        const response = await fetch('/api/v1/instances/status');
        const data = await response.json();

        // Clear loading state
        instancesGrid.innerHTML = '';

        // If no instances, show message
        if (data.instances.length === 0) {
            instancesGrid.innerHTML = '<div class="empty-state"><h2>无实例配置</h2><p>请在配置文件中添加实例</p></div>';
            return;
        }

        // Render instance cards
        data.instances.forEach(instance => {
            const card = createInstanceCard(instance);
            instancesGrid.appendChild(card);
        });
    } catch (error) {
        console.error('Failed to load instance status:', error);
        instancesGrid.innerHTML = '<div class="empty-state"><h2>加载失败</h2><p>无法获取实例列表，请检查服务器状态</p></div>';
    }
}

// Create instance card element
function createInstanceCard(instance) {
    const card = document.createElement('div');
    card.className = 'instance-card';

    // Status indicator
    const statusClass = instance.running ? 'status-running' : 'status-stopped';
    const statusText = instance.running ? '运行中' : '已停止';

    const nameLink = document.createElement('a');
    nameLink.href = '/logs/' + encodeURIComponent(instance.name);
    nameLink.className = 'instance-name';
    nameLink.textContent = instance.name;
    card.appendChild(nameLink);

    const infoDiv = document.createElement('div');
    infoDiv.className = 'instance-info';

    const portRow = document.createElement('div');
    portRow.className = 'info-row';
    const portLabel = document.createElement('span');
    portLabel.className = 'label';
    portLabel.textContent = '端口:';
    const portValue = document.createElement('span');
    portValue.className = 'value';
    portValue.textContent = instance.port;
    portRow.appendChild(portLabel);
    portRow.appendChild(portValue);
    infoDiv.appendChild(portRow);

    const statusRow = document.createElement('div');
    statusRow.className = 'info-row';
    const statusLabel = document.createElement('span');
    statusLabel.className = 'label';
    statusLabel.textContent = '状态:';
    const statusValue = document.createElement('span');
    statusValue.className = 'value ' + statusClass;
    statusValue.textContent = statusText;
    statusRow.appendChild(statusLabel);
    statusRow.appendChild(statusValue);
    infoDiv.appendChild(statusRow);
    card.appendChild(infoDiv);

    const restartBtn = document.createElement('button');
    restartBtn.className = 'btn-restart';
    restartBtn.dataset.instance = instance.name;
    restartBtn.textContent = '重启实例';
    restartBtn.addEventListener('click', function() {
        restartInstance(instance.name, restartBtn);
    });
    card.appendChild(restartBtn);

    return card;
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
            // Refresh instance status after 2 seconds
            setTimeout(() => {
                loadInstances();
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
    // Load instances
    loadInstances();

    // Auto-refresh every 5 seconds
    setInterval(loadInstances, 5000);

    // Initialize self-update module
    initSelfUpdate();
});

// Self-update module
let authToken = '';
let pollTimer = null;
let pollStartTime = 0;
let isUpdating = false;

// Initialize self-update module
async function initSelfUpdate() {
    try {
        const resp = await fetch('/api/v1/web-config');
        if (!resp.ok) {
            throw new Error('web-config unavailable');
        }
        const data = await resp.json();
        authToken = data.auth_token;
        // Load current version from check API
        await loadCurrentVersion();
        // Bind button events
        document.getElementById('btn-check-update').addEventListener('click', checkUpdate);
        document.getElementById('btn-start-update').addEventListener('click', startUpdate);
    } catch (e) {
        // web-config fetch failed (non-localhost)
        console.error('Failed to init self-update:', e);
        const section = document.getElementById('selfupdate-section');
        section.innerHTML = '<p class="selfupdate-warning">请在本地访问以使用自更新功能</p>';
    }
}

// Load current version from check API
async function loadCurrentVersion() {
    try {
        const resp = await fetch('/api/v1/self-update/check', {
            headers: { 'Authorization': 'Bearer ' + authToken }
        });
        if (!resp.ok) {
            throw new Error('check API failed');
        }
        const data = await resp.json();
        document.getElementById('current-version').textContent = data.current_version;
    } catch (e) {
        console.error('Failed to load current version:', e);
        document.getElementById('current-version').textContent = 'N/A';
    }
}

// Check for updates
async function checkUpdate() {
    const btn = document.getElementById('btn-check-update');
    const startBtn = document.getElementById('btn-start-update');
    const resultDiv = document.getElementById('update-result');

    if (isUpdating) return;

    // Disable button during check
    btn.disabled = true;
    btn.textContent = '检测中...';
    startBtn.disabled = true;

    try {
        const resp = await fetch('/api/v1/self-update/check', {
            headers: { 'Authorization': 'Bearer ' + authToken }
        });
        if (!resp.ok) {
            throw new Error('check API returned ' + resp.status);
        }
        const data = await resp.json();

        if (!data.needs_update) {
            // Already up to date
            resultDiv.className = 'update-result visible';
            resultDiv.innerHTML = '';
            const infoDiv = document.createElement('div');
            infoDiv.className = 'update-info';
            const label = document.createElement('span');
            label.className = 'info-label';
            label.textContent = '已是最新版本';
            infoDiv.appendChild(label);
            const value = document.createElement('span');
            value.className = 'info-value';
            value.textContent = data.current_version;
            infoDiv.appendChild(value);
            resultDiv.appendChild(infoDiv);
            startBtn.disabled = true;
        } else {
            // New version available — render details
            resultDiv.className = 'update-result visible';
            resultDiv.innerHTML = '';

            const infoDiv = document.createElement('div');
            infoDiv.className = 'update-info';

            // Version row
            const versionRow = document.createElement('div');
            versionRow.style.marginBottom = '4px';
            const versionLabel = document.createElement('span');
            versionLabel.className = 'info-label';
            versionLabel.textContent = '最新版本:';
            const versionValue = document.createElement('span');
            versionValue.className = 'info-value';
            versionValue.textContent = data.latest_version;
            versionRow.appendChild(versionLabel);
            versionRow.appendChild(versionValue);
            infoDiv.appendChild(versionRow);

            // Date row
            if (data.published_at) {
                let dateStr = '';
                try {
                    const d = new Date(data.published_at);
                    dateStr = d.getFullYear() + '-' +
                        String(d.getMonth() + 1).padStart(2, '0') + '-' +
                        String(d.getDate()).padStart(2, '0');
                } catch (e) {
                    dateStr = data.published_at;
                }
                const dateRow = document.createElement('div');
                dateRow.style.marginBottom = '4px';
                const dateLabel = document.createElement('span');
                dateLabel.className = 'info-label';
                dateLabel.textContent = '发布日期:';
                const dateValue = document.createElement('span');
                dateValue.className = 'info-value';
                dateValue.textContent = dateStr;
                dateRow.appendChild(dateLabel);
                dateRow.appendChild(dateValue);
                infoDiv.appendChild(dateRow);
            }

            resultDiv.appendChild(infoDiv);

            // Release notes with expand/collapse (textContent for XSS safety)
            if (data.release_notes) {
                const notesDiv = document.createElement('div');
                notesDiv.className = 'release-notes';
                notesDiv.textContent = data.release_notes;
                resultDiv.appendChild(notesDiv);

                const toggleBtn = document.createElement('span');
                toggleBtn.className = 'release-notes-toggle';
                toggleBtn.textContent = '展开';
                toggleBtn.addEventListener('click', function() {
                    if (notesDiv.classList.contains('expanded')) {
                        notesDiv.classList.remove('expanded');
                        toggleBtn.textContent = '展开';
                    } else {
                        notesDiv.classList.add('expanded');
                        toggleBtn.textContent = '收起';
                    }
                });
                resultDiv.appendChild(toggleBtn);
            }

            startBtn.disabled = false;
        }
    } catch (e) {
        console.error('Failed to check for updates:', e);
        resultDiv.className = 'update-result visible';
        resultDiv.innerHTML = '';
        const errorDiv = document.createElement('div');
        errorDiv.className = 'update-error';
        errorDiv.textContent = '检测更新失败: ' + e.message;
        resultDiv.appendChild(errorDiv);
        startBtn.disabled = true;
    } finally {
        btn.disabled = false;
        btn.textContent = '检测更新';
    }
}

// Start update
async function startUpdate() {
    const btn = document.getElementById('btn-start-update');
    if (isUpdating || btn.disabled) return;

    try {
        btn.disabled = true;
        document.getElementById('btn-check-update').disabled = true;

        const resp = await fetch('/api/v1/self-update', {
            method: 'POST',
            headers: { 'Authorization': 'Bearer ' + authToken }
        });

        if (resp.status === 409) {
            // Already updating
            const resultDiv = document.getElementById('update-result');
            resultDiv.className = 'update-result visible';
            resultDiv.innerHTML = '';
            const errorDiv = document.createElement('div');
            errorDiv.className = 'update-error';
            errorDiv.textContent = '更新已在进行中，请稍后再试';
            resultDiv.appendChild(errorDiv);
            btn.disabled = false;
            document.getElementById('btn-check-update').disabled = false;
            return;
        }

        if (!resp.ok) {
            throw new Error('update API returned ' + resp.status);
        }

        // Start progress polling
        isUpdating = true;
        pollStartTime = Date.now();
        startProgressPolling();
    } catch (e) {
        console.error('Failed to start update:', e);
        const resultDiv = document.getElementById('update-result');
        resultDiv.className = 'update-result visible';
        resultDiv.innerHTML = '';
        const errorDiv = document.createElement('div');
        errorDiv.className = 'update-error';
        errorDiv.textContent = '启动更新失败: ' + e.message;
        resultDiv.appendChild(errorDiv);
        btn.disabled = false;
        document.getElementById('btn-check-update').disabled = false;
    }
}

// Progress polling (500ms interval, 60s timeout)
function startProgressPolling() {
    const resultDiv = document.getElementById('update-result');
    resultDiv.className = 'update-result visible';
    resultDiv.innerHTML = '';

    const container = document.createElement('div');
    container.className = 'progress-container';

    const statusEl = document.createElement('div');
    statusEl.className = 'progress-status';
    statusEl.id = 'progress-status';
    statusEl.textContent = '检查中...';
    container.appendChild(statusEl);

    const barTrack = document.createElement('div');
    barTrack.className = 'progress-bar-track';
    const barFill = document.createElement('div');
    barFill.className = 'progress-bar-fill';
    barFill.id = 'progress-fill';
    barTrack.appendChild(barFill);
    container.appendChild(barTrack);

    const textEl = document.createElement('div');
    textEl.className = 'progress-text';
    textEl.id = 'progress-text';
    container.appendChild(textEl);

    resultDiv.appendChild(container);

    pollTimer = setInterval(async function() {
        // 60 second timeout
        if (Date.now() - pollStartTime > 60000) {
            clearInterval(pollTimer);
            pollTimer = null;
            isUpdating = false;
            resultDiv.innerHTML = '';
            const errorDiv = document.createElement('div');
            errorDiv.className = 'update-error';
            errorDiv.textContent = '更新超时，请检查服务状态';
            resultDiv.appendChild(errorDiv);
            document.getElementById('btn-start-update').disabled = false;
            document.getElementById('btn-check-update').disabled = false;
            return;
        }

        try {
            const resp = await fetch('/api/v1/self-update/check', {
                headers: { 'Authorization': 'Bearer ' + authToken }
            });
            if (!resp.ok) {
                // Network error during poll — might be restarting
                return;
            }
            const data = await resp.json();
            const progress = data.progress;

            const currentStatusEl = document.getElementById('progress-status');
            const currentFillEl = document.getElementById('progress-fill');
            const currentTextEl = document.getElementById('progress-text');

            if (!progress || progress.stage === 'idle') {
                if (currentStatusEl) currentStatusEl.textContent = '检查中...';
            } else if (progress.stage === 'checking') {
                if (currentStatusEl) currentStatusEl.textContent = '检查中...';
            } else if (progress.stage === 'downloading') {
                if (currentStatusEl) currentStatusEl.textContent = '下载中 ' + progress.download_percent + '%';
                if (currentFillEl) currentFillEl.style.width = progress.download_percent + '%';
                if (currentTextEl) currentTextEl.textContent = progress.download_percent + '%';
            } else if (progress.stage === 'installing') {
                if (currentStatusEl) currentStatusEl.textContent = '安装中...';
                if (currentFillEl) currentFillEl.style.width = '100%';
            } else if (progress.stage === 'complete') {
                clearInterval(pollTimer);
                pollTimer = null;
                isUpdating = false;
                resultDiv.innerHTML = '';
                const successDiv = document.createElement('div');
                successDiv.className = 'update-success';
                successDiv.textContent = '更新完成，服务即将重启';
                resultDiv.appendChild(successDiv);
                // Reload page after 3 seconds
                setTimeout(function() { location.reload(); }, 3000);
            } else if (progress.stage === 'failed') {
                clearInterval(pollTimer);
                pollTimer = null;
                isUpdating = false;
                const errorMsg = progress.error || '未知错误';
                resultDiv.innerHTML = '';
                const errorDiv = document.createElement('div');
                errorDiv.className = 'update-error';
                errorDiv.textContent = '更新失败: ' + errorMsg;
                resultDiv.appendChild(errorDiv);
                document.getElementById('btn-start-update').disabled = false;
                document.getElementById('btn-check-update').disabled = false;
            }
        } catch (e) {
            // Network error — service might be restarting
            console.log('Poll request failed (service may be restarting):', e.message);
        }
    }, 500);
}
