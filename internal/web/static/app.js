const API_BASE = '/api';
let currentPage = 'dashboard';

document.addEventListener('DOMContentLoaded', () => {
    initNavigation();
    loadDashboard();
    initTriggerButton();
});

function initNavigation() {
    document.querySelectorAll('.nav-item').forEach(item => {
        item.addEventListener('click', () => {
            const page = item.dataset.page;
            navigateTo(page);
        });
    });
}

function navigateTo(page) {
    document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
    document.querySelector(`[data-page="${page}"]`).classList.add('active');
    
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    document.getElementById(page).classList.add('active');
    
    currentPage = page;
    loadPageData(page);
}

function loadPageData(page) {
    switch (page) {
        case 'dashboard': loadDashboard(); break;
        case 'providers': loadProviders(); break;
        case 'models': loadModels(); break;
        case 'stats': loadStats(); break;
    }
}

function initTriggerButton() {
    document.getElementById('triggerBtn').addEventListener('click', async () => {
        try {
            await apiCall('/probe/trigger', 'POST');
            showToast('Probe triggered successfully', 'success');
        } catch (error) {
            showToast('Failed to trigger probe', 'error');
        }
    });
}

async function apiCall(endpoint, method = 'GET', body = null) {
    const options = {
        method,
        headers: { 'Content-Type': 'application/json' }
    };
    if (body) options.body = JSON.stringify(body);
    
    const response = await fetch(API_BASE + endpoint, options);
    if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Request failed');
    }
    return response.json();
}

async function loadDashboard() {
    try {
        const status = await apiCall('/status');
        const statusEl = document.getElementById('engineStatus');
        statusEl.textContent = status.running ? 'Running' : 'Stopped';
        statusEl.className = `status-badge ${status.running ? 'success' : 'danger'}`;
        
        const stats = await apiCall('/stats?hours=24');
        renderStatsGrid(stats);
        
        renderRecentActivity(stats);
    } catch (error) {
        console.error('Failed to load dashboard:', error);
    }
}

function renderStatsGrid(stats) {
    const grid = document.getElementById('statsGrid');
    
    let totalProbes = 0;
    let totalSuccess = 0;
    let avgLatency = 0;
    let modelCount = 0;
    
    stats.forEach(ps => {
        ps.models.forEach(ms => {
            totalProbes += ms.total_probes;
            totalSuccess += ms.success_count;
            if (ms.avg_latency_ms > 0) {
                avgLatency += ms.avg_latency_ms;
                modelCount++;
            }
        });
    });
    
    const successRate = totalProbes > 0 ? (totalSuccess / totalProbes * 100) : 0;
    const avgLatencyMs = modelCount > 0 ? (avgLatency / modelCount) : 0;
    
    grid.innerHTML = `
        <div class="stat-card">
            <div class="label">Total Probes (24h)</div>
            <div class="value">${totalProbes}</div>
        </div>
        <div class="stat-card">
            <div class="label">Success Rate</div>
            <div class="value ${getRateClass(successRate)}">${successRate.toFixed(1)}%</div>
        </div>
        <div class="stat-card">
            <div class="label">Avg Latency</div>
            <div class="value">${avgLatencyMs.toFixed(0)}ms</div>
        </div>
        <div class="stat-card">
            <div class="label">Active Models</div>
            <div class="value">${stats.reduce((acc, ps) => acc + ps.models.length, 0)}</div>
        </div>
    `;
}

function renderRecentActivity(stats) {
    const container = document.getElementById('recentActivity');
    
    if (stats.length === 0) {
        container.innerHTML = '<div class="empty-state">No recent activity</div>';
        return;
    }
    
    let html = '<table class="data-table"><thead><tr><th>Provider</th><th>Model</th><th>Status</th></tr></thead><tbody>';
    
    stats.forEach(ps => {
        ps.models.forEach(ms => {
            const rate = ms.success_rate;
            html += `
                <tr>
                    <td>${ms.provider_name}</td>
                    <td>${ms.model}</td>
                    <td><span class="${getRateClass(rate)}">${rate.toFixed(1)}%</span></td>
                </tr>
            `;
        });
    });
    
    html += '</tbody></table>';
    container.innerHTML = html;
}

