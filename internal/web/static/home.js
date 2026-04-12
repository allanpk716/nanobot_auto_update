// DOM elements
const instancesGrid = document.getElementById('instances-grid');

// Auth token (shared with self-update module)
let authToken = '';
let pollTimer = null;
let pollStartTime = 0;
let isUpdating = false;

// Get Bearer token, fetch and cache if not already available
async function getToken() {
    if (authToken) return authToken;
    try {
        const resp = await fetch('/api/v1/web-config');
        if (!resp.ok) throw new Error('web-config unavailable');
        const data = await resp.json();
        authToken = data.auth_token;
        return authToken;
    } catch (e) {
        console.error('Failed to get auth token:', e);
        return null;
    }
}

// Toast notification system
function showToast(message, type) {
    // type: "success" or "error"
    const toastContainer = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    toast.textContent = message;
    toastContainer.insertBefore(toast, toastContainer.firstChild);
    setTimeout(function() {
        toast.classList.add('toast-fade-out');
        setTimeout(function() {
            if (toast.parentNode) toast.parentNode.removeChild(toast);
        }, 300);
    }, 3000);
}

// Modal system
function showModal(title, bodyHtml, footerHtml) {
    var container = document.getElementById('modal-container');
    document.getElementById('modal-title').textContent = title;
    document.getElementById('modal-body').innerHTML = bodyHtml || '';
    document.getElementById('modal-footer').innerHTML = footerHtml || '';
    container.style.display = 'flex';
    return container;
}

function closeModal() {
    document.getElementById('modal-container').style.display = 'none';
}

// Validate instance form fields, returns true if valid
function validateInstanceForm() {
    var valid = true;
    var nameInput = document.getElementById('field-name');
    var portInput = document.getElementById('field-port');
    var cmdInput = document.getElementById('field-start-command');
    var timeoutInput = document.getElementById('field-startup-timeout');

    // Clear previous errors
    document.querySelectorAll('.field-error').forEach(function(el) { el.textContent = ''; });

    // Name validation
    var name = nameInput.value.trim();
    if (!name) {
        document.getElementById('error-name').textContent = '名称不能为空';
        valid = false;
    } else if (name.length > 64) {
        document.getElementById('error-name').textContent = '名称长度不能超过64个字符';
        valid = false;
    }

    // Port validation
    var port = parseInt(portInput.value, 10);
    if (isNaN(port) || port < 1 || port > 65535) {
        document.getElementById('error-port').textContent = '端口必须在1-65535之间';
        valid = false;
    }

    // Start command validation
    if (!cmdInput.value.trim()) {
        document.getElementById('error-start-command').textContent = '启动命令不能为空';
        valid = false;
    }

    // Startup timeout validation
    var timeout = parseInt(timeoutInput.value, 10);
    if (isNaN(timeout) || (timeout > 0 && timeout < 5)) {
        document.getElementById('error-startup-timeout').textContent = '启动超时不能小于5秒';
        valid = false;
    }

    return valid;
}

// Display server validation errors (422) in form
function displayServerFieldErrors(errors) {
    if (!errors || !errors.length) return;
    errors.forEach(function(err) {
        var errorEl = document.getElementById('error-' + err.field);
        if (errorEl) {
            errorEl.textContent = err.message;
        }
    });
}

// Build instance form HTML for create/edit dialogs
function buildInstanceFormHtml(options) {
    // options: { nameValue, nameReadOnly, portValue, cmdValue, timeoutValue, autoStartValue }
    var nameReadonlyAttr = options.nameReadOnly ? ' readonly' : '';
    var autoStartActive = (options.autoStartValue === null || options.autoStartValue === undefined || options.autoStartValue === true);

    return '<div class="form-grid">' +
        '<div class="form-group">' +
            '<label for="field-name">名称</label>' +
            '<input type="text" id="field-name" value="' + escapeAttr(options.nameValue || '') + '"' + nameReadonlyAttr + ' required>' +
            '<span class="field-error" id="error-name"></span>' +
        '</div>' +
        '<div class="form-group">' +
            '<label for="field-port">端口</label>' +
            '<input type="number" id="field-port" value="' + (options.portValue || '') + '" min="1" max="65535" required>' +
            '<span class="field-error" id="error-port"></span>' +
        '</div>' +
        '<div class="form-group full-width">' +
            '<label for="field-start-command">启动命令</label>' +
            '<input type="text" id="field-start-command" value="' + escapeAttr(options.cmdValue || '') + '" required>' +
            '<span class="field-error" id="error-start-command"></span>' +
        '</div>' +
        '<div class="form-group">' +
            '<label for="field-startup-timeout">启动超时(秒)</label>' +
            '<input type="number" id="field-startup-timeout" value="' + (options.timeoutValue || 30) + '" min="5">' +
            '<span class="field-error" id="error-startup-timeout"></span>' +
        '</div>' +
        '<div class="form-group">' +
            '<label>自动启动</label>' +
            '<div class="toggle-container">' +
                '<div class="toggle-switch' + (autoStartActive ? ' active' : '') + '" id="toggle-auto-start"></div>' +
                '<span class="toggle-label" id="toggle-auto-start-label">' + (autoStartActive ? '开启' : '关闭') + '</span>' +
            '</div>' +
        '</div>' +
    '</div>';
}

