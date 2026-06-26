const API_BASE = '/api';
let currentPage = 'dashboard';
let isGuest = false;

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
    const token = localStorage.getItem('auth_token');
    const options = {
        method,
        headers: { 
            'Content-Type': 'application/json',
            ...(token && { 'Authorization': `Bearer ${token}` })
        }
    };
    if (body) options.body = JSON.stringify(body);
    
    const response = await fetch(API_BASE + endpoint, options);
    if (response.status === 401) {
        if (isGuest) {
            const error = await response.json();
            throw new Error(error.error || 'unauthorized');
        }
        localStorage.removeItem('auth_token');
        window.location.href = '/login.html';
        return;
    }
    if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Request failed');
    }
    return response.json();
}

async function loadDashboard() {
    try {
        const status = await apiCall('/status');
        isGuest = !!status.guest;

        const statusEl = document.getElementById('engineStatus');
        statusEl.textContent = status.running ? 'Running' : 'Stopped';
        statusEl.className = `status-badge ${status.running ? 'success' : 'danger'}`;

        const guestIndicator = document.getElementById('guestIndicator');
        if (guestIndicator) {
            guestIndicator.style.display = isGuest ? 'inline-block' : 'none';
        }

        const triggerBtn = document.getElementById('triggerBtn');
        if (triggerBtn) triggerBtn.style.display = isGuest ? 'none' : '';

        const lastProbeEl = document.getElementById('lastProbeTime');
        if (status.last_probe_time) {
            lastProbeEl.textContent = new Date(status.last_probe_time).toLocaleString();
        } else {
            lastProbeEl.textContent = 'Never';
        }

        const stats = await apiCall('/stats?hours=24');
        renderModelCards(stats);
        renderRecentActivity(stats);
    } catch (error) {
        console.error('Failed to load dashboard:', error);
    }
}

function renderModelCards(stats) {
    const grid = document.getElementById('modelCards');

    if (stats.length === 0) {
        grid.innerHTML = '<div class="empty-state">No statistics yet</div>';
        return;
    }

    let html = '';
    stats.forEach(ps => {
        ps.models.forEach(ms => {
            const icon = getStatusIcon(ms.last_status, ms.last_tps);
            const rate = ms.success_rate;
            const rateColor = rate >= 99 ? '#10b981' : (rate >= 95 ? '#f59e0b' : '#ef4444');
            const tpsColor = ms.avg_tps >= 10 ? '#10b981' : (ms.avg_tps >= 1 ? '#f59e0b' : '#ef4444');
            const clickAttr = isGuest ? '' : `onclick="showProbeDetails(${ms.probe_id || 0}, '${escapeHtml(ms.provider_name)}', '${escapeHtml(ms.model)}')"`;

            html += `
                <div class="model-card" ${clickAttr}>
                    <div class="mc-header">
                        <span class="mc-icon">${icon}</span>
                        <div class="mc-title">
                            <div class="mc-provider">${escapeHtml(ms.provider_name)}</div>
                            <div class="mc-model">${escapeHtml(ms.model)}</div>
                        </div>
                    </div>
                    <div class="mc-body">
                        <div class="mc-metric">
                            <div class="mc-label">Uptime</div>
                            <div class="mc-bar-bg"><div class="mc-bar" style="width:${Math.min(rate,100)}%;background:${rateColor}"></div></div>
                            <div class="mc-value" style="color:${rateColor}">${rate.toFixed(1)}%</div>
                        </div>
                        <div class="mc-metric">
                            <div class="mc-label">Avg TPS</div>
                            <div class="mc-value" style="color:${tpsColor};font-size:1.5em">${ms.avg_tps.toFixed(1)}</div>
                        </div>
                        <div class="mc-sub">
                            <span>${ms.total_probes} probes</span>
                            <span>${ms.avg_latency_ms.toFixed(0)}ms</span>
                            ${ms.avg_ttft_ms > 0 ? `<span>TTFT ${ms.avg_ttft_ms.toFixed(0)}ms</span>` : ''}
                        </div>
                        <div class="timeline-blocks" id="tl-${ms.probe_id}"></div>
                        <div class="daily-summary" id="ds-${ms.probe_id}"></div>
                    </div>
                </div>
            `;
            // load timeline data async
            if (ms.probe_id) { loadTimeline(ms.probe_id); loadDailySummary(ms.probe_id); }
        });
    });

    grid.innerHTML = html;
}

