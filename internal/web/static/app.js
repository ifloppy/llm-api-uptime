const API_BASE = '/api';
const DEFAULT_CHART_DAYS = 7;
const CHART_RANGE_OPTIONS = [0, 7, 30];
const CHART_COLORS = ['#2563eb', '#d97706', '#059669', '#7c3aed', '#db2777', '#0891b2', '#9333ea', '#4d7c0f'];
const METRICS = {
    uptime: { label: 'Uptime', field: 'success', unit: '%', decimals: 1 },
    latency: { label: 'Latency', field: 'avg_latency_ms', unit: 'ms', decimals: 0 },
    ttft: { label: 'TTFT', field: 'avg_ttft_ms', unit: 'ms', decimals: 0 }
};

let currentPage = 'dashboard';
let isGuest = true;
let accessKnown = false;
let dashboardStats = [];
let providerRows = [];
let modelRows = [];
let statsRows = [];
let dailyStats = [];
let hourlyStats = [];
let chartLoadError = '';
let chartsHidden = false;
let modalOpener = null;
let modalDirty = false;
let toastTimer = null;
let statsLoadGeneration = 0;
let modalGeneration = 0;
const chartMetrics = new Map();
const collapsedCharts = new Set();
const expandedTimelines = new Set();
const timelineCache = new Map();
const chartPreferences = {
    rangeDays: DEFAULT_CHART_DAYS,
    mode: 'per-point',
    selectedModelByProvider: {}
};
let chartPreferencesLoaded = false;

document.addEventListener('DOMContentLoaded', () => {
    initNavigation();
    initActions();
    initModal();
    initTheme();
    initChartPreferences();
    loadDashboard();
});

function initChartPreferences() {
    if (chartPreferencesLoaded) return;
    let stored = null;
    try {
        stored = JSON.parse(localStorage.getItem('llm_chart_preferences') || 'null');
    } catch (error) { stored = null; }
    if (stored && typeof stored === 'object') {
        if (CHART_RANGE_OPTIONS.includes(Number(stored.rangeDays))) chartPreferences.rangeDays = Number(stored.rangeDays);
        if (stored.mode === 'per-point' || stored.mode === 'per-day') chartPreferences.mode = stored.mode;
        if (stored.selectedModelByProvider && typeof stored.selectedModelByProvider === 'object') chartPreferences.selectedModelByProvider = stored.selectedModelByProvider;
    }
    chartPreferencesLoaded = true;
    applyChartControlsState();
}

function persistChartPreferences() {
    if (!chartPreferencesLoaded) return;
    try {
        localStorage.setItem('llm_chart_preferences', JSON.stringify({
            rangeDays: chartPreferences.rangeDays,
            mode: chartPreferences.mode,
            selectedModelByProvider: chartPreferences.selectedModelByProvider
        }));
    } catch (error) { /* ignore quota errors */ }
}

function applyChartControlsState() {
    const rangeToday = document.getElementById('chartRangeToday');
    const range7 = document.getElementById('chartRange7');
    const range30 = document.getElementById('chartRange30');
    const modePerPoint = document.getElementById('chartModePerPoint');
    const modePerDay = document.getElementById('chartModePerDay');
    const range = chartPreferences.rangeDays;
    if (rangeToday) {
        rangeToday.classList.toggle('active', range === 0);
        rangeToday.setAttribute('aria-pressed', String(range === 0));
    }
    if (range7) {
        range7.classList.toggle('active', range === 7);
        range7.setAttribute('aria-pressed', String(range === 7));
    }
    if (range30) {
        range30.classList.toggle('active', range === 30);
        range30.setAttribute('aria-pressed', String(range === 30));
    }
    if (modePerPoint && modePerDay) {
        const isPerPoint = chartPreferences.mode === 'per-point';
        modePerPoint.classList.toggle('active', isPerPoint);
        modePerPoint.setAttribute('aria-pressed', String(isPerPoint));
        modePerDay.classList.toggle('active', !isPerPoint);
        modePerDay.setAttribute('aria-pressed', String(!isPerPoint));
        modePerDay.textContent = range === 0 ? 'Per hour' : 'Per day';
        modePerPoint.textContent = 'Per point';
    }
    const hint = document.getElementById('chartControlsHint');
    if (hint) {
        if (chartPreferences.mode === 'per-day') {
            hint.textContent = range === 0
                ? 'Hover any hour in the chart to see every model at that hour.'
                : 'Hover any day in the chart to see every model on that date.';
        } else {
            hint.textContent = 'Hover any data point in the chart for a single model\'s value.';
        }
    }
}

function initNavigation() {
    document.querySelectorAll('.nav-item').forEach(button => {
        button.addEventListener('click', () => navigateTo(button.dataset.page));
    });
}

function initActions() {
    document.getElementById('triggerBtn').addEventListener('click', triggerProbe);
    document.getElementById('copyStatusBtn').addEventListener('click', copyCurrentStatus);
    document.getElementById('restartBtn').addEventListener('click', restartUpdate);
    document.getElementById('addProviderBtn').addEventListener('click', showAddProvider);
    document.getElementById('addModelBtn').addEventListener('click', showAddModel);
    document.getElementById('timeRange').addEventListener('change', loadStats);
    document.getElementById('statsCopyStatusBtn').addEventListener('click', copyCurrentStatus);
    document.getElementById('toggleChartsBtn').addEventListener('click', toggleAllCharts);
    document.getElementById('exportCsvBtn').addEventListener('click', exportCSV);
    document.getElementById('clearStatsBtn').addEventListener('click', clearStats);
    document.getElementById('chartRangeToday').addEventListener('click', () => setChartRange(0));
    document.getElementById('chartRange7').addEventListener('click', () => setChartRange(7));
    document.getElementById('chartRange30').addEventListener('click', () => setChartRange(30));
    document.getElementById('chartModePerPoint').addEventListener('click', () => setChartMode('per-point'));
    document.getElementById('chartModePerDay').addEventListener('click', () => setChartMode('per-day'));
    document.getElementById('dashboardProviders').addEventListener('click', handleDashboardAction);
    document.getElementById('providersTable').addEventListener('click', handleProviderAction);
    document.getElementById('modelsTable').addEventListener('click', handleModelAction);
    document.getElementById('statsProviders').addEventListener('click', handleStatsAction);
    document.getElementById('statsProviders').addEventListener('keydown', handleStatsKeydown);
    document.addEventListener('pointerdown', event => {
        if (!event.target.closest('.chart-hit') && !event.target.closest('#chartTooltip')) hideChartTooltip();
    });
}

function setChartRange(days) {
    if (!CHART_RANGE_OPTIONS.includes(days) || chartPreferences.rangeDays === days) return;
    chartPreferences.rangeDays = days;
    chartPreferences.selectedModelByProvider = {};
    persistChartPreferences();
    applyChartControlsState();
    if (currentPage === 'stats') loadStats();
}

function setChartMode(mode) {
    if (mode !== 'per-point' && mode !== 'per-day') return;
    if (chartPreferences.mode === mode) return;
    chartPreferences.mode = mode;
    persistChartPreferences();
    applyChartControlsState();
    if (currentPage === 'stats') renderStats();
}

function navigateTo(page) {
    const target = document.getElementById(page);
    if (!target) return;

    document.querySelectorAll('.nav-item').forEach(button => {
        const active = button.dataset.page === page;
        button.classList.toggle('active', active);
        if (active) button.setAttribute('aria-current', 'page');
        else button.removeAttribute('aria-current');
    });
    document.querySelectorAll('.page').forEach(section => {
        const active = section.id === page;
        section.classList.toggle('active', active);
        section.hidden = !active;
    });

    currentPage = page;
    loadPageData(page);
    document.getElementById('mainContent').focus({ preventScroll: true });
}

function loadPageData(page) {
    if (page === 'dashboard') loadDashboard();
    if (page === 'providers') loadProviders();
    if (page === 'models') loadModels();
    if (page === 'stats') loadStats();
}

async function apiCall(endpoint, method = 'GET', body = null) {
    const token = localStorage.getItem('auth_token');
    const options = {
        method,
        headers: {
            'Content-Type': 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {})
        }
    };
    if (body !== null) options.body = JSON.stringify(body);

    const response = await fetch(API_BASE + endpoint, options);
    let data = null;
    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
        try { data = await response.json(); } catch (error) { data = null; }
    }

    if (response.status === 401) {
        if (isGuest || !token) throw new Error((data && data.error) || 'Authentication required');
        localStorage.removeItem('auth_token');
        window.location.href = '/login.html';
        throw new Error('Session expired');
    }
    if (!response.ok) throw new Error((data && data.error) || `Request failed (${response.status})`);
    return data;
}

async function triggerProbe() {
    const button = document.getElementById('triggerBtn');
    setButtonBusy(button, true, 'Running');
    try {
        await apiCall('/probe/trigger', 'POST');
        showToast('Probe queued successfully', 'success');
        window.setTimeout(() => currentPage === 'dashboard' && loadDashboard(), 800);
    } catch (error) {
        showToast(error.message || 'Failed to trigger probe', 'error');
    } finally {
        setButtonBusy(button, false);
    }
}

async function loadDashboard() {
    const container = document.getElementById('dashboardProviders');
    if (!dashboardStats.length) container.innerHTML = loadingMarkup('Loading service status');
    try {
        const status = await apiCall('/status');
        applyAccessState(status || {});
        dashboardStats = asArray(await apiCall('/stats?hours=24'));
        timelineCache.clear();
        renderDashboard(dashboardStats);
        loadUpdateStatus();
    } catch (error) {
        container.innerHTML = errorMarkup('Service status is unavailable', 'Refresh the page to try again.');
        showToast('Failed to load dashboard', 'error');
    }
}

function applyAccessState(status) {
    const previousGuest = isGuest;
    accessKnown = true;
    isGuest = Boolean(status.guest);
    const running = Boolean(status.running);
    const engine = document.getElementById('engineStatus');
    const dot = document.getElementById('engineDot');
    engine.textContent = running ? 'Running' : 'Stopped';
    dot.className = `state-dot ${running ? 'available' : 'unavailable'}`;
    document.getElementById('lastProbeTime').textContent = status.last_probe_time ? formatDateTime(status.last_probe_time) : 'Never';
    document.getElementById('guestIndicator').textContent = isGuest ? 'Guest' : 'Authenticated';
    document.getElementById('triggerBtn').hidden = isGuest;
    if (previousGuest !== isGuest && currentPage !== 'dashboard') loadPageData(currentPage);
}