// Escape HTML attribute value
function escapeAttr(str) {
    return str.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// Show create instance dialog
function showCreateDialog() {
    var formHtml = buildInstanceFormHtml({
        nameValue: '',
        nameReadOnly: false,
        portValue: '',
        cmdValue: '',
        timeoutValue: 30,
        autoStartValue: null
    });
    var footerHtml = '<button class="btn-form-cancel" onclick="closeModal()">取消</button>' +
        '<button class="btn-form-primary" id="btn-submit-form">创建</button>';
    showModal('新建实例', formHtml, footerHtml);

    // Toggle switch handler
    var toggleEl = document.getElementById('toggle-auto-start');
    var toggleLabel = document.getElementById('toggle-auto-start-label');
    toggleEl.addEventListener('click', function() {
        toggleEl.classList.toggle('active');
        toggleLabel.textContent = toggleEl.classList.contains('active') ? '开启' : '关闭';
    });

    // Submit button handler
    document.getElementById('btn-submit-form').addEventListener('click', async function() {
        if (!validateInstanceForm()) return;

        var submitBtn = this;
        submitBtn.disabled = true;
        submitBtn.textContent = '创建中...';

        var toggleEl = document.getElementById('toggle-auto-start');
        var autoStart = toggleEl.classList.contains('active');

        var body = {
            name: document.getElementById('field-name').value.trim(),
            port: parseInt(document.getElementById('field-port').value, 10),
            start_command: document.getElementById('field-start-command').value.trim(),
            startup_timeout: parseInt(document.getElementById('field-startup-timeout').value, 10) || 30,
            auto_start: autoStart
        };

        try {
            var token = await getToken();
            if (!token) {
                showToast('获取认证令牌失败', 'error');
                submitBtn.disabled = false;
                submitBtn.textContent = '创建';
                return;
            }
            var resp = await fetch('/api/v1/instance-configs', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': 'Bearer ' + token
                },
                body: JSON.stringify(body)
            });
            var data = await resp.json();

            if (resp.ok) {
                closeModal();
                showToast('实例 ' + body.name + ' 创建成功', 'success');
                loadInstances();
            } else if (resp.status === 422 && data.errors) {
                displayServerFieldErrors(data.errors);
                submitBtn.disabled = false;
                submitBtn.textContent = '创建';
            } else {
                showToast('创建实例失败: ' + (data.message || data.error || '未知错误'), 'error');
                submitBtn.disabled = false;
                submitBtn.textContent = '创建';
            }
        } catch (e) {
            showToast('创建实例失败: ' + e.message, 'error');
            submitBtn.disabled = false;
            submitBtn.textContent = '创建';
        }
    });
}

// Show edit instance dialog
function showEditDialog(instanceName) {
    // Fetch current config first
    getToken().then(function(token) {
        if (!token) {
            showToast('获取认证令牌失败', 'error');
            return;
        }
        return fetch('/api/v1/instance-configs/' + encodeURIComponent(instanceName), {
            headers: { 'Authorization': 'Bearer ' + token }
        });
    }).then(function(resp) {
        if (!resp || !resp.ok) {
            throw new Error('获取实例配置失败');
        }
        return resp.json();
    }).then(function(cfg) {
        if (!cfg) return;

        var formHtml = buildInstanceFormHtml({
            nameValue: cfg.name,
            nameReadOnly: true,
            portValue: cfg.port,
            cmdValue: cfg.start_command,
            timeoutValue: cfg.startup_timeout,
            autoStartValue: cfg.auto_start
        });
        var footerHtml = '<button class="btn-form-cancel" onclick="closeModal()">取消</button>' +
            '<button class="btn-form-primary" id="btn-submit-form">保存更改</button>';
        showModal('编辑实例 - ' + cfg.name, formHtml, footerHtml);

        // Toggle switch handler
        var toggleEl = document.getElementById('toggle-auto-start');
        var toggleLabel = document.getElementById('toggle-auto-start-label');
        toggleEl.addEventListener('click', function() {
            toggleEl.classList.toggle('active');
            toggleLabel.textContent = toggleEl.classList.contains('active') ? '开启' : '关闭';
        });

        // Submit button handler
        document.getElementById('btn-submit-form').addEventListener('click', async function() {
            if (!validateInstanceForm()) return;

            var submitBtn = this;
            submitBtn.disabled = true;
            submitBtn.textContent = '保存中...';

            var toggleEl = document.getElementById('toggle-auto-start');
            var autoStart = toggleEl.classList.contains('active');

            var body = {
                name: cfg.name,
                port: parseInt(document.getElementById('field-port').value, 10),
                start_command: document.getElementById('field-start-command').value.trim(),
                startup_timeout: parseInt(document.getElementById('field-startup-timeout').value, 10) || 30,
                auto_start: autoStart
            };

            try {
                var token = await getToken();
                if (!token) {
                    showToast('获取认证令牌失败', 'error');
                    submitBtn.disabled = false;
                    submitBtn.textContent = '保存更改';
                    return;
                }
                var resp = await fetch('/api/v1/instance-configs/' + encodeURIComponent(cfg.name), {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + token
                    },
                    body: JSON.stringify(body)
                });
                var data = await resp.json();

                if (resp.ok) {
                    closeModal();
                    showToast('实例 ' + cfg.name + ' 更新成功', 'success');
                    loadInstances();
                } else if (resp.status === 422 && data.errors) {
                    displayServerFieldErrors(data.errors);
                    submitBtn.disabled = false;
                    submitBtn.textContent = '保存更改';
                } else {
                    showToast('更新实例失败: ' + (data.message || data.error || '未知错误'), 'error');
                    submitBtn.disabled = false;
                    submitBtn.textContent = '保存更改';
                }
            } catch (e) {
                showToast('更新实例失败: ' + e.message, 'error');
                submitBtn.disabled = false;
                submitBtn.textContent = '保存更改';
            }
        });
    }).catch(function(e) {
        showToast('获取实例配置失败: ' + e.message, 'error');
    });
}