async function loadTimeline(probeId) {
    try {
        const summary = await apiCall(`/probes/${probeId}/hourly?hours=24`);
        const container = document.getElementById(`tl-${probeId}`);
        if (!container || !summary || summary.length === 0) return;
        
        let html = '';
        summary.forEach(h => {
            let cls = 'tb-gray';
            if (h.total > 0) {
                const failRate = h.failed / h.total;
                if (failRate === 0) cls = 'tb-green';
                else if (failRate < 0.5) cls = 'tb-yellow';
                else cls = 'tb-red';
            }
            html += `<div class="timeline-block ${cls}" title="${h.hour}: ${h.total} probes, ${h.failed} failed"></div>`;
        });
        container.innerHTML = html;
    } catch (e) { /* silently ignore */ }
}

async function loadDailySummary(probeId) {
    try {
        const summary = await apiCall(`/probes/${probeId}/daily?days=7`);
        const container = document.getElementById(`ds-${probeId}`);
        if (!container || !summary || summary.length === 0) return;
        
        let html = '';
        summary.forEach(d => {
            const rate = d.success;
            const color = rate >= 99 ? '#10b981' : (rate >= 95 ? '#f59e0b' : '#ef4444');
            const date = d.date.substring(5); // "MM-DD"
            html += `<div class="ds-day" title="${d.date}: ${rate.toFixed(1)}%">
                <div class="ds-bar" style="height:${Math.max(rate, 5)}%;background:${color}"></div>
                <div class="ds-label">${date}</div>
            </div>`;
        });
        container.innerHTML = html;
    } catch (e) { /* silently ignore */ }
}

