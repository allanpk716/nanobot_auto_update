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
    `;

    return card;
}

// Initialize on DOMContentLoaded
document.addEventListener('DOMContentLoaded', function() {
    // Load instances
    loadInstances();

    // Auto-refresh every 5 seconds
    setInterval(loadInstances, 5000);
});