// Show copy instance dialog
function showCopyDialog(sourceName) {
    // Fetch source instance config
    getToken().then(function(token) {
        if (!token) {
            showToast('获取认证令牌失败', 'error');
            return;
        }
        return fetch('/api/v1/instance-configs/' + encodeURIComponent(sourceName), {
            headers: { 'Authorization': 'Bearer ' + token }
        });
    }).then(function(resp) {
        if (!resp || !resp.ok) {
            throw new Error('获取实例配置失败');
        }
        return resp.json();
    }).then(function(cfg) {
        if (!cfg) return;

        var suggestedPort = cfg.port + 1;
        if (suggestedPort > 65535) suggestedPort = cfg.port;

        var formHtml = '<div class="source-info-box">源实例: ' + escapeAttr(sourceName) + '</div>' +
            buildInstanceFormHtml({
                nameValue: sourceName + '-copy',
                nameReadOnly: false,
                portValue: suggestedPort,
                cmdValue: cfg.start_command,
                timeoutValue: cfg.startup_timeout,
                autoStartValue: cfg.auto_start
            });
        var footerHtml = '<button class="btn-form-cancel" onclick="closeModal()">取消</button>' +
            '<button class="btn-form-primary" id="btn-submit-form">复制实例</button>';
        showModal('复制实例', formHtml, footerHtml);

        // Toggle switch handler
        var toggleEl = document.getElementById('toggle-auto-start');
        var toggleLabel = document.getElementById('toggle-auto-start-label');
        toggleEl.addEventListener('click', function() {
            toggleEl.classList.toggle('active');
            toggleLabel.textContent = toggleEl.classList.contains('active') ? '开启' : '关闭';
        });

        // Submit button handler
        document.getElementById('btn-submit-form').addEventListener('click', async function() {
            if (!validateInstanceForm()) return;

            var submitBtn = this;
            submitBtn.disabled = true;
            submitBtn.textContent = '复制中...';

            var toggleEl = document.getElementById('toggle-auto-start');
            var autoStart = toggleEl.classList.contains('active');
            var newName = document.getElementById('field-name').value.trim();

            var body = {
                name: newName,
                port: parseInt(document.getElementById('field-port').value, 10),
                start_command: document.getElementById('field-start-command').value.trim(),
                startup_timeout: parseInt(document.getElementById('field-startup-timeout').value, 10) || 30,
                auto_start: autoStart
            };

            try {
                var token = await getToken();
                if (!token) {
                    showToast('获取认证令牌失败', 'error');
                    submitBtn.disabled = false;
                    submitBtn.textContent = '复制实例';
                    return;
                }
                var resp = await fetch('/api/v1/instance-configs/' + encodeURIComponent(sourceName) + '/copy', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + token
                    },
                    body: JSON.stringify(body)
                });
                var data = await resp.json();

                if (resp.ok) {
                    closeModal();
                    showToast('实例已复制为 ' + newName, 'success');
                    loadInstances();
                } else if (resp.status === 422 && data.errors) {
                    displayServerFieldErrors(data.errors);
                    submitBtn.disabled = false;
                    submitBtn.textContent = '复制实例';
                } else {
                    showToast('复制实例失败: ' + (data.message || data.error || '未知错误'), 'error');
                    submitBtn.disabled = false;
                    submitBtn.textContent = '复制实例';
                }
            } catch (e) {
                showToast('复制实例失败: ' + e.message, 'error');
                submitBtn.disabled = false;
                submitBtn.textContent = '复制实例';
            }
        });
    }).catch(function(e) {
        showToast('获取实例配置失败: ' + e.message, 'error');
    });
}

// Show delete instance confirmation dialog
function showDeleteDialog(instanceName, isRunning) {
    var warningHtml = '';
    if (isRunning) {
        warningHtml = '<div class="delete-warning">警告: 该实例正在运行中，删除前将自动停止该实例。</div>';
    }

    // Fetch instance config for details
    getToken().then(function(token) {
        if (!token) {
            showToast('获取认证令牌失败', 'error');
            return;
        }
        return fetch('/api/v1/instance-configs/' + encodeURIComponent(instanceName), {
            headers: { 'Authorization': 'Bearer ' + token }
        });
    }).then(function(resp) {
        if (!resp || !resp.ok) {
            throw new Error('获取实例配置失败');
        }
        return resp.json();
    }).then(function(cfg) {
        if (!cfg) return;

        var mainText = document.createElement('div');
        mainText.style.marginBottom = 'var(--spacing-md)';
        mainText.style.fontSize = '16px';
        var questionText = document.createElement('span');
        questionText.textContent = '确定要删除实例 "' + instanceName + '" 吗？';
        mainText.appendChild(questionText);

        var infoBox = document.createElement('div');
        infoBox.className = 'delete-info-box';
        var portInfo = document.createElement('div');
        portInfo.textContent = '端口: ' + cfg.port;
        infoBox.appendChild(portInfo);
        var cmdInfo = document.createElement('div');
        cmdInfo.textContent = '命令: ' + cfg.start_command;
        infoBox.appendChild(cmdInfo);

        var bodyContainer = document.createElement('div');
        if (warningHtml) {
            var warningDiv = document.createElement('div');
            warningDiv.className = 'delete-warning';
            warningDiv.textContent = '警告: 该实例正在运行中，删除前将自动停止该实例。';
            bodyContainer.appendChild(warningDiv);
        }
        bodyContainer.appendChild(mainText);
        bodyContainer.appendChild(infoBox);

        var bodyHtml = bodyContainer.innerHTML;

        var footerHtml = '<button class="btn-form-cancel" onclick="closeModal()">取消</button>' +
            '<button class="btn-form-danger" id="btn-confirm-delete">删除</button>';
        showModal('删除实例', bodyHtml, footerHtml);

        // Delete button handler
        document.getElementById('btn-confirm-delete').addEventListener('click', async function() {
            var deleteBtn = this;
            deleteBtn.disabled = true;
            deleteBtn.textContent = '删除中...';

            try {
                var token = await getToken();
                if (!token) {
                    showToast('获取认证令牌失败', 'error');
                    deleteBtn.disabled = false;
                    deleteBtn.textContent = '删除';
                    return;
                }
                var resp = await fetch('/api/v1/instance-configs/' + encodeURIComponent(instanceName), {
                    method: 'DELETE',
                    headers: { 'Authorization': 'Bearer ' + token }
                });

                if (resp.ok) {
                    closeModal();
                    showToast('实例 ' + instanceName + ' 已删除', 'success');
                    loadInstances();
                } else {
                    var data = await resp.json().catch(function() { return {}; });
                    showToast('删除失败: ' + (data.message || data.error || '未知错误'), 'error');
                    deleteBtn.disabled = false;
                    deleteBtn.textContent = '删除';
                }
            } catch (e) {
                showToast('删除失败: ' + e.message, 'error');
                deleteBtn.disabled = false;
                deleteBtn.textContent = '删除';
            }
        });
    }).catch(function(e) {
        showToast('获取实例配置失败: ' + e.message, 'error');
    });
}