function renderDashboard(stats) {
    const container = document.getElementById('dashboardProviders');
    const providers = normalizeProviders(stats);
    const models = providers.flatMap(provider => provider.models);
    document.getElementById('providerCount').textContent = String(providers.length);
    document.getElementById('modelCount').textContent = String(models.length);

    if (!providers.length) {
        container.innerHTML = emptyMarkup('No probe data yet', 'Add a provider and model, then run the first probe.');
        return;
    }

    container.innerHTML = providers.map((provider, providerIndex) => {
        const down = provider.models.filter(model => currentState(model) === 'unavailable').length;
        const unknown = provider.models.filter(model => currentState(model) === 'neutral').length;
        const providerState = down ? `${down} unavailable` : unknown ? `${unknown} unknown` : 'All available';
        const rows = provider.models.map((model, modelIndex) => {
            const state = currentState(model);
            const available = state === 'available';
            const unavailable = state === 'unavailable';
            const error = displayErrorMessage(currentError(model));
            const today = todayUptime(model);
            return `
                <article class="model-row ${unavailable ? 'is-down' : ''}">
                    <div class="model-identity">
                        <div class="identity-row">
                            <span class="state-dot ${state}" aria-hidden="true"></span>
                            <div class="identity-text"><strong>${escapeHtml(model.model || 'Unknown model')}</strong><span>${available ? 'Available' : statusLabel(model.last_status)}</span></div>
                        </div>
                    </div>
                    <div class="model-metric model-metric-uptime"><span>Today uptime</span><strong class="tabular ${today === null ? '' : rateClass(today)}">${formatPercent(today)}</strong></div>
                    <div class="model-metric model-metric-status"><span>Current status</span><strong>${available ? 'Operational' : unavailable ? 'Unavailable' : 'Unknown'}</strong></div>
                    <div class="row-actions">
                        <button class="icon-text-btn" type="button" data-action="copy-model" data-provider-index="${providerIndex}" data-model-index="${modelIndex}">${copyIcon()}<span>Copy model</span></button>
                        ${unavailable && error ? `<button class="icon-text-btn" type="button" data-action="copy-error" data-provider-index="${providerIndex}" data-model-index="${modelIndex}">${copyIcon()}<span>Copy error</span></button>` : ''}
                        ${!isGuest && Number(model.probe_id) ? `<button class="icon-text-btn" type="button" data-action="logs" data-provider-index="${providerIndex}" data-model-index="${modelIndex}">${logsIcon()}<span>Request logs</span></button>` : ''}
                    </div>
                    ${renderErrorPanel(model, error, unavailable, providerIndex, modelIndex)}
                    ${Number(model.probe_id) ? `<div class="timeline-panel dashboard-timeline" id="dashboard-timeline-${providerIndex}-${modelIndex}">${loadingMarkup('Loading uptime timeline')}</div>` : ''}
                </article>`;
        }).join('');

        return `<section class="provider-panel" aria-labelledby="dashboard-provider-${providerIndex}">
            <header class="provider-header">
                <div><p class="provider-label">Provider</p><h2 id="dashboard-provider-${providerIndex}">${escapeHtml(provider.provider_name)}</h2></div>
                <div class="provider-header-actions"><span class="status-pill ${down ? 'danger' : unknown ? 'neutral' : 'success'}">${providerState}</span><button class="btn btn-secondary btn-sm" type="button" data-action="copy-provider" data-provider-index="${providerIndex}">${copyIcon()}Copy report</button></div>
            </header>
            <div class="model-list">${rows}</div>
        </section>`;
    }).join('');

    loadDashboardTimelines(providers);
}

function renderErrorPanel(model, error, unavailable, providerIndex, modelIndex) {
    if (!unavailable) {
        return '';
    }
    const detailsId = `error-detail-${providerIndex}-${modelIndex}`;
    const httpLine = finiteNumber(model.status_code, 0) ? `<dt>HTTP status</dt><dd>${finiteNumber(model.status_code, 0)}</dd>` : '';
    const codeLine = model.error_code ? `<dt>Error code</dt><dd>${escapeHtml(textValue(model.error_code))}</dd>` : '';
    const requestLine = model.request_id ? `<dt>Request ID</dt><dd>${escapeHtml(textValue(model.request_id))}</dd>` : '';
    const checkedLine = model.latest_result_time ? `<dt>Last checked</dt><dd>${escapeHtml(formatDateTime(model.latest_result_time))}</dd>` : '';
    const hasDetails = !!(httpLine || codeLine || requestLine || checkedLine);
    return `<div class="current-error">
        <span>Current error</span>
        <code class="error-text">${escapeHtml(error || 'No error message')}</code>
        <div class="error-actions">
            ${hasDetails ? `<button class="icon-text-btn" type="button" data-action="toggle-error" data-error-target="${detailsId}" aria-expanded="false" aria-controls="${detailsId}">Details</button>` : ''}
            ${!isGuest && Number(model.probe_id) ? `<button class="icon-text-btn" type="button" data-action="open-logs" data-provider-index="${providerIndex}" data-model-index="${modelIndex}">Logs</button>` : ''}
            <button class="icon-text-btn" type="button" data-action="copy-error" data-provider-index="${providerIndex}" data-model-index="${modelIndex}">Copy</button>
        </div>
        ${hasDetails ? `<div class="error-full" id="${detailsId}" hidden><dl>${checkedLine}${httpLine}${codeLine}${requestLine}</dl></div>` : ''}
    </div>`;
}

function toggleErrorDetails(targetId, button) {
    const panel = document.getElementById(targetId);
    if (!panel) return;
    const expanded = !panel.hidden;
    panel.hidden = expanded;
    if (button) {
        button.setAttribute('aria-expanded', String(!expanded));
        button.textContent = expanded ? 'Details' : 'Hide details';
    }
}

function handleDashboardAction(event) {
    const button = event.target.closest('button[data-action]');
    if (!button) return;
    const action = button.dataset.action;
    if (action === 'timeline-logs') {
        showDowntimeLogs(Number(button.dataset.probeId), button.dataset.providerName, button.dataset.modelName, Number(button.dataset.start), Number(button.dataset.end));
        return;
    }
    const provider = normalizeProviders(dashboardStats)[Number(button.dataset.providerIndex)];
    const model = provider && provider.models[Number(button.dataset.modelIndex)];
    if (action === 'copy-provider' && provider) copyText(providerReport(provider), `${provider.provider_name} report copied`);
    if (action === 'copy-model' && model) copyText(String(model.model || ''), 'Model name copied');
    if (action === 'copy-error' && provider && model) copyText(modelReport(provider.provider_name, model), 'Current error details copied');
    if (action === 'open-logs' && provider && model) showProbeDetails(Number(model.probe_id), provider.provider_name, model.model);
    if (action === 'logs' && provider && model) showProbeDetails(Number(model.probe_id), provider.provider_name, model.model);
    if (action === 'toggle-error') toggleErrorDetails(button.dataset.errorTarget, button);
}

function copyCurrentStatus() {
    const providers = normalizeProviders(currentPage === 'stats' && statsRows.length ? statsRows : dashboardStats);
    if (!providers.length) return showToast('No current status to copy', 'error');
    const lines = [`LLM API status - ${new Date().toLocaleString()}`];
    providers.forEach(provider => lines.push('', providerReport(provider)));
    copyText(lines.join('\n'), 'Current status copied');
}

function providerReport(provider) {
    const lines = [`Provider: ${reportValue(provider.provider_name)}`];
    provider.models.forEach(model => lines.push('', modelReport(provider.provider_name, model, false)));
    return lines.join('\n');
}

function modelReport(providerName, model, includeProvider = true) {
    const available = isAvailable(model);
    const lines = [];
    if (includeProvider) lines.push(`Provider: ${reportValue(providerName)}`);
    lines.push(`Model: ${reportValue(model.model)}`);
    lines.push(`Today uptime: ${formatPercent(todayUptime(model))} (${finiteNumber(model.today_success_count, 0)}/${finiteNumber(model.today_total, 0)})`);
    lines.push(`Current status: ${available ? 'Operational' : statusLabel(model.last_status)}`);
    if (model.latest_result_time) lines.push(`Last checked: ${formatDateTime(model.latest_result_time)}`);
    if (!available) {
        if (finiteNumber(model.status_code, 0)) lines.push(`HTTP status: ${finiteNumber(model.status_code, 0)}`);
        if (model.error_code) lines.push(`Error code: ${reportValue(model.error_code)}`);
        if (model.request_id) lines.push(`Request ID: ${reportValue(model.request_id)}`);
        if (currentError(model)) lines.push(`Error: ${reportValue(displayErrorMessage(currentError(model)))}`);
    }
    return lines.join('\n');
}

async function loadUpdateStatus() {
    const panel = document.getElementById('updatePanel');
    try {
        const update = await apiCall('/update');
        if (!update) return;
        panel.classList.remove('hidden');
        document.getElementById('updateCurrent').textContent = textValue(update.current_version ?? update.current ?? '-');
        document.getElementById('updateLatest').textContent = textValue(update.latest_version ?? update.latest ?? '-');
        document.getElementById('updateStatus').textContent = textValue(update.status ?? (update.update_available ? 'Update available' : 'Up to date'));
        const releaseLink = document.getElementById('updateReleaseLink');
        const releaseUrl = safeHttpUrl(update.release_url ?? update.release_link ?? update.url);
        releaseLink.classList.toggle('hidden', !releaseUrl);
        if (releaseUrl) releaseLink.href = releaseUrl;
        const restart = document.getElementById('restartBtn');
        restart.classList.toggle('hidden', isGuest || !update.restart_required || update.restart_allowed === false);
    } catch (error) {
        panel.classList.add('hidden');
    }
}

async function restartUpdate() {
    if (isGuest) return;
    if (!window.confirm('Restart the service now to complete the update?')) return;
    const button = document.getElementById('restartBtn');
    setButtonBusy(button, true, 'Restarting');
    try {
        await apiCall('/update/restart', 'POST');
        showToast('Restart requested. The dashboard will reconnect shortly.', 'success');
        button.disabled = true;
    } catch (error) {
        showToast(error.message || 'Restart request failed', 'error');
        setButtonBusy(button, false);
    }
}