async function loadProviders() {
    try {
        const providers = await apiCall('/providers');
        const tbody = document.getElementById('providersTable');
        
        if (providers.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="empty-state">No providers configured</td></tr>';
            return;
        }
        
        tbody.innerHTML = providers.map(p => `
            <tr>
                <td>${escapeHtml(p.name)}</td>
                <td>${escapeHtml(p.base_url)}</td>
                <td>${p.api_type}</td>
                <td><span class="status-badge ${p.enabled ? 'success' : 'neutral'}">${p.enabled ? 'Active' : 'Disabled'}</span></td>
                <td>
                    <button class="btn btn-sm btn-secondary" onclick="fetchModels(${p.id})">Fetch Models</button>
                    <button class="btn btn-sm btn-danger" onclick="deleteProvider(${p.id})">Delete</button>
                </td>
            </tr>
        `).join('');
    } catch (error) {
        showToast('Failed to load providers', 'error');
    }
}

function showAddProvider() {
    document.getElementById('modalTitle').textContent = 'Add Provider';
    document.getElementById('modalBody').innerHTML = `
        <form id="providerForm">
            <div class="form-group">
                <label>Name</label>
                <input type="text" name="name" required placeholder="My Provider">
            </div>
            <div class="form-group">
                <label>Base URL</label>
                <input type="url" name="base_url" required placeholder="https://api.example.com">
            </div>
            <div class="form-group">
                <label>API Key</label>
                <input type="password" name="api_key" required placeholder="sk-...">
            </div>
            <div class="form-group">
                <label>API Type</label>
                <select name="api_type">
                    <option value="openai">OpenAI Compatible</option>
                    <option value="anthropic">Anthropic</option>
                </select>
            </div>
            <div class="form-actions">
                <button type="button" class="btn btn-secondary" onclick="hideModal()">Cancel</button>
                <button type="submit" class="btn btn-primary">Add</button>
            </div>
        </form>
    `;
    
    document.getElementById('providerForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        try {
            await apiCall('/providers', 'POST', {
                name: formData.get('name'),
                base_url: formData.get('base_url'),
                api_key: formData.get('api_key'),
                api_type: formData.get('api_type')
            });
            hideModal();
            loadProviders();
            showToast('Provider added successfully', 'success');
        } catch (error) {
            showToast(error.message, 'error');
        }
    });
    
    showModal();
}

async function deleteProvider(id) {
    if (!confirm('Are you sure you want to delete this provider?')) return;
    
    try {
        await apiCall(`/providers/${id}`, 'DELETE');
        loadProviders();
        showToast('Provider deleted', 'success');
    } catch (error) {
        showToast('Failed to delete provider', 'error');
    }
}

async function fetchModels(providerId) {
    try {
        const result = await apiCall(`/providers/${providerId}/models`);
        
        if (result.error) {
            showToast(result.error, 'error');
            return;
        }
        
        document.getElementById('modalTitle').textContent = 'Available Models';
        document.getElementById('modalBody').innerHTML = `
            <div style="max-height: 400px; overflow-y: auto;">
                ${result.models.length === 0 ? '<p class="empty-state">No models found</p>' : 
                    result.models.map(m => `
                        <div style="padding: 8px; border-bottom: 1px solid var(--border-color); display: flex; justify-content: space-between; align-items: center;">
                            <span>${escapeHtml(m)}</span>
                            <button class="btn btn-sm btn-primary" onclick="addModel(${providerId}, '${escapeHtml(m)}')">Add</button>
                        </div>
                    `).join('')}
            </div>
        `;
        showModal();
    } catch (error) {
        showToast('Failed to fetch models: ' + error.message, 'error');
    }
}

async function addModel(providerId, model) {
    try {
        await apiCall('/probes', 'POST', { provider_id: providerId, model });
        showToast(`Model ${model} added`, 'success');
        if (currentPage === 'models') loadModels();
    } catch (error) {
        showToast('Failed to add model: ' + error.message, 'error');
    }
}