async function showNanobotConfigDialog(instanceName) {
    // Helper: get nested value from object by dot-separated path
    function getNestedValue(obj, path) {
        var keys = path.split('.');
        var current = obj;
        for (var i = 0; i < keys.length; i++) {
            if (current === null || current === undefined) return undefined;
            current = current[keys[i]];
        }
        return current;
    }

    // Helper: set nested value in object, creating intermediate objects if needed
    function setNestedValue(obj, path, value) {
        var keys = path.split('.');
        var current = obj;
        for (var i = 0; i < keys.length - 1; i++) {
            if (current[keys[i]] === null || current[keys[i]] === undefined || typeof current[keys[i]] !== 'object') {
                current[keys[i]] = {};
            }
            current = current[keys[i]];
        }
        current[keys[keys.length - 1]] = value;
    }

    // Helper: find first provider with a non-empty apiKey
    function findFirstApiKey(providers) {
        if (!providers || typeof providers !== 'object') return { providerName: null, apiKey: '' };
        var keys = Object.keys(providers);
        for (var i = 0; i < keys.length; i++) {
            var p = providers[keys[i]];
            if (p && p.apiKey && typeof p.apiKey === 'string' && p.apiKey.trim() !== '') {
                return { providerName: keys[i], apiKey: p.apiKey };
            }
        }
        return { providerName: null, apiKey: '' };
    }

    // Step 1: Load current nanobot config
    try {
        var token = await getToken();
        if (!token) {
            showToast('获取认证令牌失败', 'error');
            return;
        }
        var resp = await fetch('/api/v1/instances/' + encodeURIComponent(instanceName) + '/nanobot-config', {
            headers: { 'Authorization': 'Bearer ' + token }
        });
        if (!resp.ok) {
            showToast('加载 ' + instanceName + ' 配置失败', 'error');
            return;
        }
        var data = await resp.json();
        var currentConfig = data.config;

        // Step 2: Build modal body HTML with hybrid editor layout
        var bodyHtml = '<div class="hybrid-editor">' +
            '<div class="hybrid-editor-left">' +
                '<div class="nanobot-section-label">常用参数</div>' +
                '<div class="form-group">' +
                    '<label>模型</label>' +
                    '<input type="text" id="nb-model" class="nb-field" data-path="agents.defaults.model">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>提供商</label>' +
                    '<input type="text" id="nb-provider" class="nb-field" data-path="agents.defaults.provider">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>API 密钥</label>' +
                    '<div class="api-key-wrapper">' +
                        '<input type="password" id="nb-apikey" placeholder="输入 API 密钥...">' +
                        '<button type="button" class="api-key-toggle" id="nb-apikey-toggle">显示</button>' +
                    '</div>' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>网关端口</label>' +
                    '<input type="number" id="nb-port" class="nb-field" data-path="gateway.port" min="1" max="65535">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Telegram Token</label>' +
                    '<input type="text" id="nb-telegram-token" placeholder="输入 Telegram Bot Token...">' +
                '</div>' +
            '</div>' +
            '<div class="hybrid-editor-right">' +
                '<label>完整 JSON 配置</label>' +
                '<textarea id="nb-json" class="nanobot-json-textarea"></textarea>' +
                '<div id="nb-json-error" class="json-error"></div>' +
            '</div>' +
        '</div>';

        // Step 3: Footer with cancel and save buttons
        var footerHtml = '<button class="btn-form-cancel" onclick="closeModal()">取消</button>' +
            '<button class="btn-form-primary" id="btn-save-nanobot-config">保存配置</button>';

        // Step 4: Show modal and add modal-wide class
        showModal('Nanobot 配置 - ' + instanceName, bodyHtml, footerHtml);
        var dialog = document.querySelector('.modal-dialog');
        if (dialog) dialog.classList.add('modal-wide');

        // Step 5: Populate initial values from loaded config
        var modelVal = getNestedValue(currentConfig, 'agents.defaults.model') || '';
        var providerVal = getNestedValue(currentConfig, 'agents.defaults.provider') || '';
        var portVal = getNestedValue(currentConfig, 'gateway.port') || '';
        var telegramVal = getNestedValue(currentConfig, 'channels.telegram.token') || '';
        var apiResult = findFirstApiKey(getNestedValue(currentConfig, 'providers'));

        document.getElementById('nb-model').value = modelVal;
        document.getElementById('nb-provider').value = providerVal;
        document.getElementById('nb-apikey').value = apiResult.apiKey;
        document.getElementById('nb-port').value = portVal;
        document.getElementById('nb-telegram-token').value = telegramVal;
        document.getElementById('nb-json').value = JSON.stringify(currentConfig, null, 2);

        // Step 6: API key show/hide toggle
        var apiKeyInput = document.getElementById('nb-apikey');
        var apiKeyToggle = document.getElementById('nb-apikey-toggle');
        apiKeyToggle.addEventListener('click', function() {
            if (apiKeyInput.type === 'password') {
                apiKeyInput.type = 'text';
                apiKeyToggle.textContent = '隐藏';
            } else {
                apiKeyInput.type = 'password';
                apiKeyToggle.textContent = '显示';
            }
        });

        // Step 7: Bidirectional sync with syncGuard
        var syncGuard = false;

        // Form -> JSON sync: attach 'input' listeners to all form fields
        var formFields = ['nb-model', 'nb-provider', 'nb-apikey', 'nb-port', 'nb-telegram-token'];
        formFields.forEach(function(fieldId) {
            document.getElementById(fieldId).addEventListener('input', function() {
                if (syncGuard) return;

                var jsonStr = document.getElementById('nb-json').value;
                try {
                    var jsonObj = JSON.parse(jsonStr);

                    // Update corresponding path in JSON object
                    if (fieldId === 'nb-model') {
                        setNestedValue(jsonObj, 'agents.defaults.model', this.value);
                    } else if (fieldId === 'nb-provider') {
                        setNestedValue(jsonObj, 'agents.defaults.provider', this.value);
                    } else if (fieldId === 'nb-apikey') {
                        // Update the apiKey of the current provider
                        var currentProvider = document.getElementById('nb-provider').value.trim();
                        var providers = getNestedValue(jsonObj, 'providers');
                        if (providers && currentProvider && providers[currentProvider]) {
                            providers[currentProvider].apiKey = this.value;
                        } else if (apiResult.providerName && providers && providers[apiResult.providerName]) {
                            providers[apiResult.providerName].apiKey = this.value;
                        } else if (providers) {
                            // Create provider entry if needed
                            if (currentProvider) {
                                providers[currentProvider] = providers[currentProvider] || {};
                                providers[currentProvider].apiKey = this.value;
                            }
                        }
                    } else if (fieldId === 'nb-port') {
                        var portNum = parseInt(this.value, 10);
                        setNestedValue(jsonObj, 'gateway.port', isNaN(portNum) ? this.value : portNum);
                    } else if (fieldId === 'nb-telegram-token') {
                        setNestedValue(jsonObj, 'channels.telegram.token', this.value);
                    }

                    syncGuard = true;
                    document.getElementById('nb-json').value = JSON.stringify(jsonObj, null, 2);
                    syncGuard = false;
                    document.getElementById('nb-json-error').textContent = '';
                } catch (e) {
                    // JSON parse error - don't update textarea
                }
            });
        });

        // JSON -> Form sync: attach 'input' listener to textarea
        document.getElementById('nb-json').addEventListener('input', function() {
            if (syncGuard) return;

            var jsonStr = this.value;
            try {
                var jsonObj = JSON.parse(jsonStr);
                document.getElementById('nb-json-error').textContent = '';

                syncGuard = true;
                document.getElementById('nb-model').value = getNestedValue(jsonObj, 'agents.defaults.model') || '';
                document.getElementById('nb-provider').value = getNestedValue(jsonObj, 'agents.defaults.provider') || '';
                document.getElementById('nb-port').value = getNestedValue(jsonObj, 'gateway.port') || '';

                // API key: find first non-empty apiKey from providers
                var apiKeyResult = findFirstApiKey(getNestedValue(jsonObj, 'providers'));
                document.getElementById('nb-apikey').value = apiKeyResult.apiKey;

                document.getElementById('nb-telegram-token').value = getNestedValue(jsonObj, 'channels.telegram.token') || '';
                syncGuard = false;
            } catch (e) {
                document.getElementById('nb-json-error').textContent = 'JSON 格式错误: ' + e.message;
            }
        });

        // Step 8: Save handler
        document.getElementById('btn-save-nanobot-config').addEventListener('click', async function() {
            var saveBtn = this;
            var jsonStr = document.getElementById('nb-json').value;
            var jsonObj;
            try {
                jsonObj = JSON.parse(jsonStr);
            } catch (e) {
                document.getElementById('nb-json-error').textContent = 'JSON 格式错误: ' + e.message;
                return;
            }

            saveBtn.disabled = true;
            saveBtn.textContent = '保存中...';

            try {
                var tkn = await getToken();
                if (!tkn) {
                    showToast('获取认证令牌失败', 'error');
                    saveBtn.disabled = false;
                    saveBtn.textContent = '保存配置';
                    return;
                }
                var saveResp = await fetch('/api/v1/instances/' + encodeURIComponent(instanceName) + '/nanobot-config', {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + tkn
                    },
                    body: JSON.stringify(jsonObj)
                });

                if (saveResp.ok) {
                    closeModal();
                    showToast('配置已保存。重启实例以应用更改。', 'success');
                } else {
                    var errData = await saveResp.json().catch(function() { return {}; });
                    showToast('保存配置失败: ' + (errData.message || errData.error || '未知错误'), 'error');
                    saveBtn.disabled = false;
                    saveBtn.textContent = '保存配置';
                }
            } catch (e) {
                showToast('保存配置失败: ' + e.message, 'error');
                saveBtn.disabled = false;
                saveBtn.textContent = '保存配置';
            }
        });

    } catch (e) {
        showToast('加载 ' + instanceName + ' 配置失败: ' + e.message, 'error');
    }
}