async function loadProviders() {
    const tbody = document.getElementById('providersTable');
    tbody.innerHTML = `<tr><td colspan="6">${loadingMarkup('Loading providers')}</td></tr>`;
    try {
        providerRows = asArray(await apiCall('/providers'));
        document.getElementById('addProviderBtn').hidden = isGuest;
        if (!providerRows.length) {
            tbody.innerHTML = `<tr><td colspan="6">${emptyMarkup('No providers configured', 'Add a provider to begin monitoring.')}</td></tr>`;
            return;
        }
        tbody.innerHTML = providerRows.map((provider, index) => `<tr>
            <td data-label="Name"><strong>${escapeHtml(provider.name)}</strong></td>
            <td data-label="Base URL" class="truncate-cell">${isGuest ? '<span class="muted">Hidden</span>' : escapeHtml(provider.base_url || '-')}</td>
            <td data-label="Type"><span class="code-label">${escapeHtml(provider.api_type || '-')}</span></td>
            <td data-label="Max tokens" class="tabular">${finiteNumber(provider.max_tokens, 2)}</td>
            <td data-label="Status"><span class="status-pill ${provider.enabled ? 'success' : 'neutral'}">${provider.enabled ? 'Active' : 'Disabled'}</span></td>
            <td data-label="Actions"><div class="table-actions">${isGuest ? '<span class="muted">Read only</span>' : `<button class="btn btn-secondary btn-sm" type="button" data-action="edit" data-index="${index}">Edit</button><button class="btn btn-secondary btn-sm" type="button" data-action="fetch" data-index="${index}">Fetch models</button><button class="btn btn-danger-outline btn-sm" type="button" data-action="delete" data-index="${index}">Delete</button>`}</div></td>
        </tr>`).join('');
    } catch (error) {
        tbody.innerHTML = `<tr><td colspan="6">${errorMarkup('Providers could not be loaded', error.message)}</td></tr>`;
    }
}

function handleProviderAction(event) {
    const button = event.target.closest('button[data-action]');
    if (!button) return;
    const provider = providerRows[Number(button.dataset.index)];
    if (!provider) return;
    if (button.dataset.action === 'edit') showProviderForm(provider);
    if (button.dataset.action === 'fetch') fetchModels(provider);
    if (button.dataset.action === 'delete') deleteProvider(provider);
}

function showAddProvider() {
    showProviderForm(null);
}

function showProviderForm(provider) {
    const editing = Boolean(provider);
    setModalContent(editing ? 'Edit provider' : 'Add provider', editing ? 'Provider configuration' : 'New integration', `
        <form id="providerForm">
            <div class="form-grid">
                <div class="form-group"><label for="providerName">Name</label><input id="providerName" type="text" name="name" required autocomplete="off" value="${escapeAttribute(provider && provider.name)}"></div>
                <div class="form-group"><label for="providerType">API type</label><select id="providerType" name="api_type"><option value="openai" ${provider && provider.api_type === 'openai' ? 'selected' : ''}>OpenAI compatible</option><option value="anthropic" ${provider && provider.api_type === 'anthropic' ? 'selected' : ''}>Anthropic</option></select></div>
                <div class="form-group form-span"><label for="providerUrl">Base URL</label><input id="providerUrl" type="url" name="base_url" required autocomplete="url" value="${escapeAttribute(provider && provider.base_url)}" placeholder="https://api.example.com"></div>
                <div class="form-group form-span"><label for="providerKey">API key</label><input id="providerKey" type="password" name="api_key" required autocomplete="off" value="${escapeAttribute(provider && provider.api_key)}"></div>
                <div class="form-group"><label for="providerTokens">Max tokens</label><input id="providerTokens" type="number" name="max_tokens" min="1" value="${finiteNumber(provider && provider.max_tokens, 2)}"></div>
                ${editing ? `<div class="form-group checkbox-group"><label><input type="checkbox" name="enabled" ${provider.enabled ? 'checked' : ''}> Enabled for probing</label></div>` : ''}
            </div>
            <div class="form-actions"><button type="button" class="btn btn-secondary" data-modal-close>Cancel</button><button type="submit" class="btn btn-primary">${editing ? 'Save changes' : 'Add provider'}</button></div>
        </form>`);

    document.getElementById('providerForm').addEventListener('submit', async event => {
        event.preventDefault();
        const button = event.submitter;
        const data = new FormData(event.currentTarget);
        setButtonBusy(button, true, editing ? 'Saving' : 'Adding');
        try {
            const payload = {
                name: data.get('name'), base_url: data.get('base_url'), api_key: data.get('api_key'),
                api_type: data.get('api_type'), max_tokens: parseInt(data.get('max_tokens'), 10) || 2,
                ...(editing ? { enabled: data.has('enabled') } : {})
            };
            await apiCall(editing ? `/providers/${provider.id}` : '/providers', editing ? 'PUT' : 'POST', payload);
            hideModal(true);
            loadProviders();
            showToast(editing ? 'Provider updated' : 'Provider added', 'success');
        } catch (error) {
            showToast(error.message, 'error');
            setButtonBusy(button, false);
        }
    });
    showModal();
}

async function deleteProvider(provider) {
    if (!window.confirm(`Delete provider "${provider.name}" and its configured probes?`)) return;
    try {
        await apiCall(`/providers/${provider.id}`, 'DELETE');
        showToast('Provider deleted', 'success');
        loadProviders();
    } catch (error) { showToast(error.message || 'Failed to delete provider', 'error'); }
}

async function fetchModels(provider) {
    const generation = setModalContent('Available models', provider.name, loadingMarkup('Fetching model catalog'));
    showModal();
    try {
        const result = await apiCall(`/providers/${provider.id}/models`);
        if (generation !== modalGeneration) return;
        const models = asArray(result && result.models);
        const body = document.getElementById('modalBody');
        body.innerHTML = models.length ? `<div class="catalog-list">${models.map((model, index) => `<div><code>${escapeHtml(model)}</code><button class="btn btn-primary btn-sm" type="button" data-catalog-index="${index}">Add</button></div>`).join('')}</div>` : emptyMarkup('No models found', 'The provider returned an empty model catalog.');
        body.querySelectorAll('[data-catalog-index]').forEach(button => button.addEventListener('click', () => addModel(provider.id, models[Number(button.dataset.catalogIndex)], button)));
    } catch (error) {
        if (generation !== modalGeneration) return;
        document.getElementById('modalBody').innerHTML = errorMarkup('Model catalog could not be loaded', error.message);
    }
}

async function addModel(providerId, model, button = null) {
    if (button) setButtonBusy(button, true, 'Adding');
    try {
        await apiCall('/probes', 'POST', { provider_id: Number(providerId), model });
        showToast(`Model ${model} added`, 'success');
        if (button) { button.textContent = 'Added'; button.disabled = true; }
        if (currentPage === 'models') loadModels();
    } catch (error) {
        showToast(error.message || 'Failed to add model', 'error');
        if (button) setButtonBusy(button, false);
    }
}

async function loadModels() {
    const tbody = document.getElementById('modelsTable');
    tbody.innerHTML = `<tr><td colspan="4">${loadingMarkup('Loading models')}</td></tr>`;
    try {
        modelRows = asArray(await apiCall('/probes'));
        document.getElementById('addModelBtn').hidden = isGuest;
        if (!modelRows.length) {
            tbody.innerHTML = `<tr><td colspan="4">${emptyMarkup('No models configured', 'Add a model to include it in the probe schedule.')}</td></tr>`;
            return;
        }
        tbody.innerHTML = modelRows.map((probe, index) => `<tr><td data-label="Provider"><strong>${escapeHtml(probe.provider_name)}</strong></td><td data-label="Model"><code>${escapeHtml(probe.model)}</code></td><td data-label="Status"><span class="status-pill ${probe.enabled ? 'success' : 'neutral'}">${probe.enabled ? 'Active' : 'Disabled'}</span></td><td data-label="Actions">${isGuest ? '<span class="muted">Read only</span>' : `<button class="btn btn-danger-outline btn-sm" type="button" data-action="delete" data-index="${index}">Delete</button>`}</td></tr>`).join('');
    } catch (error) {
        tbody.innerHTML = `<tr><td colspan="4">${errorMarkup('Models could not be loaded', error.message)}</td></tr>`;
    }
}

function handleModelAction(event) {
    const button = event.target.closest('button[data-action="delete"]');
    if (!button) return;
    const probe = modelRows[Number(button.dataset.index)];
    if (probe) deleteProbe(probe);
}

async function showAddModel() {
    let providers;
    try { providers = asArray(await apiCall('/providers')); } catch (error) { return showToast('Failed to load providers', 'error'); }
    if (!providers.length) return showToast('Add a provider first', 'error');
    setModalContent('Add model', 'Probe configuration', `<form id="modelForm"><div class="form-group"><label for="modelProvider">Provider</label><select id="modelProvider" name="provider_id" required>${providers.map(provider => `<option value="${Number(provider.id)}">${escapeHtml(provider.name)}</option>`).join('')}</select></div><div class="form-group"><label for="modelName">Model name</label><input id="modelName" type="text" name="model" required autocomplete="off" placeholder="gpt-4o-mini"></div><div class="form-actions"><button type="button" class="btn btn-secondary" data-modal-close>Cancel</button><button type="submit" class="btn btn-primary">Add model</button></div></form>`);
    document.getElementById('modelForm').addEventListener('submit', async event => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const button = event.submitter;
        setButtonBusy(button, true, 'Adding');
        try {
            await apiCall('/probes', 'POST', { provider_id: Number(data.get('provider_id')), model: data.get('model') });
            hideModal(true); loadModels(); showToast('Model added', 'success');
        } catch (error) { showToast(error.message, 'error'); setButtonBusy(button, false); }
    });
    showModal();
}

async function deleteProbe(probe) {
    if (!window.confirm(`Delete model "${probe.model}" and all of its history?`)) return;
    try { await apiCall(`/probes/${probe.id}`, 'DELETE'); showToast('Model deleted', 'success'); loadModels(); }
    catch (error) { showToast(error.message || 'Failed to delete model', 'error'); }
}

async function loadStats() {
    const generation = ++statsLoadGeneration;
    const container = document.getElementById('statsProviders');
    container.innerHTML = loadingMarkup('Loading summary and trend charts');
    const hours = Number(document.getElementById('timeRange').value);
    const isToday = chartPreferences.rangeDays === 0;
    const chartPromise = isToday
        ? apiCall(`/stats/hourly?hours=${todayHoursElapsed()}`)
        : apiCall(`/stats/daily?days=${chartPreferences.rangeDays}`);
    const [summaryResult, chartResult] = await Promise.allSettled([
        apiCall(`/stats?hours=${hours}`),
        chartPromise
    ]);
    if (generation !== statsLoadGeneration || currentPage !== 'stats') return;

    document.getElementById('clearStatsBtn').hidden = isGuest;
    document.getElementById('exportCsvBtn').hidden = isGuest;
    if (summaryResult.status === 'rejected') {
        container.innerHTML = errorMarkup('Statistics could not be loaded', summaryResult.reason.message);
        return;
    }
    statsRows = asArray(summaryResult.value);
    if (isToday) {
        hourlyStats = chartResult.status === 'fulfilled' ? normalizeHourlyStats(chartResult.value) : [];
        dailyStats = [];
    } else {
        dailyStats = chartResult.status === 'fulfilled' ? normalizeDailyStats(chartResult.value) : [];
        hourlyStats = [];
    }
    chartLoadError = chartResult.status === 'rejected' ? chartResult.reason.message : '';
    timelineCache.clear();
    renderStats();
}