async function loadModels() {
    try {
        const probes = await apiCall('/probes');
        const tbody = document.getElementById('modelsTable');
        
        if (probes.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="empty-state">No models configured</td></tr>';
            return;
        }
        
        tbody.innerHTML = probes.map(p => `
            <tr>
                <td>${escapeHtml(p.provider_name)}</td>
                <td>${escapeHtml(p.model)}</td>
                <td><span class="status-badge ${p.enabled ? 'success' : 'neutral'}">${p.enabled ? 'Active' : 'Disabled'}</span></td>
                <td>
                    <button class="btn btn-sm btn-danger" onclick="deleteProbe(${p.id})">Delete</button>
                </td>
            </tr>
        `).join('');
    } catch (error) {
        showToast('Failed to load models', 'error');
    }
}

function showAddModel() {
    loadProvidersForSelect().then(providers => {
        document.getElementById('modalTitle').textContent = 'Add Model';
        document.getElementById('modalBody').innerHTML = `
            <form id="modelForm">
                <div class="form-group">
                    <label>Provider</label>
                    <select name="provider_id" required>
                        ${providers.map(p => `<option value="${p.id}">${escapeHtml(p.name)}</option>`).join('')}
                    </select>
                </div>
                <div class="form-group">
                    <label>Model Name</label>
                    <input type="text" name="model" required placeholder="gpt-4">
                </div>
                <div class="form-actions">
                    <button type="button" class="btn btn-secondary" onclick="hideModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Add</button>
                </div>
            </form>
        `;
        
        document.getElementById('modelForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            const formData = new FormData(e.target);
            try {
                await apiCall('/probes', 'POST', {
                    provider_id: parseInt(formData.get('provider_id')),
                    model: formData.get('model')
                });
                hideModal();
                loadModels();
                showToast('Model added successfully', 'success');
            } catch (error) {
                showToast(error.message, 'error');
            }
        });
        
        showModal();
    });
}

async function loadProvidersForSelect() {
    try {
        return await apiCall('/providers');
    } catch (error) {
        return [];
    }
}

async function deleteProbe(id) {
    if (!confirm('Are you sure you want to delete this model?')) return;
    
    try {
        await apiCall(`/probes/${id}`, 'DELETE');
        loadModels();
        showToast('Model deleted', 'success');
    } catch (error) {
        showToast('Failed to delete model', 'error');
    }
}

async function loadStats() {
    const hours = document.getElementById('timeRange').value;
    try {
        const stats = await apiCall(`/stats?hours=${hours}`);
        const tbody = document.getElementById('statsTable');
        
        if (stats.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="empty-state">No statistics available</td></tr>';
            return;
        }
        
        let html = '';
        stats.forEach(ps => {
            ps.models.forEach(ms => {
                html += `
                    <tr>
                        <td>${escapeHtml(ms.provider_name)}</td>
                        <td>${escapeHtml(ms.model)}</td>
                        <td>${ms.total_probes}</td>
                        <td><span class="${getRateClass(ms.success_rate)}">${ms.success_rate.toFixed(1)}%</span></td>
                        <td>${ms.avg_latency_ms.toFixed(0)}ms</td>
                    </tr>
                `;
            });
        });
        
        tbody.innerHTML = html;
    } catch (error) {
        showToast('Failed to load statistics', 'error');
    }
}

document.getElementById('timeRange').addEventListener('change', loadStats);

async function exportCSV() {
    const hours = document.getElementById('timeRange').value;
    window.location.href = `${API_BASE}/export/csv?hours=${hours}`;
}

function showModal() {
    document.getElementById('modal').classList.remove('hidden');
}

function hideModal() {
    document.getElementById('modal').classList.add('hidden');
}

function showToast(message, type = 'success') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast ${type}`;
    
    setTimeout(() => {
        toast.classList.add('hidden');
    }, 3000);
}

function getRateClass(rate) {
    if (rate >= 99) return 'rate-good';
    if (rate >= 95) return 'rate-warning';
    return 'rate-bad';
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