// Load instance configs and status from APIs
async function loadInstances() {
    try {
        var results = await Promise.allSettled([
            fetch('/api/v1/instances/status').then(function(r) { return r.json(); }),
            getToken().then(function(token) {
                if (!token) throw new Error('no token');
                return fetch('/api/v1/instance-configs', {
                    headers: { 'Authorization': 'Bearer ' + token }
                }).then(function(r) {
                    if (!r.ok) throw new Error('HTTP ' + r.status);
                    return r.json();
                });
            })
        ]);

        var statusResult = results[0];
        var configResult = results[1];

        var statusMap = {};
        var statusOk = statusResult.status === 'fulfilled';
        var configOk = configResult.status === 'fulfilled';

        if (statusOk && statusResult.value && statusResult.value.instances) {
            statusResult.value.instances.forEach(function(inst) {
                statusMap[inst.name] = inst.running;
            });
        }

        instancesGrid.innerHTML = '';

        // Both rejected - error state
        if (!statusOk && !configOk) {
            instancesGrid.innerHTML = '<div class="empty-state"><h2>加载失败</h2><p>无法获取实例列表，请检查服务器状态</p></div>';
            return;
        }

        // Configs available - render full cards
        if (configOk && configResult.value && configResult.value.instances) {
            var configs = configResult.value.instances;
            if (configs.length === 0) {
                instancesGrid.innerHTML = '<div class="empty-state"><h2>无实例配置</h2><p>无实例配置，点击「+ 新建实例」创建第一个实例。</p></div>';
                return;
            }
            configs.forEach(function(cfg) {
                var isRunning = statusOk ? (statusMap[cfg.name] || false) : null;
                var card = createInstanceCard(cfg, isRunning);
                instancesGrid.appendChild(card);
            });
            return;
        }

        // Status only (auth failed) - render status-only cards
        if (statusOk && statusResult.value && statusResult.value.instances) {
            var statuses = statusResult.value.instances;
            if (statuses.length === 0) {
                instancesGrid.innerHTML = '<div class="empty-state"><h2>无实例配置</h2><p>请在配置文件中添加实例</p></div>';
                return;
            }
            statuses.forEach(function(inst) {
                var card = createInstanceCard({ name: inst.name, port: inst.port }, inst.running);
                // Disable secondary buttons when auth failed
                var secondaryBtns = card.querySelectorAll('.btn-secondary');
                secondaryBtns.forEach(function(btn) { btn.disabled = true; });
                instancesGrid.appendChild(card);
            });
            return;
        }

        instancesGrid.innerHTML = '<div class="empty-state"><h2>加载失败</h2><p>无法获取实例列表，请检查服务器状态</p></div>';
    } catch (error) {
        console.error('Failed to load instance status:', error);
        instancesGrid.innerHTML = '<div class="empty-state"><h2>加载失败</h2><p>无法获取实例列表，请检查服务器状态</p></div>';
    }
}