function renderStats() {
    const container = document.getElementById('statsProviders');
    const providers = normalizeProviders(statsRows);
    if (!providers.length) {
        container.innerHTML = emptyMarkup('No statistics available', 'Run probes to build a performance history.');
        return;
    }

    const rangeLabel = chartPreferences.rangeDays === 0 ? "Today's hourly" : chartPreferences.rangeDays === 7 ? '7-day' : '30-day';
    const bucket = chartPreferences.rangeDays === 0 ? 'hour' : 'date';
    const subtitle = chartPreferences.mode === 'per-day'
        ? `Aggregates by ${bucket}, all models shown per ${bucket}`
        : `Aggregates by ${bucket}, one model can be highlighted`;

    container.innerHTML = providers.map((provider, providerIndex) => {
        const metric = chartMetrics.get(provider.provider_name) || 'uptime';
        const collapsed = collapsedCharts.has(provider.provider_name);
        const rows = provider.models.map((model, modelIndex) => {
            const hasData = finiteNumber(model.total_probes, 0) > 0;
            const timelineKey = `${providerIndex}:${modelIndex}`;
            const timelineOpen = expandedTimelines.has(timelineKey);
            return `<tr class="stats-model-row" data-provider-index="${providerIndex}" data-model-index="${modelIndex}">
            <td data-label="Status"><span class="status-with-dot"><span class="state-dot ${currentState(model)}" aria-hidden="true"></span>${currentState(model) === 'available' ? 'Up' : currentState(model) === 'unavailable' ? 'Down' : 'Unknown'}</span></td>
            <td data-label="Model"><code>${escapeHtml(model.model)}</code></td>
            <td data-label="Probes" class="tabular">${finiteNumber(model.total_probes, 0)}</td>
            <td data-label="Uptime" class="tabular ${hasData ? rateClass(numberValue(model.success_rate)) : ''}">${hasData ? formatPercent(numberValue(model.success_rate)) : 'No data'}</td>
            <td data-label="Latency" class="tabular">${hasData ? formatMetric(numberValue(model.avg_latency_ms), 'ms', 0) : '-'}</td>
            <td data-label="TTFT" class="tabular">${formatMetric(numberValue(model.avg_ttft_ms), 'ms', 0, true)}</td>
            <td data-label="Actions"><div class="table-actions">${Number(model.probe_id) ? `<button class="btn btn-secondary btn-sm" type="button" data-action="timeline" data-provider-index="${providerIndex}" data-model-index="${modelIndex}" aria-expanded="${timelineOpen}">${timelineOpen ? 'Hide timeline' : 'Timeline'}</button>` : ''}${!isGuest && Number(model.probe_id) ? `<button class="btn btn-secondary btn-sm" type="button" data-action="logs" data-provider-index="${providerIndex}" data-model-index="${modelIndex}">Logs</button><button class="btn btn-danger-outline btn-sm" type="button" data-action="delete" data-provider-index="${providerIndex}" data-model-index="${modelIndex}">Delete</button>` : isGuest ? '<span class="muted">Read only</span>' : ''}</div></td>
        </tr>
        <tr class="timeline-row ${timelineOpen ? '' : 'is-collapsed'}" data-timeline-row="${timelineKey}" ${timelineOpen ? '' : 'hidden'}>
            <td colspan="7"><div class="timeline-panel" id="timeline-${providerIndex}-${modelIndex}">${timelineOpen ? loadingMarkup('Loading uptime timeline') : ''}</div></td>
        </tr>`;
        }).join('');
        const providerHasData = finiteNumber(provider.total_probes, 0) > 0;
        const effectiveCollapsed = chartsHidden || collapsed;
        return `<section class="provider-panel stats-panel" aria-labelledby="stats-provider-${providerIndex}">
            <header class="provider-header"><div><p class="provider-label">Provider</p><h2 id="stats-provider-${providerIndex}">${escapeHtml(provider.provider_name)}</h2></div><div class="provider-header-actions"><div class="provider-summary"><span><strong>${finiteNumber(provider.total_probes, 0)}</strong> probes</span><span class="${providerHasData ? rateClass(numberValue(provider.success_rate)) : ''}"><strong>${providerHasData ? formatPercent(numberValue(provider.success_rate)) : 'No data'}</strong> uptime</span></div><button class="btn btn-secondary btn-sm" type="button" data-action="copy-provider" data-provider-index="${providerIndex}">${copyIcon()}Copy report</button></div></header>
            <div class="table-scroll"><table class="data-table stats-table"><thead><tr><th scope="col">Status</th><th scope="col">Model</th><th scope="col">Probes</th><th scope="col">Uptime</th><th scope="col">Latency</th><th scope="col">TTFT</th><th scope="col">Actions</th></tr></thead><tbody>${rows}</tbody></table></div>
            <div class="chart-section ${chartsHidden || collapsed ? 'is-collapsed' : ''}" data-chart-section="${providerIndex}">
                <div class="chart-toolbar"><div><p class="chart-title">${rangeLabel} model trends</p><p class="chart-subtitle">${subtitle}</p></div><div class="chart-controls" role="group" aria-label="Chart metric for ${escapeAttribute(provider.provider_name)}">${Object.entries(METRICS).map(([key, config]) => `<button type="button" class="metric-btn ${metric === key ? 'active' : ''}" data-action="metric" data-provider-index="${providerIndex}" data-metric="${key}" aria-pressed="${metric === key}">${config.label}</button>`).join('')}<button type="button" class="icon-btn collapse-btn" data-action="collapse-chart" data-provider-index="${providerIndex}" aria-expanded="${!effectiveCollapsed}" aria-label="${effectiveCollapsed ? 'Expand' : 'Collapse'} ${escapeAttribute(provider.provider_name)} chart" ${chartsHidden ? 'disabled' : ''}>${chevronIcon(effectiveCollapsed)}</button></div></div>
                <div class="chart-content" id="provider-chart-${providerIndex}">${chartsHidden ? '' : chartLoadError ? errorMarkup('Chart data unavailable', chartLoadError) : loadingMarkup('Rendering chart')}</div>
            </div>
        </section>`;
    }).join('');

    updateChartsToggle();
    if (!chartsHidden && !chartLoadError) providers.forEach((provider, index) => {
        if (!collapsedCharts.has(provider.provider_name)) renderProviderChart(index, provider);
    });
    providers.forEach((provider, providerIndex) => {
        provider.models.forEach((model, modelIndex) => {
            const key = `${providerIndex}:${modelIndex}`;
            if (expandedTimelines.has(key)) loadModelTimeline(providerIndex, modelIndex, provider, model);
        });
    });
}

function handleStatsAction(event) {
    const button = event.target.closest('button[data-action]');
    const legend = event.target.closest('button[data-action="legend"]');
    const target = button || legend;
    if (!target) return;
    const action = target.dataset.action;
    if (action === 'timeline-logs') {
        showDowntimeLogs(Number(target.dataset.probeId), target.dataset.providerName, target.dataset.modelName, Number(target.dataset.start), Number(target.dataset.end));
        return;
    }
    const providerIndex = Number(target.dataset.providerIndex);
    const modelIndexRaw = target.dataset.modelIndex;
    const modelIndex = modelIndexRaw === undefined ? -1 : Number(modelIndexRaw);
    const provider = normalizeProviders(statsRows)[providerIndex];
    if (!provider) return;
    if (action === 'copy-provider') copyText(providerReport(provider), `${provider.provider_name} report copied`);
    if (action === 'metric') {
        chartMetrics.set(provider.provider_name, target.dataset.metric);
        renderStats();
    }
    if (action === 'collapse-chart') {
        if (collapsedCharts.has(provider.provider_name)) collapsedCharts.delete(provider.provider_name);
        else collapsedCharts.add(provider.provider_name);
        renderStats();
    }
    if (action === 'legend') {
        const model = provider.models[Number(target.dataset.modelIndex)];
        if (!model) return;
        const current = chartPreferences.selectedModelByProvider[provider.provider_name];
        if (current === model.model) chartPreferences.selectedModelByProvider[provider.provider_name] = '';
        else chartPreferences.selectedModelByProvider[provider.provider_name] = model.model;
        if (!chartPreferences.selectedModelByProvider[provider.provider_name]) delete chartPreferences.selectedModelByProvider[provider.provider_name];
        persistChartPreferences();
        renderProviderChart(providerIndex, provider);
    }
    const model = modelIndex >= 0 ? provider.models[modelIndex] : null;
    if (action === 'timeline' && model) toggleModelTimeline(providerIndex, modelIndex, model);
    if (action === 'logs' && model) showProbeDetails(Number(model.probe_id), provider.provider_name, model.model);
    if (action === 'delete' && model) deleteModelFromStats(provider, model);
}

function handleStatsKeydown(event) {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    const target = event.target.closest('button[data-action="legend"]');
    if (!target) return;
    event.preventDefault();
    target.click();
}

function toggleAllCharts() {
    chartsHidden = !chartsHidden;
    renderStats();
}

function updateChartsToggle() {
    const button = document.getElementById('toggleChartsBtn');
    button.textContent = chartsHidden ? 'Show charts' : 'Hide charts';
    button.setAttribute('aria-pressed', String(chartsHidden));
}

async function deleteModelFromStats(provider, model) {
    if (!window.confirm(`Delete model "${model.model}" from ${provider.provider_name} and remove its history?`)) return;
    try { await apiCall(`/probes/${model.probe_id}`, 'DELETE'); showToast('Model deleted', 'success'); loadStats(); }
    catch (error) { showToast(error.message || 'Failed to delete model', 'error'); }
}

