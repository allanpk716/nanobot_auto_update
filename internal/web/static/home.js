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

    card.innerHTML = `
        <a href="/logs/${instance.name}" class="instance-name">${instance.name}</a>
        <div class="instance-info">
            <div class="info-row">
                <span class="label">端口:</span>
                <span class="value">${instance.port}</span>
            </div>
            <div class="info-row">
                <span class="label">状态:</span>
                <span class="value ${statusClass}">${statusText}</span>
            </div>
        </div>
        <button class="btn-restart" data-instance="${instance.name}">重启实例</button>
    `;

    // Add restart button click handler
    const restartBtn = card.querySelector('.btn-restart');
    restartBtn.addEventListener('click', function() {
        restartInstance(instance.name, restartBtn);
    });

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
});