// Create instance card element
function createInstanceCard(config, isRunning) {
    var card = document.createElement('div');
    card.className = 'instance-card';

    // Card header area: name link + status indicator
    var headerDiv = document.createElement('div');
    headerDiv.style.display = 'flex';
    headerDiv.style.alignItems = 'center';
    headerDiv.style.gap = 'var(--spacing-xs)';
    headerDiv.style.marginBottom = 'var(--spacing-sm)';

    var nameLink = document.createElement('a');
    nameLink.href = '/logs/' + encodeURIComponent(config.name);
    nameLink.className = 'instance-name';
    nameLink.textContent = config.name;
    headerDiv.appendChild(nameLink);

    // Status dot indicator
    if (isRunning !== null && isRunning !== undefined) {
        var statusDot = document.createElement('span');
        statusDot.className = isRunning ? 'status-dot status-dot-running' : 'status-dot status-dot-stopped';
        headerDiv.appendChild(statusDot);

        var statusSpan = document.createElement('span');
        statusSpan.className = isRunning ? 'status-running' : 'status-stopped';
        statusSpan.textContent = isRunning ? '运行中' : '已停止';
        headerDiv.appendChild(statusSpan);
    }

    card.appendChild(headerDiv);

    // Config info section
    var infoDiv = document.createElement('div');
    infoDiv.className = 'instance-info';

    // Port row
    if (config.port !== undefined) {
        var portRow = document.createElement('div');
        portRow.className = 'info-row';
        var portLabel = document.createElement('span');
        portLabel.className = 'label';
        portLabel.textContent = '端口:';
        var portValue = document.createElement('span');
        portValue.className = 'value';
        portValue.textContent = config.port;
        portRow.appendChild(portLabel);
        portRow.appendChild(portValue);
        infoDiv.appendChild(portRow);
    }

    // Start command row
    if (config.start_command) {
        var cmdRow = document.createElement('div');
        cmdRow.className = 'info-row';
        var cmdLabel = document.createElement('span');
        cmdLabel.className = 'label';
        cmdLabel.textContent = '命令:';
        var cmdValue = document.createElement('span');
        cmdValue.className = 'value command-text';
        if (config.start_command.length > 40) {
            cmdValue.textContent = config.start_command.substring(0, 40) + '...';
            cmdValue.title = config.start_command;
        } else {
            cmdValue.textContent = config.start_command;
        }
        cmdRow.appendChild(cmdLabel);
        cmdRow.appendChild(cmdValue);
        infoDiv.appendChild(cmdRow);
    }

    // Startup timeout row
    if (config.startup_timeout !== undefined) {
        var timeoutRow = document.createElement('div');
        timeoutRow.className = 'info-row';
        var timeoutLabel = document.createElement('span');
        timeoutLabel.className = 'label';
        timeoutLabel.textContent = '启动超时:';
        var timeoutValue = document.createElement('span');
        timeoutValue.className = 'value';
        timeoutValue.textContent = config.startup_timeout + '秒';
        timeoutRow.appendChild(timeoutLabel);
        timeoutRow.appendChild(timeoutValue);
        infoDiv.appendChild(timeoutRow);
    }

    // Auto-start row
    if (config.auto_start !== undefined) {
        var autoRow = document.createElement('div');
        autoRow.className = 'info-row';
        var autoLabel = document.createElement('span');
        autoLabel.className = 'label';
        autoLabel.textContent = '自动启动:';
        var autoValue = document.createElement('span');
        autoValue.className = 'value';
        if (config.auto_start === null || config.auto_start === undefined) {
            var tag = document.createElement('span');
            tag.className = 'auto-start-tag auto-start-default';
            tag.textContent = '默认';
            autoValue.appendChild(tag);
        } else if (config.auto_start === true) {
            var tagYes = document.createElement('span');
            tagYes.className = 'auto-start-tag auto-start-yes';
            tagYes.textContent = '是';
            autoValue.appendChild(tagYes);
        } else {
            var tagNo = document.createElement('span');
            tagNo.className = 'auto-start-tag auto-start-no';
            tagNo.textContent = '否';
            autoValue.appendChild(tagNo);
        }
        autoRow.appendChild(autoLabel);
        autoRow.appendChild(autoValue);
        infoDiv.appendChild(autoRow);
    }

    card.appendChild(infoDiv);

    // Primary action area (start/stop)
    var primaryDiv = document.createElement('div');
    primaryDiv.className = 'card-actions-primary';

    if (isRunning !== null && isRunning !== undefined) {
        var actionBtn = document.createElement('button');
        if (isRunning) {
            actionBtn.className = 'btn-action btn-stop';
            actionBtn.textContent = '停止';
            actionBtn.addEventListener('click', function() {
                handleLifecycleAction(config.name, 'stop', actionBtn);
            });
        } else {
            actionBtn.className = 'btn-action btn-start';
            actionBtn.textContent = '启动';
            actionBtn.addEventListener('click', function() {
                handleLifecycleAction(config.name, 'start', actionBtn);
            });
        }
        primaryDiv.appendChild(actionBtn);
    }

    card.appendChild(primaryDiv);

    // Secondary action row
    var secondaryDiv = document.createElement('div');
    secondaryDiv.className = 'card-actions-secondary';

    var editBtn = document.createElement('button');
    editBtn.className = 'btn-secondary';
    editBtn.textContent = '编辑';
    editBtn.addEventListener('click', function() { showEditDialog(config.name); });
    secondaryDiv.appendChild(editBtn);

    var copyBtn = document.createElement('button');
    copyBtn.className = 'btn-secondary';
    copyBtn.textContent = '复制';
    copyBtn.addEventListener('click', function() { showCopyDialog(config.name); });
    secondaryDiv.appendChild(copyBtn);

    var deleteBtn = document.createElement('button');
    deleteBtn.className = 'btn-secondary btn-delete-danger';
    deleteBtn.textContent = '删除';
    deleteBtn.addEventListener('click', function() { showDeleteDialog(config.name, isRunning); });
    secondaryDiv.appendChild(deleteBtn);

    var configBtn = document.createElement('button');
    configBtn.className = 'btn-secondary';
    configBtn.textContent = '配置';
    configBtn.addEventListener('click', function() { showNanobotConfigDialog(config.name); });
    secondaryDiv.appendChild(configBtn);

    card.appendChild(secondaryDiv);

    return card;
}