function normalizeDailyStats(payload) {
    const result = [];
    const add = (providerName, modelName, points) => {
        if (!providerName || !modelName || !Array.isArray(points)) return;
        let provider = result.find(item => item.provider_name === providerName);
        if (!provider) { provider = { provider_name: String(providerName), models: [] }; result.push(provider); }
        provider.models.push({ model: String(modelName), points: points.map(normalizeDailyPoint).filter(point => point.date) });
    };

    const source = Array.isArray(payload) ? payload : asArray(payload && (payload.providers || payload.data || payload.stats));
    source.forEach(item => {
        const providerName = item.provider_name ?? item.provider ?? item.name;
        if (Array.isArray(item.models)) {
            item.models.forEach(model => add(providerName, model.model ?? model.model_name ?? model.name, model.days ?? model.daily ?? model.daily_stats ?? model.points ?? model.data ?? model.summaries ?? []));
        } else if (item.model || item.model_name) {
            if (item.date) add(providerName, item.model ?? item.model_name, [item]);
            else add(providerName, item.model ?? item.model_name, item.days ?? item.daily ?? item.daily_stats ?? item.points ?? item.data ?? []);
        }
    });

    // Some APIs return a flat row per provider/model/day.
    if (source.some(item => item.date && (item.model || item.model_name))) {
        result.length = 0;
        const groups = new Map();
        source.forEach(item => {
            const providerName = String(item.provider_name ?? item.provider ?? 'Unknown provider');
            const modelName = String(item.model ?? item.model_name ?? 'Unknown model');
            const key = `${providerName}\u0000${modelName}`;
            if (!groups.has(key)) groups.set(key, { providerName, modelName, points: [] });
            groups.get(key).points.push(item);
        });
        groups.forEach(group => add(group.providerName, group.modelName, group.points));
    }
    return result;
}

function normalizeDailyPoint(point) {
    const total = finiteNumber(point.total ?? point.total_probes, 0);
    const failed = finiteNumber(point.failed ?? point.failed_probes, 0);
    const hasSuccessfulSample = total > failed;
    const successRaw = point.success ?? point.uptime ?? point.success_rate;
    const success = successRaw === null || successRaw === undefined
        ? (total > 0 ? ((total - failed) / total) * 100 : null)
        : numberValue(successRaw);
    return {
        date: String(point.date ?? point.day ?? '').slice(0, 10),
        success: Number.isFinite(success) ? success : null,
        avg_latency_ms: hasSuccessfulSample ? numberValue(point.avg_latency_ms ?? point.latency_ms ?? point.latency) : null,
        avg_ttft_ms: hasSuccessfulSample ? numberValue(point.avg_ttft_ms ?? point.ttft_ms ?? point.ttft) : null,
        total,
        failed
    };
}

function normalizeHourlyStats(payload) {
    const result = [];
    const source = Array.isArray(payload) ? payload : asArray(payload && (payload.providers || payload.data || payload.stats));
    source.forEach(item => {
        const providerName = textValue(item.provider_name ?? item.provider ?? item.name ?? 'Unknown provider');
        let provider = result.find(row => row.provider_name === providerName);
        if (!provider) {
            provider = { provider_name: providerName, models: [] };
            result.push(provider);
        }
        asArray(item.models).forEach(model => {
            const points = asArray(model.hourly ?? model.hours ?? model.points ?? model.data).map(normalizeHourlyPoint).filter(point => point.date);
            provider.models.push({
                model: textValue(model.model ?? model.model_name ?? model.name ?? 'Unknown model'),
                probe_id: Number(model.probe_id) || 0,
                points
            });
        });
    });
    return result;
}

function normalizeHourlyPoint(point) {
    const total = finiteNumber(point.total ?? point.total_probes, 0);
    const failed = finiteNumber(point.failed ?? point.failed_probes, 0);
    const hourRaw = textValue(point.hour ?? point.date ?? point.time ?? '');
    let key = hourRaw;
    // Canonical key: "YYYY-MM-DD HH:00" (backend may send "YYYY-MM-DD HH:00:00").
    const match = hourRaw.match(/^(\d{4}-\d{2}-\d{2})[ T](\d{2})/);
    if (match) key = `${match[1]} ${match[2]}:00`;
    const success = total > 0 ? ((total - failed) / total) * 100 : null;
    return {
        date: key,
        success,
        avg_latency_ms: null,
        avg_ttft_ms: null,
        total,
        failed
    };
}

function todayHoursElapsed() {
    const now = new Date();
    return Math.min(24, Math.max(1, now.getHours() + 1));
}

function todayHourKeys() {
    const now = new Date();
    const count = todayHoursElapsed();
    const keys = [];
    for (let hour = 0; hour < count; hour += 1) {
        const stamp = new Date(now.getFullYear(), now.getMonth(), now.getDate(), hour, 0, 0, 0);
        const year = stamp.getFullYear();
        const month = String(stamp.getMonth() + 1).padStart(2, '0');
        const day = String(stamp.getDate()).padStart(2, '0');
        const hh = String(stamp.getHours()).padStart(2, '0');
        keys.push(`${year}-${month}-${day} ${hh}:00`);
    }
    return keys;
}

function chartBucketKeys() {
    return chartPreferences.rangeDays === 0 ? todayHourKeys() : recentCalendarDates(chartPreferences.rangeDays);
}

function chartSeriesForProvider(providerName) {
    if (chartPreferences.rangeDays === 0) return hourlyStats.find(item => item.provider_name === providerName) || null;
    return dailyStats.find(item => item.provider_name === providerName) || null;
}

function formatBucketLabel(bucket) {
    if (chartPreferences.rangeDays === 0) {
        const hour = Number(String(bucket).slice(11, 13));
        if (Number.isFinite(hour)) return `${String(hour).padStart(2, '0')}:00`;
        return bucket;
    }
    return formatShortDate(bucket);
}