function renderRecentActivity(stats) {
    const container = document.getElementById('recentActivity');
    
    if (stats.length === 0) {
        container.innerHTML = '<div class="empty-state">No recent activity</div>';
        return;
    }
    
    let html = '<table class="data-table"><thead><tr><th></th><th>Provider</th><th>Model</th><th>Status</th></tr></thead><tbody>';
    
    stats.forEach(ps => {
        ps.models.forEach(ms => {
            const icon = getStatusIcon(ms.last_status, ms.last_tps);
            const rate = ms.success_rate;
            html += `
                <tr>
                    <td style="text-align:center;font-size:1.2em">${icon}</td>
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

        const addBtn = document.getElementById('addProviderBtn');
        if (addBtn) addBtn.style.display = isGuest ? 'none' : '';

        if (providers.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="empty-state">No providers configured</td></tr>';
            return;
        }

        if (isGuest) {
            tbody.innerHTML = providers.map(p => `
                <tr>
                    <td>${escapeHtml(p.name)}</td>
                    <td>${p.api_type}</td>
                    <td>${p.max_tokens || 2}</td>
                    <td><span class="status-badge ${p.enabled ? 'success' : 'neutral'}">${p.enabled ? 'Active' : 'Disabled'}</span></td>
                </tr>
            `).join('');
        } else {
            tbody.innerHTML = providers.map(p => `
                <tr>
                    <td>${escapeHtml(p.name)}</td>
                    <td>${escapeHtml(p.base_url)}</td>
                    <td>${p.api_type}</td>
                    <td>${p.max_tokens || 2}</td>
                    <td><span class="status-badge ${p.enabled ? 'success' : 'neutral'}">${p.enabled ? 'Active' : 'Disabled'}</span></td>
                    <td>
                        <button class="btn btn-sm btn-secondary" onclick="editProvider(${p.id})">Edit</button>
                        <button class="btn btn-sm btn-secondary" onclick="fetchModels(${p.id})">Fetch Models</button>
                        <button class="btn btn-sm btn-danger" onclick="deleteProvider(${p.id})">Delete</button>
                    </td>
                </tr>
            `).join('');
        }
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
            <div class="form-group">
                <label>Max Tokens</label>
                <input type="number" name="max_tokens" value="2" min="1" placeholder="2">
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
                api_type: formData.get('api_type'),
                max_tokens: parseInt(formData.get('max_tokens')) || 2
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

async function editProvider(id) {
    try {
        const providers = await apiCall('/providers');
        const provider = providers.find(p => p.id === id);
        if (!provider) {
            showToast('Provider not found', 'error');
            return;
        }
        
        document.getElementById('modalTitle').textContent = 'Edit Provider';
        document.getElementById('modalBody').innerHTML = `
            <form id="editProviderForm">
                <div class="form-group">
                    <label>Name</label>
                    <input type="text" name="name" required value="${escapeHtml(provider.name)}">
                </div>
                <div class="form-group">
                    <label>Base URL</label>
                    <input type="url" name="base_url" required value="${escapeHtml(provider.base_url)}">
                </div>
                <div class="form-group">
                    <label>API Key</label>
                    <input type="password" name="api_key" required value="${escapeHtml(provider.api_key)}">
                </div>
                <div class="form-group">
                    <label>API Type</label>
                    <select name="api_type">
                        <option value="openai" ${provider.api_type === 'openai' ? 'selected' : ''}>OpenAI Compatible</option>
                        <option value="anthropic" ${provider.api_type === 'anthropic' ? 'selected' : ''}>Anthropic</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>Max Tokens</label>
                    <input type="number" name="max_tokens" min="1" value="${provider.max_tokens || 2}">
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" name="enabled" ${provider.enabled ? 'checked' : ''}>
                        Enabled
                    </label>
                </div>
                <div class="form-actions">
                    <button type="button" class="btn btn-secondary" onclick="hideModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Save</button>
                </div>
            </form>
        `;
        
        document.getElementById('editProviderForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            const formData = new FormData(e.target);
            try {
                await apiCall(`/providers/${id}`, 'PUT', {
                    name: formData.get('name'),
                    base_url: formData.get('base_url'),
                    api_key: formData.get('api_key'),
                    api_type: formData.get('api_type'),
                    max_tokens: parseInt(formData.get('max_tokens')) || 2,
                    enabled: formData.has('enabled')
                });
                hideModal();
                loadProviders();
                showToast('Provider updated successfully', 'success');
            } catch (error) {
                showToast(error.message, 'error');
            }
        });
        
        showModal();
    } catch (error) {
        showToast('Failed to load provider', 'error');
    }
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

        const addBtn = document.getElementById('addModelBtn');
        if (addBtn) addBtn.style.display = isGuest ? 'none' : '';

        if (probes.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="empty-state">No models configured</td></tr>';
            return;
        }

        if (isGuest) {
            tbody.innerHTML = probes.map(p => `
                <tr>
                    <td>${escapeHtml(p.provider_name)}</td>
                    <td>${escapeHtml(p.model)}</td>
                    <td><span class="status-badge ${p.enabled ? 'success' : 'neutral'}">${p.enabled ? 'Active' : 'Disabled'}</span></td>
                </tr>
            `).join('');
        } else {
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
        }
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

        const clearBtn = document.getElementById('clearStatsBtn');
        if (clearBtn) clearBtn.style.display = isGuest ? 'none' : '';
        const exportBtn = document.getElementById('exportCsvBtn');
        if (exportBtn) exportBtn.style.display = isGuest ? 'none' : '';

        if (stats.length === 0) {
            tbody.innerHTML = '<tr><td colspan="9" class="empty-state">No statistics available</td></tr>';
            return;
        }

        let html = '';
        stats.forEach(ps => {
            ps.models.forEach(ms => {
                const icon = getStatusIcon(ms.last_status, ms.last_tps);
                const tpsClass = ms.avg_tps >= 10 ? 'rate-good' : (ms.avg_tps >= 1 ? 'rate-warning' : 'rate-bad');
                const clickAttr = isGuest ? '' : `onclick="showProbeDetails(${ms.probe_id || 0}, '${escapeHtml(ms.provider_name)}', '${escapeHtml(ms.model)}')"` ;
                const cursorStyle = isGuest ? '' : 'style="cursor: pointer;"';
                html += `
                    <tr>
                        <td style="text-align:center;font-size:1.2em" ${clickAttr}>${icon}</td>
                        <td ${cursorStyle} ${clickAttr}>${escapeHtml(ms.provider_name)}</td>
                        <td ${cursorStyle} ${clickAttr}>${escapeHtml(ms.model)}</td>
                        <td ${cursorStyle} ${clickAttr}>${ms.total_probes}</td>
                        <td ${cursorStyle} ${clickAttr}><span class="${getRateClass(ms.success_rate)}">${ms.success_rate.toFixed(1)}%</span></td>
                        <td ${cursorStyle} ${clickAttr}>${ms.avg_latency_ms.toFixed(0)}ms</td>
                        <td ${cursorStyle} ${clickAttr}>${ms.avg_ttft_ms > 0 ? ms.avg_ttft_ms.toFixed(0) + 'ms' : '-'}</td>
                        <td ${cursorStyle} ${clickAttr}><span class="${tpsClass}">${ms.avg_tps.toFixed(2)}</span></td>
                        ${isGuest ? '' : `<td><button class="btn btn-sm btn-danger" onclick="event.stopPropagation(); deleteModel(${ms.probe_id || 0}, '${escapeHtml(ms.provider_name)}', '${escapeHtml(ms.model)}')">Delete</button></td>`}
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

async function clearStats() {
    if (!confirm('Are you sure you want to clear all statistics? This action cannot be undone.')) {
        return;
    }
    
    try {
        await apiCall('/stats', 'DELETE');
        loadStats();
        showToast('Statistics cleared', 'success');
    } catch (error) {
        showToast('Failed to clear statistics', 'error');
    }
}

async function deleteModel(probeId, providerName, model) {
    if (!probeId) {
        showToast('No probe ID available', 'error');
        return;
    }
    
    if (!confirm(`Delete model '${model}' from ${providerName}? This will remove the probe and all its history.`)) {
        return;
    }
    
    try {
        await apiCall(`/probes/${probeId}`, 'DELETE');
        loadStats();
        showToast('Model deleted', 'success');
    } catch (error) {
        showToast('Failed to delete model', 'error');
    }
}

async function exportCSV() {
    const hours = document.getElementById('timeRange').value;
    const token = localStorage.getItem('auth_token');
    
    try {
        const response = await fetch(`${API_BASE}/export/csv?hours=${hours}`, {
            headers: {
                'Content-Type': 'application/json',
                ...(token && { 'Authorization': `Bearer ${token}` })
            }
        });
        
        if (response.status === 401) {
            localStorage.removeItem('auth_token');
            window.location.href = '/login.html';
            return;
        }
        
        if (!response.ok) {
            throw new Error('Export failed');
        }
        
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `uptime_report_${new Date().toISOString().slice(0,10)}.csv`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        window.URL.revokeObjectURL(url);
        showToast('CSV exported successfully', 'success');
    } catch (error) {
        showToast('Failed to export CSV', 'error');
    }
}

async function showProbeDetails(probeId, providerName, model, statusFilter = '', page = 1) {
    if (!probeId) {
        showToast('No probe ID available', 'error');
        return;
    }

    try {
        let url = `/probes/${probeId}/results?limit=20&page=${page}`;
        if (statusFilter) {
            url += `&status=${encodeURIComponent(statusFilter)}`;
        }
        const data = await apiCall(url);
        const results = data.results || data;
        const total = data.total || 0;
        const totalPages = data.pages || 1;

        document.getElementById('modalTitle').textContent = `${providerName} - ${model}`;

        let content = '<div>';
        content += '<div style="margin-bottom: 12px; display: flex; gap: 8px;">';
        content += `<button class="btn btn-sm ${statusFilter === '' ? 'btn-primary' : 'btn-secondary'}" onclick="showProbeDetails(${probeId}, '${escapeHtml(providerName)}', '${escapeHtml(model)}', '', 1)">All</button>`;
        content += `<button class="btn btn-sm ${statusFilter === 'failed' ? 'btn-danger' : 'btn-secondary'}" onclick="showProbeDetails(${probeId}, '${escapeHtml(providerName)}', '${escapeHtml(model)}', 'failed', 1)">Failed Only</button>`;
        content += '</div>';
        content += '<div style="max-height: 55vh; overflow-y: auto;">';
        content += '<table class="data-table"><thead><tr>';
        content += '<th>Time</th><th>Status</th><th>Latency</th><th>TTFT</th><th>TPS</th><th>Request ID</th><th>Error</th><th style="width:60px"></th>';
        content += '</tr></thead><tbody>';

        if (results && results.length > 0) {
            results.forEach(r => {
                const statusClass = r.status === 'success' ? 'rate-good' : 'rate-bad';
                const time = new Date(r.created_at).toLocaleString();
                content += `<tr>
                    <td>${time}</td>
                    <td><span class="${statusClass}">${r.status}</span></td>
                    <td>${r.latency_ms}ms</td>
                    <td>${r.ttft_ms > 0 ? r.ttft_ms + 'ms' : '-'}</td>
                    <td>${r.tps.toFixed(2)}</td>
                    <td style="max-width:120px;overflow:hidden;text-overflow:ellipsis">${r.request_id || '-'}</td>
                    <td style="max-width:200px;overflow:hidden;text-overflow:ellipsis">${r.error_message || '-'}</td>
                    <td><button class="btn btn-sm btn-danger" onclick="event.stopPropagation();deleteResult(${r.id}, ${probeId}, '${escapeHtml(providerName)}', '${escapeHtml(model)}', '${statusFilter}')">Del</button></td>
                </tr>`;
            });
        } else {
            content += '<tr><td colspan="8" class="empty-state">No results found</td></tr>';
        }

        content += '</tbody></table></div>';

        // pagination
        if (totalPages > 1) {
            content += '<div class="pagination">';
            content += `<button class="btn btn-sm btn-secondary" onclick="showProbeDetails(${probeId}, '${escapeHtml(providerName)}', '${escapeHtml(model)}', '${statusFilter}', ${page - 1})" ${page <= 1 ? 'disabled' : ''}>Prev</button>`;
            content += `<span class="muted">${page} / ${totalPages}</span>`;
            content += `<button class="btn btn-sm btn-secondary" onclick="showProbeDetails(${probeId}, '${escapeHtml(providerName)}', '${escapeHtml(model)}', '${statusFilter}', ${page + 1})" ${page >= totalPages ? 'disabled' : ''}>Next</button>`;
            content += '</div>';
        }

        content += '</div>';
        document.getElementById('modalBody').innerHTML = content;
        showModal();
    } catch (error) {
        showToast('Failed to load probe details', 'error');
    }
}

async function deleteResult(resultId, probeId, providerName, model, statusFilter) {
    if (!confirm('Delete this record? This cannot be undone.')) {
        return;
    }
    
    try {
        await apiCall(`/results/${resultId}`, 'DELETE');
        showToast('Record deleted', 'success');
        // Refresh the details view
        showProbeDetails(probeId, providerName, model, statusFilter);
    } catch (error) {
        showToast('Failed to delete record', 'error');
    }
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

function getStatusIcon(status, tps) {
    if (status === 'success') {
        return tps >= 1 ? '🟢' : '🟡';
    }
    return '🔴';
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