// Handle lifecycle actions (start/stop) with loading state and timeout
async function handleLifecycleAction(instanceName, action, button) {
    var originalText = button.textContent;
    var originalClass = button.className;
    var loadingText = action === 'start' ? '启动中...' : '停止中...';

    // Set loading state
    button.disabled = true;
    button.classList.add('loading');
    button.textContent = loadingText;

    // AbortController with timeout
    var timeout = action === 'start' ? 65000 : 35000;
    var controller = new AbortController();
    var timeoutId = setTimeout(function() { controller.abort(); }, timeout);

    try {
        var token = await getToken();
        if (!token) {
            showToast('获取认证令牌失败', 'error');
            return;
        }

        var response = await fetch('/api/v1/instances/' + encodeURIComponent(instanceName) + '/' + action, {
            method: 'POST',
            headers: { 'Authorization': 'Bearer ' + token },
            signal: controller.signal
        });

        var data = await response.json();

        if (response.ok) {
            var actionLabel = action === 'start' ? '启动' : '停止';
            showToast('实例 ' + instanceName + ' 已' + actionLabel + '成功', 'success');
            loadInstances();
        } else {
            showToast(actionLabel + '实例 ' + instanceName + ' 失败: ' + (data.message || data.error || '未知错误'), 'error');
        }
    } catch (error) {
        if (error.name === 'AbortError') {
            showToast('操作超时，请稍后查看实例状态', 'error');
        } else {
            var failLabel = action === 'start' ? '启动' : '停止';
            showToast(failLabel + '实例 ' + instanceName + ' 失败: ' + error.message, 'error');
        }
    } finally {
        clearTimeout(timeoutId);
        button.classList.remove('loading');
        button.disabled = false;
        button.textContent = originalText;
    }
}

// Initialize on DOMContentLoaded
document.addEventListener('DOMContentLoaded', function() {
    // Load instances
    loadInstances();

    // Auto-refresh every 5 seconds
    setInterval(loadInstances, 5000);

    // Load version into header badge
    loadHeaderVersion();

    // Initialize self-update module
    initSelfUpdate();

    // New instance button
    document.getElementById('btn-new-instance').addEventListener('click', function() {
        showCreateDialog();
    });

    // Modal close handlers
    document.getElementById('modal-close').addEventListener('click', closeModal);
    document.getElementById('modal-container').addEventListener('click', function(e) {
        if (e.target.id === 'modal-container') closeModal();
    });

    // Escape key to close modal
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') closeModal();
    });
});

// Load version for header badge (no auth required)
async function loadHeaderVersion() {
    try {
        var resp = await fetch('/api/v1/version');
        if (resp.ok) {
            var data = await resp.json();
            var badge = document.getElementById('header-version');
            if (badge && data.version) {
                badge.textContent = 'v' + data.version.replace(/^v/, '');
            }
        }
    } catch (e) {
        // Non-critical, ignore
    }
}

// Self-update module: Initialize
async function initSelfUpdate() {
    try {
        var resp = await fetch('/api/v1/web-config');
        if (!resp.ok) {
            throw new Error('web-config unavailable');
        }
        var data = await resp.json();
        authToken = data.auth_token;
        // Load current version from check API
        await loadCurrentVersion();
        // Bind button events
        document.getElementById('btn-check-update').addEventListener('click', checkUpdate);
        document.getElementById('btn-start-update').addEventListener('click', startUpdate);
    } catch (e) {
        // web-config fetch failed (non-localhost)
        console.error('Failed to init self-update:', e);
        var section = document.getElementById('selfupdate-section');
        section.innerHTML = '<p class="selfupdate-warning">请在本地访问以使用自更新功能</p>';
    }
}

// Load current version from version API (no auth required)
async function loadCurrentVersion() {
    try {
        var resp = await fetch('/api/v1/version');
        if (!resp.ok) {
            throw new Error('version API failed');
        }
        var data = await resp.json();
        document.getElementById('current-version').textContent = data.version;
    } catch (e) {
        console.error('Failed to load current version:', e);
        document.getElementById('current-version').textContent = 'N/A';
    }
}