function formatBucketLong(bucket) {
    if (chartPreferences.rangeDays === 0) {
        const date = new Date(String(bucket).replace(' ', 'T'));
        if (Number.isNaN(date.getTime())) return bucket;
        return date.toLocaleString(document.documentElement.lang, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
    }
    return formatLongDate(bucket);
}

function summaryRangeHours() {
    const hours = Number(document.getElementById('timeRange')?.value);
    return Number.isFinite(hours) && hours > 0 ? hours : 24;
}

function toggleModelTimeline(providerIndex, modelIndex, model) {
    const key = `${providerIndex}:${modelIndex}`;
    if (expandedTimelines.has(key)) expandedTimelines.delete(key);
    else expandedTimelines.add(key);
    renderStats();
}

async function loadDashboardTimelines(providers) {
    const hours = 24;
    providers.forEach((provider, providerIndex) => {
        provider.models.forEach((model, modelIndex) => {
            loadDashboardTimeline(providerIndex, modelIndex, provider, model, hours);
        });
    });
}

async function loadDashboardTimeline(providerIndex, modelIndex, provider, model, hours = 24) {
    const panel = document.getElementById(`dashboard-timeline-${providerIndex}-${modelIndex}`);
    if (!panel) return;
    const probeId = Number(model.probe_id);
    if (!probeId) {
        panel.innerHTML = emptyMarkup('No timeline', 'This model has no probe id.');
        return;
    }
    const cacheKey = `${probeId}:${hours}`;
    const meta = { probeId, providerName: provider.provider_name, modelName: model.model };
    if (timelineCache.has(cacheKey)) {
        panel.innerHTML = renderTimelineMarkup(timelineCache.get(cacheKey), hours, meta);
        return;
    }
    panel.innerHTML = loadingMarkup('Loading uptime timeline');
    try {
        const periods = asArray(await apiCall(`/probes/${probeId}/downtime?hours=${hours}`)).map(period => ({
            start: new Date(period.start),
            end: new Date(period.end)
        })).filter(period => !Number.isNaN(period.start.getTime()) && !Number.isNaN(period.end.getTime()));
        timelineCache.set(cacheKey, periods);
        if (!document.getElementById(`dashboard-timeline-${providerIndex}-${modelIndex}`)) return;
        panel.innerHTML = renderTimelineMarkup(periods, hours, meta);
    } catch (error) {
        if (!document.getElementById(`dashboard-timeline-${providerIndex}-${modelIndex}`)) return;
        panel.innerHTML = errorMarkup('Timeline unavailable', error.message);
    }
}

async function loadModelTimeline(providerIndex, modelIndex, provider, model) {
    const key = `${providerIndex}:${modelIndex}`;
    const panel = document.getElementById(`timeline-${providerIndex}-${modelIndex}`);
    if (!panel || !expandedTimelines.has(key)) return;
    const probeId = Number(model.probe_id);
    if (!probeId) {
        panel.innerHTML = emptyMarkup('No timeline', 'This model has no probe id.');
        return;
    }
    const hours = summaryRangeHours();
    const cacheKey = `${probeId}:${hours}`;
    const meta = { probeId, providerName: provider.provider_name, modelName: model.model };
    if (timelineCache.has(cacheKey)) {
        panel.innerHTML = renderTimelineMarkup(timelineCache.get(cacheKey), hours, meta);
        return;
    }
    panel.innerHTML = loadingMarkup('Loading uptime timeline');
    try {
        const periods = asArray(await apiCall(`/probes/${probeId}/downtime?hours=${hours}`)).map(period => ({
            start: new Date(period.start),
            end: new Date(period.end)
        })).filter(period => !Number.isNaN(period.start.getTime()) && !Number.isNaN(period.end.getTime()));
        timelineCache.set(cacheKey, periods);
        if (!expandedTimelines.has(key)) return;
        panel.innerHTML = renderTimelineMarkup(periods, hours, meta);
    } catch (error) {
        if (!expandedTimelines.has(key)) return;
        panel.innerHTML = errorMarkup('Timeline unavailable', error.message);
    }
}

function renderTimelineMarkup(periods, hours, meta = null) {
    const now = Date.now();
    const rangeStart = now - hours * 3600 * 1000;
    const rangeMs = Math.max(1, now - rangeStart);
    const clipped = periods.map(period => ({
        start: Math.max(rangeStart, period.start.getTime()),
        end: Math.min(now, period.end.getTime())
    })).filter(period => period.end > period.start);
    const downtimeMs = clipped.reduce((sum, period) => sum + (period.end - period.start), 0);
    const uptimePct = Math.max(0, Math.min(100, ((rangeMs - downtimeMs) / rangeMs) * 100));
    const canOpenLogs = meta && Number(meta.probeId) && !isGuest;
    const periodAttrs = period => canOpenLogs
        ? ` data-action="timeline-logs" data-probe-id="${Number(meta.probeId)}" data-provider-name="${escapeAttribute(meta.providerName || '')}" data-model-name="${escapeAttribute(meta.modelName || '')}" data-start="${period.start}" data-end="${period.end}"`
        : '';
    const segments = clipped.map(period => {
        const left = ((period.start - rangeStart) / rangeMs) * 100;
        const width = ((period.end - period.start) / rangeMs) * 100;
        const title = `${formatDateTime(new Date(period.start))} – ${formatDateTime(new Date(period.end))}`;
        if (canOpenLogs) {
            return `<button class="segment segment-btn" type="button" style="left:${left.toFixed(3)}%;width:${Math.max(width, 0.35).toFixed(3)}%" title="${escapeAttribute(title)}"${periodAttrs(period)} aria-label="View down logs ${escapeAttribute(title)}"></button>`;
        }
        return `<span class="segment" style="left:${left.toFixed(3)}%;width:${Math.max(width, 0.35).toFixed(3)}%" title="${escapeAttribute(title)}"></span>`;
    }).join('');
    const list = clipped.length
        ? `<ul class="timeline-periods">${clipped.map(period => {
            const duration = formatDurationMs(period.end - period.start);
            const rangeLabel = `${formatDateTime(new Date(period.start))} – ${formatDateTime(new Date(period.end))}`;
            if (canOpenLogs) {
                return `<li><button class="timeline-period-btn" type="button"${periodAttrs(period)}><span class="pill-down">Down</span><span>${escapeHtml(rangeLabel)}</span><strong class="tabular">${escapeHtml(duration)}</strong></button></li>`;
            }
            return `<li><span class="pill-down">Down</span><span>${escapeHtml(rangeLabel)}</span><strong class="tabular">${escapeHtml(duration)}</strong></li>`;
        }).join('')}</ul>`
        : emptyMarkup('No downtime in range', 'All recorded probes in this window succeeded.');
    return `<div class="timeline-meta"><span>Window <strong>${hours}h</strong></span><span>Uptime <strong class="${rateClass(uptimePct)}">${uptimePct.toFixed(1)}%</strong></span><span>Downtime segments <strong>${clipped.length}</strong></span></div>
        <div class="timeline-bar" role="img" aria-label="Uptime timeline for the selected window">${segments}</div>
        ${list}`;
}

function formatDurationMs(ms) {
    const totalSec = Math.max(0, Math.round(ms / 1000));
    if (totalSec < 60) return `${totalSec}s`;
    const minutes = Math.floor(totalSec / 60);
    if (minutes < 60) return `${minutes}m`;
    const hours = Math.floor(minutes / 60);
    const remMin = minutes % 60;
    if (hours < 48) return remMin ? `${hours}h ${remMin}m` : `${hours}h`;
    const days = Math.floor(hours / 24);
    const remHours = hours % 24;
    return remHours ? `${days}d ${remHours}h` : `${days}d`;
}

function displayErrorMessage(raw) {
    let text = textValue(raw).trim();
    if (!text) return '';
    // Strip SDK request line prefixes like: POST "https://host/path": 403 {...}
    text = text.replace(/^(?:POST|GET|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(?:"[^"]+"|'[^']+'|\S+)\s*(?::\s*|\s+)?/i, '');
    text = text.replace(/^(?:POST|GET|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+https?:\/\/\S+\s*/i, '');
    // Common wrappers: "error: ", "Error code: xxx "
    text = text.replace(/^(?:error|error message)\s*:\s*/i, '');
    text = text.replace(/^Error code:\s*[^\s]+\s*/i, '');
    text = text.replace(/^\d{3}\s+/, '');
    text = text.trim();
    return text || textValue(raw).trim();
}

function renderProviderChart(providerIndex, provider) {
    const container = document.getElementById(`provider-chart-${providerIndex}`);
    if (!container) return;
    container.replaceChildren();
    const seriesProvider = chartSeriesForProvider(provider.provider_name);
    const isToday = chartPreferences.rangeDays === 0;
    const rangeLabel = isToday ? "today's hourly" : chartPreferences.rangeDays === 7 ? '7-day' : '30-day';
    if (!seriesProvider || !seriesProvider.models.some(series => series.points.length)) {
        container.innerHTML = emptyMarkup(`No ${rangeLabel} chart data`, isToday
            ? 'Hourly SLA will appear after probes have run today.'
            : 'Daily aggregates will appear after probes have run.');
        return;
    }

    const metricKey = chartMetrics.get(provider.provider_name) || 'uptime';
    const metric = METRICS[metricKey];
    const allDates = chartBucketKeys();
    if (!allDates.length) {
        container.innerHTML = emptyMarkup(`No ${rangeLabel} chart data`, 'No chart buckets available for the selected range.');
        return;
    }

    const selectedModel = chartPreferences.selectedModelByProvider[provider.provider_name] || '';
    const highlightMode = Boolean(selectedModel);
    const visibleSeries = highlightMode
        ? seriesProvider.models.filter(series => series.model === selectedModel)
        : seriesProvider.models;
    const values = visibleSeries.flatMap(series => series.points.map(point => point[metric.field]).filter(Number.isFinite));
    const maxValue = metricKey === 'uptime' ? 100 : niceMax(Math.max(...values, 0));
    const width = 960;
    const height = 286;
    const left = 64;
    const right = 20;
    const top = 18;
    const bottom = 48;
    const plotWidth = width - left - right;
    const plotHeight = height - top - bottom;
    const svg = svgElement('svg', { viewBox: `0 0 ${width} ${height}`, class: 'line-chart', role: 'group', 'aria-label': `${provider.provider_name} ${metric.label} trends for ${rangeLabel}` });

    for (let tick = 0; tick <= 4; tick += 1) {
        const y = top + plotHeight * (tick / 4);
        const value = maxValue * (1 - tick / 4);
        svg.append(svgElement('line', { x1: left, y1: y, x2: width - right, y2: y, class: 'chart-gridline' }));
        const label = svgElement('text', { x: left - 10, y: y + 4, class: 'chart-axis-label', 'text-anchor': 'end' });
        label.textContent = formatAxis(value, metric);
        svg.append(label);
    }

    const labelStep = Math.max(1, Math.ceil(allDates.length / 6));
    allDates.forEach((date, index) => {
        if (index % labelStep !== 0 && index !== allDates.length - 1) return;
        const x = chartX(index, allDates.length, left, plotWidth);
        const label = svgElement('text', { x, y: height - 18, class: 'chart-axis-label', 'text-anchor': 'middle' });
        label.textContent = formatBucketLabel(date);
        svg.append(label);
    });

    seriesProvider.models.forEach((series, seriesIndex) => {
        const isHighlighted = highlightMode && series.model === selectedModel;
        const isMuted = highlightMode && !isHighlighted;
        const color = CHART_COLORS[seriesIndex % CHART_COLORS.length];
        const byDate = new Map(series.points.map(point => [point.date, point]));

        let pathData = '';
        let drawing = false;
        allDates.forEach((date, index) => {
            const point = byDate.get(date);
            const value = point && point[metric.field];
            if (!Number.isFinite(value)) { drawing = false; return; }
            const x = chartX(index, allDates.length, left, plotWidth);
            const y = top + plotHeight - (Math.max(0, value) / maxValue) * plotHeight;
            pathData += `${drawing ? ' L' : ' M'} ${x.toFixed(2)} ${y.toFixed(2)}`;
            drawing = true;
        });
        if (pathData) {
            const path = svgElement('path', { d: pathData.trim(), class: 'chart-line' + (isMuted ? ' is-muted' : ''), stroke: color, 'stroke-width': isHighlighted ? 3 : (highlightMode ? 1.4 : 2.2) });
            path.setAttribute('opacity', isMuted ? '0.25' : (highlightMode && !isHighlighted ? '0.35' : '1'));
            svg.append(path);
        }

        allDates.forEach((date, index) => {
            const point = byDate.get(date);
            const value = point && point[metric.field];
            if (!Number.isFinite(value)) return;
            const x = chartX(index, allDates.length, left, plotWidth);
            const y = top + plotHeight - (Math.max(0, value) / maxValue) * plotHeight;
            const dot = svgElement('circle', { cx: x, cy: y, r: isHighlighted ? 4 : 3, fill: color, class: 'chart-dot' + (isMuted ? ' is-muted' : ''), 'aria-hidden': 'true' });
            dot.setAttribute('opacity', isMuted ? '0.25' : (highlightMode && !isHighlighted ? '0.35' : '1'));
            svg.append(dot);

            if (chartPreferences.mode === 'per-point') {
                const hit = svgElement('circle', { cx: x, cy: y, r: 12, class: 'chart-hit', tabindex: '0', role: 'button', 'aria-describedby': 'chartTooltip', 'aria-label': `${series.model}, ${formatBucketLong(date)}, ${metric.label} ${formatMetric(value, metric.unit, metric.decimals)}` });
                if (isMuted) hit.setAttribute('aria-disabled', 'true');
                const show = event => showChartTooltip(event, series.model, date, value, metric, point, color);
                hit.addEventListener('pointerenter', show);
                hit.addEventListener('pointermove', show);
                hit.addEventListener('pointerdown', show);
                hit.addEventListener('focus', show);
                hit.addEventListener('pointerleave', event => { if (document.activeElement !== event.currentTarget) hideChartTooltip(); });
                hit.addEventListener('blur', hideChartTooltip);
                svg.append(hit);
            }
        });
    });

    if (chartPreferences.mode === 'per-day') {
        const colWidth = allDates.length > 1 ? plotWidth / (allDates.length - 1) : plotWidth;
        const hitWidth = Math.max(colWidth * 0.8, 16);
        allDates.forEach((date, index) => {
            const x = chartX(index, allDates.length, left, plotWidth);
            const rect = svgElement('rect', { x: Math.max(left, x - hitWidth / 2), y: top, width: Math.min(hitWidth, width - right - Math.max(left, x - hitWidth / 2)), height: plotHeight, class: 'chart-hit chart-day-hit', tabindex: '0', role: 'button', 'aria-describedby': 'chartTooltip', 'aria-label': `${formatBucketLong(date)} all models` });
            rect.addEventListener('pointerenter', event => showDayTooltip(event, date, seriesProvider.models, metric));
            rect.addEventListener('pointermove', event => showDayTooltip(event, date, seriesProvider.models, metric));
            rect.addEventListener('pointerdown', event => showDayTooltip(event, date, seriesProvider.models, metric));
            rect.addEventListener('focus', event => showDayTooltip(event, date, seriesProvider.models, metric));
            rect.addEventListener('pointerleave', event => { if (document.activeElement !== event.currentTarget) hideChartTooltip(); });
            rect.addEventListener('blur', hideChartTooltip);
            svg.append(rect);
        });
    }

    const legend = document.createElement('div');
    legend.className = 'chart-legend';
    seriesProvider.models.forEach((series, index) => {
        const color = CHART_COLORS[index % CHART_COLORS.length];
        const active = highlightMode && series.model === selectedModel;
        const muted = highlightMode && !active;
        const item = document.createElement('button');
        item.type = 'button';
        item.className = `chart-legend-btn${active ? ' active' : ''}${muted ? ' muted' : ''}`;
        item.dataset.action = 'legend';
        item.dataset.providerIndex = String(providerIndex);
        item.dataset.modelIndex = String(index);
        item.setAttribute('aria-pressed', String(active));
        item.title = active ? `Click to clear highlight` : `Highlight ${series.model} only`;
        const swatch = document.createElement('i');
        swatch.style.backgroundColor = color;
        item.append(swatch, document.createTextNode(series.model));
        legend.append(item);
    });
    if (highlightMode) {
        const reset = document.createElement('button');
        reset.type = 'button';
        reset.className = 'icon-text-btn';
        reset.dataset.action = 'legend';
        reset.dataset.providerIndex = String(providerIndex);
        reset.dataset.modelIndex = '0';
        reset.style.display = 'none';
        legend.append(reset);
        const hint = document.createElement('span');
        hint.className = 'muted-hint';
        hint.textContent = 'Click a model again to clear highlight.';
        legend.append(hint);
    }
    const scroll = document.createElement('div');
    scroll.className = 'chart-scroll';
    scroll.append(svg);
    container.append(scroll, legend);
}

function showChartTooltip(event, model, date, value, metric, point, color) {
    const tooltip = document.getElementById('chartTooltip');
    tooltip.replaceChildren();
    const heading = document.createElement('strong');
    const swatch = document.createElement('i');
    swatch.style.backgroundColor = color;
    heading.append(swatch, document.createTextNode(model));
    const dateLine = document.createElement('span');
    dateLine.textContent = formatBucketLong(date);
    const valueLine = document.createElement('b');
    valueLine.textContent = `${metric.label}: ${formatMetric(value, metric.unit, metric.decimals)}`;
    const probeLine = document.createElement('small');
    probeLine.textContent = `${finiteNumber(point.total, 0)} probes, ${finiteNumber(point.failed, 0)} failed`;
    tooltip.append(heading, dateLine, valueLine, probeLine);
    positionChartTooltip(tooltip, event);
}

function showDayTooltip(event, date, seriesList, metric) {
    const tooltip = document.getElementById('chartTooltip');
    tooltip.replaceChildren();
    const heading = document.createElement('strong');
    heading.textContent = formatBucketLong(date);
    tooltip.append(heading);
    const list = document.createElement('ul');
    list.style.margin = '0';
    list.style.padding = '0';
    list.style.listStyle = 'none';
    list.style.display = 'grid';
    list.style.gap = '2px';
    seriesList.forEach((series, index) => {
        const byDate = new Map(series.points.map(point => [point.date, point]));
        const point = byDate.get(date);
        const value = point && point[metric.field];
        const item = document.createElement('li');
        item.style.display = 'grid';
        item.style.gridTemplateColumns = '10px 1fr auto';
        item.style.alignItems = 'center';
        item.style.gap = '6px';
        const swatch = document.createElement('i');
        swatch.style.width = '8px';
        swatch.style.height = '8px';
        swatch.style.borderRadius = '50%';
        swatch.style.backgroundColor = CHART_COLORS[index % CHART_COLORS.length];
        const label = document.createElement('span');
        label.textContent = series.model;
        const valueLabel = document.createElement('b');
        valueLabel.style.fontFamily = 'var(--font-mono)';
        valueLabel.textContent = Number.isFinite(value) ? formatMetric(value, metric.unit, metric.decimals) : 'No data';
        item.append(swatch, label, valueLabel);
        list.append(item);
    });
    tooltip.append(list);
    positionChartTooltip(tooltip, event);
}

function positionChartTooltip(tooltip, event) {
    tooltip.hidden = false;
    tooltip.style.visibility = 'hidden';
    let x = event.clientX;
    let y = event.clientY;
    if (!Number.isFinite(x) || (!x && !y)) {
        const rect = event.currentTarget.getBoundingClientRect();
        x = rect.left + rect.width / 2;
        y = rect.top;
    }
    const rect = tooltip.getBoundingClientRect();
    const margin = 10;
    const left = Math.min(Math.max(margin, x + 14), window.innerWidth - rect.width - margin);
    const top = Math.min(Math.max(margin, y - rect.height - 14), window.innerHeight - rect.height - margin);
    tooltip.style.left = `${left}px`;
    tooltip.style.top = `${top}px`;
    tooltip.style.visibility = 'visible';
}

function hideChartTooltip() {
    document.getElementById('chartTooltip').hidden = true;
}

async function clearStats() {
    if (!window.confirm('Clear all statistics? This action cannot be undone.')) return;
    try { await apiCall('/stats', 'DELETE'); showToast('Statistics cleared', 'success'); loadStats(); }
    catch (error) { showToast(error.message || 'Failed to clear statistics', 'error'); }
}

async function exportCSV() {
    const hours = document.getElementById('timeRange').value;
    const token = localStorage.getItem('auth_token');
    try {
        const response = await fetch(`${API_BASE}/export/csv?hours=${encodeURIComponent(hours)}`, { headers: token ? { Authorization: `Bearer ${token}` } : {} });
        if (!response.ok) throw new Error(`Export failed (${response.status})`);
        const blob = await response.blob();
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement('a');
        anchor.href = url;
        anchor.download = `uptime_report_${new Date().toISOString().slice(0, 10)}.csv`;
        document.body.append(anchor); anchor.click(); anchor.remove(); URL.revokeObjectURL(url);
        showToast('CSV exported', 'success');
    } catch (error) { showToast(error.message || 'Failed to export CSV', 'error'); }
}

function logPaginationMarkup(page, totalPages, position) {
    if (totalPages <= 1) return '';
    return `<nav class="pagination ${position === 'top' ? 'pagination-top' : ''}" aria-label="Request log pages ${position}"><button class="btn btn-secondary btn-sm" type="button" data-log-page="${page - 1}" ${page <= 1 ? 'disabled' : ''}>Previous</button><span>Page ${page} of ${totalPages}</span><button class="btn btn-secondary btn-sm" type="button" data-log-page="${page + 1}" ${page >= totalPages ? 'disabled' : ''}>Next</button></nav>`;
}

async function showDowntimeLogs(probeId, providerName, modelName, startMs, endMs) {
    if (!probeId || isGuest) return;
    const start = new Date(startMs);
    const end = new Date(endMs);
    if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return;
    const rangeLabel = `${formatDateTime(start)} – ${formatDateTime(end)}`;
    const generation = setModalContent('Down event logs', `${providerName} / ${modelName}`, loadingMarkup('Loading down event logs'));
    showModal();
    try {
        const data = await apiCall(`/probes/${probeId}/results?limit=100&status=failed`);
        if (generation !== modalGeneration) return;
        const all = asArray(data && (data.results || data));
        const padMs = 1000;
        const results = all.filter(result => {
            const created = new Date(result.created_at).getTime();
            return Number.isFinite(created) && created >= (startMs - padMs) && created <= (endMs + padMs);
        });
        const body = document.getElementById('modalBody');
        body.innerHTML = `<div class="log-toolbar" role="group" aria-label="Down event logs"><span class="muted">${escapeHtml(rangeLabel)}</span><span class="muted">${results.length} failed probe${results.length === 1 ? '' : 's'}</span><button class="btn btn-secondary btn-sm" type="button" data-action="view-all-logs">View all logs</button></div>
            <div class="log-table-wrap"><table class="data-table log-table"><thead><tr><th scope="col">Time</th><th scope="col">Status</th><th scope="col">Latency</th><th scope="col">TTFT</th><th scope="col">Request ID</th><th scope="col">Error</th></tr></thead><tbody>${results.length ? results.map(result => `<tr><td data-label="Time">${escapeHtml(formatDateTime(result.created_at))}</td><td data-label="Status"><span class="status-pill danger">${escapeHtml(statusLabel(result.status))}</span></td><td data-label="Latency" class="tabular">${formatMetric(numberValue(result.latency_ms), 'ms', 0)}</td><td data-label="TTFT" class="tabular">${formatMetric(numberValue(result.ttft_ms), 'ms', 0, true)}</td><td data-label="Request ID"><code class="wrap-code">${escapeHtml(result.request_id || '-')}</code></td><td data-label="Error" class="log-error">${escapeHtml(displayErrorMessage(result.error_message) || '-')}</td></tr>`).join('') : '<tr><td colspan="6">No failed logs found for this down period.</td></tr>'}</tbody></table></div>`;
        body.querySelector('[data-action="view-all-logs"]')?.addEventListener('click', () => showProbeDetails(probeId, providerName, modelName, 'failed', 1));
    } catch (error) {
        if (generation !== modalGeneration) return;
        document.getElementById('modalBody').innerHTML = errorMarkup('Down event logs could not be loaded', error.message);
    }
}

async function showProbeDetails(probeId, providerName, model, statusFilter = '', page = 1) {
    if (!probeId || isGuest) return;
    const generation = setModalContent('Request logs', `${providerName} / ${model}`, loadingMarkup('Loading request history'));
    showModal();
    try {
        const filter = statusFilter ? `&status=${encodeURIComponent(statusFilter)}` : '';
        const data = await apiCall(`/probes/${probeId}/results?limit=20&page=${page}${filter}`);
        if (generation !== modalGeneration) return;
        const results = asArray(data && (data.results || data));
        const totalPages = Math.max(1, finiteNumber(data && data.pages, 1));
        const body = document.getElementById('modalBody');
        body.innerHTML = `<div class="log-toolbar" role="group" aria-label="Filter request logs"><button class="btn btn-sm ${statusFilter ? 'btn-secondary' : 'btn-primary'}" type="button" data-log-filter="">All</button><button class="btn btn-sm ${statusFilter === 'failed' ? 'btn-danger-outline' : 'btn-secondary'}" type="button" data-log-filter="failed">Failed only</button><span class="muted">${finiteNumber(data && data.total, results.length)} records</span></div>
            ${logPaginationMarkup(page, totalPages, 'top')}
            <div class="log-table-wrap"><table class="data-table log-table"><thead><tr><th scope="col">Time</th><th scope="col">Status</th><th scope="col">Latency</th><th scope="col">TTFT</th><th scope="col">Request ID</th><th scope="col">Error</th><th scope="col">Action</th></tr></thead><tbody>${results.length ? results.map((result, index) => `<tr><td data-label="Time">${escapeHtml(formatDateTime(result.created_at))}</td><td data-label="Status"><span class="status-pill ${result.status === 'success' ? 'success' : 'danger'}">${escapeHtml(statusLabel(result.status))}</span></td><td data-label="Latency" class="tabular">${formatMetric(numberValue(result.latency_ms), 'ms', 0)}</td><td data-label="TTFT" class="tabular">${formatMetric(numberValue(result.ttft_ms), 'ms', 0, true)}</td><td data-label="Request ID"><code class="wrap-code">${escapeHtml(result.request_id || '-')}</code></td><td data-label="Error" class="log-error">${escapeHtml(displayErrorMessage(result.error_message) || '-')}</td><td data-label="Action"><button class="btn btn-danger-outline btn-sm" type="button" data-delete-result="${index}">Delete</button></td></tr>`).join('') : '<tr><td colspan="7">No request logs match this filter.</td></tr>'}</tbody></table></div>
            ${logPaginationMarkup(page, totalPages, 'bottom')}`;
        body.querySelectorAll('[data-log-filter]').forEach(button => button.addEventListener('click', () => showProbeDetails(probeId, providerName, model, button.dataset.logFilter, 1)));
        body.querySelectorAll('[data-log-page]').forEach(button => button.addEventListener('click', () => showProbeDetails(probeId, providerName, model, statusFilter, Number(button.dataset.logPage))));
        body.querySelectorAll('[data-delete-result]').forEach(button => button.addEventListener('click', () => deleteResult(results[Number(button.dataset.deleteResult)], probeId, providerName, model, statusFilter, page)));
    } catch (error) {
        if (generation !== modalGeneration) return;
        document.getElementById('modalBody').innerHTML = errorMarkup('Request logs could not be loaded', error.message);
    }
}

async function deleteResult(result, probeId, providerName, model, statusFilter, page) {
    if (!result || !window.confirm('Delete this request log? This cannot be undone.')) return;
    try { await apiCall(`/results/${result.id}`, 'DELETE'); showToast('Request log deleted', 'success'); showProbeDetails(probeId, providerName, model, statusFilter, page); }
    catch (error) { showToast(error.message || 'Failed to delete request log', 'error'); }
}

function initModal() {
    const modal = document.getElementById('modal');
    document.getElementById('modalCloseBtn').addEventListener('click', () => hideModal());
    modal.addEventListener('click', event => {
        if (event.target === modal) hideModal();
        if (event.target.closest('[data-modal-close]')) hideModal();
    });
    modal.addEventListener('input', event => { if (event.target.closest('form')) modalDirty = true; });
    document.addEventListener('keydown', event => {
        if (modal.hidden) return;
        if (event.key === 'Escape') { event.preventDefault(); hideModal(); }
        if (event.key === 'Tab') trapModalFocus(event);
    });
}

function setModalContent(title, kicker, content) {
    const generation = ++modalGeneration;
    document.getElementById('modalTitle').textContent = textValue(title);
    document.getElementById('modalKicker').textContent = textValue(kicker);
    document.getElementById('modalBody').innerHTML = content;
    modalDirty = false;
    return generation;
}

function showModal() {
    const modal = document.getElementById('modal');
    const wasHidden = modal.hidden;
    if (wasHidden) modalOpener = document.activeElement;
    modal.hidden = false;
    document.body.classList.add('modal-open');
    if (!wasHidden) return;
    requestAnimationFrame(() => {
        const focusTarget = modal.querySelector('input:not([type="hidden"]), select, button, [href], [tabindex="0"]');
        (focusTarget || modal.querySelector('.modal-content')).focus();
    });
}

function hideModal(force = false) {
    const modal = document.getElementById('modal');
    if (modal.hidden) return true;
    if (!force && modalDirty && !window.confirm('Discard unsaved changes?')) return false;
    modal.hidden = true;
    modalGeneration += 1;
    document.body.classList.remove('modal-open');
    document.getElementById('modalBody').replaceChildren();
    modalDirty = false;
    if (modalOpener && document.contains(modalOpener)) modalOpener.focus();
    modalOpener = null;
    return true;
}

function trapModalFocus(event) {
    const focusable = [...document.getElementById('modal').querySelectorAll('button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])')].filter(element => element.offsetParent !== null);
    if (!focusable.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
}

function initTheme() {
    const saved = localStorage.getItem('color_theme');
    if (saved === 'light' || saved === 'dark') document.documentElement.dataset.theme = saved;
    updateThemeLabel();
    document.getElementById('themeBtn').addEventListener('click', () => {
        const current = document.documentElement.dataset.theme || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
        const next = current === 'dark' ? 'light' : 'dark';
        document.documentElement.dataset.theme = next;
        localStorage.setItem('color_theme', next);
        updateThemeLabel();
    });
}

function updateThemeLabel() {
    const explicit = document.documentElement.dataset.theme;
    const active = explicit || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    document.getElementById('themeLabel').textContent = active === 'dark' ? 'Use light theme' : 'Use dark theme';
}

async function copyText(text, successMessage) {
    try {
        if (navigator.clipboard && window.isSecureContext) await navigator.clipboard.writeText(text);
        else fallbackCopy(text);
        showToast(successMessage, 'success');
    } catch (error) { showToast('Clipboard access failed', 'error'); }
}

function fallbackCopy(text) {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.append(textarea);
    textarea.select();
    const copied = document.execCommand('copy');
    textarea.remove();
    if (!copied) throw new Error('Copy failed');
}

function showToast(message, type = 'success') {
    const toast = document.getElementById('toast');
    window.clearTimeout(toastTimer);
    toast.textContent = textValue(message);
    toast.className = `toast ${type}`;
    toast.hidden = false;
    document.getElementById('announcer').textContent = textValue(message);
    toastTimer = window.setTimeout(() => { toast.hidden = true; }, 3500);
}

function setButtonBusy(button, busy, label = '') {
    if (!button) return;
    if (busy) {
        button.dataset.originalContent = button.innerHTML;
        button.disabled = true;
        button.textContent = label;
    } else {
        button.disabled = false;
        if (button.dataset.originalContent) button.innerHTML = button.dataset.originalContent;
        delete button.dataset.originalContent;
    }
}

function normalizeProviders(stats) {
    return asArray(stats).map(provider => ({
        ...provider,
        provider_name: textValue(provider.provider_name ?? provider.name ?? 'Unknown provider'),
        models: asArray(provider.models).map(model => ({ ...model, model: textValue(model.model ?? model.name ?? 'Unknown model') }))
    }));
}

function asArray(value) { return Array.isArray(value) ? value : []; }
function numberValue(value) { const number = Number(value); return Number.isFinite(number) ? number : 0; }
function finiteNumber(value, fallback) { const number = Number(value); return Number.isFinite(number) ? number : fallback; }
function textValue(value) { return value === null || value === undefined ? '' : String(value); }
function reportValue(value) { return textValue(value).replace(/[\r\n\t\u0000-\u001f\u007f]+/g, ' ').replace(/\s+/g, ' ').trim(); }
function isAvailable(model) {
    if (typeof model.available === 'boolean') return model.available;
    return ['success', 'available', 'operational', 'healthy', 'up'].includes(String(model.current_status || model.last_status || '').toLowerCase());
}
function currentState(model) {
    const status = String(model.current_status || model.last_status || '').toLowerCase();
    if (!status) return 'neutral';
    return isAvailable(model) ? 'available' : 'unavailable';
}
function currentError(model) { return textValue(model.current_error ?? model.current_error_message ?? model.last_error ?? model.last_error_message ?? model.error_message ?? ''); }
function todayUptime(model) {
    if (Object.prototype.hasOwnProperty.call(model, 'today_uptime')) {
        return model.today_uptime === null ? null : numberValue(model.today_uptime);
    }
    const fallback = model.today_success_rate ?? model.daily_success_rate ?? model.uptime_today;
    return fallback === null || fallback === undefined ? null : numberValue(fallback);
}
function rateClass(rate) { return rate >= 99 ? 'rate-good' : rate >= 95 ? 'rate-warning' : 'rate-bad'; }
function statusLabel(value) { return textValue(value || 'Unknown').replaceAll('_', ' ').replace(/\b\w/g, letter => letter.toUpperCase()); }
function formatPercent(value) { return value === null || value === undefined ? 'No probes today' : `${numberValue(value).toFixed(1)}%`; }
function formatMetric(value, unit, decimals, emptyZero = false) { return emptyZero && !value ? '-' : `${numberValue(value).toFixed(decimals)}${unit}`; }
function formatDateTime(value) { const date = new Date(value); return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString(document.documentElement.lang); }
function formatShortDate(value) { const date = new Date(`${value}T00:00:00`); return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString(document.documentElement.lang, { month: 'short', day: 'numeric' }); }
function formatLongDate(value) { const date = new Date(`${value}T00:00:00`); return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString(document.documentElement.lang, { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' }); }
function formatAxis(value, metric) { return `${value.toFixed(metric.decimals > 0 ? 1 : 0)}${metric.unit}`; }
function niceMax(value) { if (value <= 0) return 1; const power = 10 ** Math.floor(Math.log10(value)); return Math.ceil(value / power * 2) / 2 * power; }
function chartX(index, count, left, width) { return count <= 1 ? left + width / 2 : left + width * (index / (count - 1)); }
function recentCalendarDates(days) {
    const count = Math.max(0, Number(days) || 0);
    if (!count) return [];
    const dates = [];
    const current = new Date();
    current.setHours(0, 0, 0, 0);
    current.setDate(current.getDate() - count + 1);
    for (let index = 0; index < count; index += 1) {
        const year = current.getFullYear();
        const month = String(current.getMonth() + 1).padStart(2, '0');
        const day = String(current.getDate()).padStart(2, '0');
        dates.push(`${year}-${month}-${day}`);
        current.setDate(current.getDate() + 1);
    }
    return dates;
}
function adjustDate(days) {
    const current = new Date();
    current.setHours(0, 0, 0, 0);
    current.setDate(current.getDate() - days);
    return current;
}
function safeHttpUrl(value) { try { const url = new URL(String(value)); return url.protocol === 'https:' || url.protocol === 'http:' ? url.href : ''; } catch (error) { return ''; } }

function svgElement(name, attributes) {
    const element = document.createElementNS('http://www.w3.org/2000/svg', name);
    Object.entries(attributes).forEach(([key, value]) => element.setAttribute(key, String(value)));
    return element;
}

function escapeHtml(value) {
    return textValue(value).replace(/[&<>'"]/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' })[character]);
}

function escapeAttribute(value) { return escapeHtml(value || ''); }
function loadingMarkup(message) { return `<div class="loading-state"><span class="spinner" aria-hidden="true"></span>${escapeHtml(message)}</div>`; }
function emptyMarkup(title, detail) { return `<div class="empty-state"><strong>${escapeHtml(title)}</strong><span>${escapeHtml(detail)}</span></div>`; }
function errorMarkup(title, detail) { return `<div class="empty-state error-state"><strong>${escapeHtml(title)}</strong><span>${escapeHtml(detail)}</span></div>`; }
function copyIcon() { return '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/></svg>'; }
function logsIcon() { return '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 4h10M7 9h10M7 14h6M5 20h14a2 2 0 0 0 2-2V3H3v15a2 2 0 0 0 2 2Z"/></svg>'; }
function chevronIcon(collapsed) { return `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="${collapsed ? 'm8 10 4 4 4-4' : 'm8 14 4-4 4 4'}"/></svg>`; }