// Check for updates
async function checkUpdate() {
    var btn = document.getElementById('btn-check-update');
    var startBtn = document.getElementById('btn-start-update');
    var resultDiv = document.getElementById('update-result');

    if (isUpdating) return;

    // Disable button during check
    btn.disabled = true;
    btn.textContent = '检测中...';
    startBtn.disabled = true;

    try {
        var resp = await fetch('/api/v1/self-update/check', {
            headers: { 'Authorization': 'Bearer ' + authToken }
        });
        if (!resp.ok) {
            throw new Error('check API returned ' + resp.status);
        }
        var data = await resp.json();

        if (!data.needs_update) {
            // Already up to date
            resultDiv.className = 'update-result visible';
            resultDiv.innerHTML = '';
            var infoDiv = document.createElement('div');
            infoDiv.className = 'update-info';
            var label = document.createElement('span');
            label.className = 'info-label';
            label.textContent = '已是最新版本';
            infoDiv.appendChild(label);
            var value = document.createElement('span');
            value.className = 'info-value';
            value.textContent = data.current_version;
            infoDiv.appendChild(value);
            resultDiv.appendChild(infoDiv);
            startBtn.disabled = true;
        } else {
            // New version available - render details
            resultDiv.className = 'update-result visible';
            resultDiv.innerHTML = '';

            var infoDiv = document.createElement('div');
            infoDiv.className = 'update-info';

            // Version row
            var versionRow = document.createElement('div');
            versionRow.style.marginBottom = '4px';
            var versionLabel = document.createElement('span');
            versionLabel.className = 'info-label';
            versionLabel.textContent = '最新版本:';
            var versionValue = document.createElement('span');
            versionValue.className = 'info-value';
            versionValue.textContent = data.latest_version;
            versionRow.appendChild(versionLabel);
            versionRow.appendChild(versionValue);
            infoDiv.appendChild(versionRow);

            // Date row
            if (data.published_at) {
                var dateStr = '';
                try {
                    var d = new Date(data.published_at);
                    dateStr = d.getFullYear() + '-' +
                        String(d.getMonth() + 1).padStart(2, '0') + '-' +
                        String(d.getDate()).padStart(2, '0');
                } catch (e) {
                    dateStr = data.published_at;
                }
                var dateRow = document.createElement('div');
                dateRow.style.marginBottom = '4px';
                var dateLabel = document.createElement('span');
                dateLabel.className = 'info-label';
                dateLabel.textContent = '发布日期:';
                var dateValue = document.createElement('span');
                dateValue.className = 'info-value';
                dateValue.textContent = dateStr;
                dateRow.appendChild(dateLabel);
                dateRow.appendChild(dateValue);
                infoDiv.appendChild(dateRow);
            }

            resultDiv.appendChild(infoDiv);

            // Release notes with expand/collapse (textContent for XSS safety)
            if (data.release_notes) {
                var notesDiv = document.createElement('div');
                notesDiv.className = 'release-notes';
                notesDiv.textContent = data.release_notes;
                resultDiv.appendChild(notesDiv);

                var toggleBtn = document.createElement('span');
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
        var errorDiv = document.createElement('div');
        errorDiv.className = 'update-error';
        errorDiv.textContent = '检测更新失败，请查看控制台获取详情';
        resultDiv.appendChild(errorDiv);
        startBtn.disabled = true;
    } finally {
        btn.disabled = false;
        btn.textContent = '检测更新';
    }
}

// Start update
async function startUpdate() {
    var btn = document.getElementById('btn-start-update');
    if (isUpdating || btn.disabled) return;

    try {
        btn.disabled = true;
        document.getElementById('btn-check-update').disabled = true;

        var resp = await fetch('/api/v1/self-update', {
            method: 'POST',
            headers: { 'Authorization': 'Bearer ' + authToken }
        });

        if (resp.status === 409) {
            // Already updating
            var resultDiv = document.getElementById('update-result');
            resultDiv.className = 'update-result visible';
            resultDiv.innerHTML = '';
            var errorDiv = document.createElement('div');
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
        var resultDiv = document.getElementById('update-result');
        resultDiv.className = 'update-result visible';
        resultDiv.innerHTML = '';
        var errorDiv = document.createElement('div');
        errorDiv.className = 'update-error';
        errorDiv.textContent = '启动更新失败，请查看控制台获取详情';
        resultDiv.appendChild(errorDiv);
        btn.disabled = false;
        document.getElementById('btn-check-update').disabled = false;
    }
}

// Progress polling (500ms interval, 60s timeout)
function startProgressPolling() {
    var resultDiv = document.getElementById('update-result');
    resultDiv.className = 'update-result visible';
    resultDiv.innerHTML = '';

    var container = document.createElement('div');
    container.className = 'progress-container';

    var statusEl = document.createElement('div');
    statusEl.className = 'progress-status';
    statusEl.id = 'progress-status';
    statusEl.textContent = '检查中...';
    container.appendChild(statusEl);

    var barTrack = document.createElement('div');
    barTrack.className = 'progress-bar-track';
    var barFill = document.createElement('div');
    barFill.className = 'progress-bar-fill';
    barFill.id = 'progress-fill';
    barTrack.appendChild(barFill);
    container.appendChild(barTrack);

    var textEl = document.createElement('div');
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
            var errorDiv = document.createElement('div');
            errorDiv.className = 'update-error';
            errorDiv.textContent = '更新超时，请检查服务状态';
            resultDiv.appendChild(errorDiv);
            document.getElementById('btn-start-update').disabled = false;
            document.getElementById('btn-check-update').disabled = false;
            return;
        }

        try {
            var resp = await fetch('/api/v1/self-update/check', {
                headers: { 'Authorization': 'Bearer ' + authToken }
            });
            if (!resp.ok) {
                // Network error during poll - might be restarting
                return;
            }
            var data = await resp.json();
            var progress = data.progress;

            var currentStatusEl = document.getElementById('progress-status');
            var currentFillEl = document.getElementById('progress-fill');
            var currentTextEl = document.getElementById('progress-text');

            if (!progress || progress.stage === 'idle') {
                if (currentStatusEl) currentStatusEl.textContent = '检查中...';
            } else if (progress.stage === 'checking') {
                if (currentStatusEl) currentStatusEl.textContent = '检查中...';
            } else if (progress.stage === 'downloading') {
                var pct = Math.max(0, Math.min(100, Number(progress.download_percent) || 0));
                if (currentStatusEl) currentStatusEl.textContent = '下载中 ' + pct + '%';
                if (currentFillEl) currentFillEl.style.width = pct + '%';
                if (currentTextEl) currentTextEl.textContent = pct + '%';
            } else if (progress.stage === 'installing') {
                if (currentStatusEl) currentStatusEl.textContent = '安装中...';
                if (currentFillEl) currentFillEl.style.width = '100%';
            } else if (progress.stage === 'complete') {
                clearInterval(pollTimer);
                pollTimer = null;
                isUpdating = false;
                resultDiv.innerHTML = '';
                var successDiv = document.createElement('div');
                successDiv.className = 'update-success';
                successDiv.textContent = '更新完成，服务即将重启';
                resultDiv.appendChild(successDiv);
                // Reload page after 3 seconds
                setTimeout(function() { location.reload(); }, 3000);
            } else if (progress.stage === 'failed') {
                clearInterval(pollTimer);
                pollTimer = null;
                isUpdating = false;
                var errorMsg = progress.error || '未知错误';
                resultDiv.innerHTML = '';
                var errorDiv = document.createElement('div');
                errorDiv.className = 'update-error';
                errorDiv.textContent = '更新失败: ' + errorMsg;
                resultDiv.appendChild(errorDiv);
                document.getElementById('btn-start-update').disabled = false;
                document.getElementById('btn-check-update').disabled = false;
            }
        } catch (e) {
            // Network error - service might be restarting
            console.log('Poll request failed (service may be restarting):', e.message);
        }
    }, 500);
}
