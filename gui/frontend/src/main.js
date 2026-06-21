import './style.css';
import './app.css';

// Import Go Bindings
import { GetToken, SetToken, StartTunnel, StopTunnel, Quit } from '../wailsjs/go/main/App';
// Import Wails Runtime APIs
import { EventsOn } from '../wailsjs/runtime/runtime';

// State Management
let state = {
    token: '',
    status: 'offline', // offline, connecting, online, error
    publicUrl: '',
    activeTab: 'tunnels',
};

// Render App Skeleton
const appContainer = document.querySelector('#app');
appContainer.innerHTML = `
    <!-- Sidebar -->
    <div class="sidebar">
        <div class="sidebar-header">
            <div class="brand">
                <div class="brand-logo">⚡</div>
                <div class="brand-name">SpeedTunnel</div>
            </div>
            <div class="nav-menu">
                <div class="nav-item active" data-tab="tunnels">
                    <span>🔀</span> Tunnels
                </div>
                <div class="nav-item" data-tab="settings">
                    <span>⚙️</span> Settings
                </div>
                <div class="nav-item" data-tab="logs">
                    <span>📋</span> Console Logs
                </div>
                <div class="nav-item" data-tab="about">
                    <span>ℹ️</span> About
                </div>
            </div>
        </div>
        <div class="sidebar-footer">
            <div>SpeedTunnel Desktop</div>
            <div>Version 1.0.0</div>
            <div class="dev-info">Abdulhafiz Davlatov</div>
        </div>
    </div>

    <!-- Main Content Area -->
    <div class="content-area">
        <!-- Top Navigation / Status Header -->
        <div class="glass-card status-widget">
            <div>
                <h1 style="font-size: 20px; font-weight: 600;">Tunnel Manager</h1>
                <p style="font-size: 13px; color: var(--text-secondary); margin-top: 4px;">Forward local ports to secure public URLs</p>
            </div>
            <div id="statusBadge" class="status-badge offline">
                <span class="status-dot"></span>
                <span id="statusText">Offline</span>
            </div>
        </div>

        <!-- Tunnels Tab -->
        <div id="tab-tunnels" class="tab-content active">
            <!-- Warning if Token is Missing -->
            <div id="tokenWarning" class="alert alert-warning" style="display: none;">
                <span>⚠️</span>
                <div>
                    <strong>Auth Token Missing:</strong> You must configure your auth token before starting a tunnel. Go to 
                    <span class="alert-link" onclick="switchTab('settings')">Settings</span> or get one at 
                    <a href="https://jprq.io/auth" target="_blank" class="alert-link">jprq.io/auth</a>.
                </div>
            </div>

            <div class="glass-card">
                <h2 class="card-title"><span>🚀</span> Quick Tunnel</h2>
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label" for="protocol">Protocol</label>
                        <select id="protocol" class="input-field">
                            <option value="http">HTTP (Web App)</option>
                            <option value="tcp">TCP (SSH, DB, Game Server)</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label class="form-label" for="port">Local Port</label>
                        <input id="port" type="number" class="input-field" value="8000" placeholder="8000" min="1" max="65535" />
                    </div>
                </div>
                <div class="form-group">
                    <label class="form-label" for="subdomain">Custom Subdomain (Optional, HTTP only)</label>
                    <input id="subdomain" type="text" class="input-field" placeholder="e.g. my-local-app" />
                </div>
                <div style="margin-top: 24px;">
                    <button id="toggleTunnelBtn" class="btn btn-success" style="width: 100%;">
                        <span>▶</span> Start Tunnel
                    </button>
                </div>
            </div>

            <!-- Forwarded URL Card (hidden by default) -->
            <div id="forwardedCard" class="glass-card forwarded-card" style="display: none;">
                <h2 class="card-title"><span>🔗</span> Active Tunnel URL</h2>
                <div class="url-box">
                    <span id="forwardedUrl" class="url-text">https://loading...</span>
                    <button id="copyUrlBtn" class="copy-btn">Copy Link</button>
                </div>
            </div>
        </div>

        <!-- Settings Tab -->
        <div id="tab-settings" class="tab-content">
            <div class="glass-card">
                <h2 class="card-title"><span>🔒</span> Authentication Settings</h2>
                <p style="font-size: 13.5px; color: var(--text-secondary); margin-bottom: 20px;">
                    SpeedTunnel supports authentication tokens for private servers (e.g. jprq.io). For tokenless or self-hosted servers, you can leave this field empty. You can obtain your free token at 
                    <a href="https://jprq.io/auth" target="_blank" class="alert-link" style="font-weight: 500;">jprq.io/auth</a>.
                </p>
                <div class="form-group">
                    <label class="form-label" for="authToken">Auth Token (Optional)</label>
                    <input id="authToken" type="password" class="input-field" placeholder="Optional - Enter token if required" />
                    <div id="tokenStatus" class="token-status">Loading token status...</div>
                </div>
                <div style="margin-top: 24px;">
                    <button id="saveTokenBtn" class="btn" style="width: 100%;">
                        <span>💾</span> Save Token
                    </button>
                </div>
            </div>
        </div>

        <!-- Logs Tab -->
        <div id="tab-logs" class="tab-content">
            <div class="console-container">
                <div class="console-header">
                    <div class="console-title">
                        <span>📟</span> System Output / Traffic Logs
                    </div>
                    <div class="console-actions">
                        <button id="clearLogsBtn" class="console-btn">Clear Logs</button>
                    </div>
                </div>
                <div id="consoleBody" class="console-body">
                    <div class="log-entry"><span class="log-time">[System]</span> SpeedTunnel console initialized. Logs will appear here.</div>
                </div>
            </div>
        </div>

        <!-- About Tab -->
        <div id="tab-about" class="tab-content">
            <div class="glass-card" style="line-height: 1.6;">
                <h2 class="card-title" style="border-bottom: 1px solid var(--border-color); padding-bottom: 12px; margin-bottom: 20px;">
                    <span>🛡️</span> SpeedTunnel Desktop
                </h2>
                <div style="display: flex; flex-direction: column; gap: 12px; font-size: 14px;">
                    <p><strong>SpeedTunnel</strong> is a fast, lightweight, and secure reverse proxy client that exposes local ports to the internet.</p>
                    <p style="color: var(--text-secondary);">This application combines the robustness of Go with a sleek webview user interface using Wails.</p>
                    
                    <div style="background: rgba(255, 255, 255, 0.02); border: 1px solid var(--border-color); border-radius: 8px; padding: 16px; margin-top: 16px;">
                        <h3 style="font-size: 14px; font-weight: 600; margin-bottom: 8px;">Developer Information</h3>
                        <p>👤 <strong>Developer:</strong> Davlatov Abdulhafiz</p>
                        <p>📧 <strong>Email:</strong> <a href="mailto:auz.offical@gmail.com" class="alert-link">auz.offical@gmail.com</a></p>
                        <p>📞 <strong>Phone:</strong> +998906960010</p>
                        <p>📦 <strong>Version:</strong> 1.0.0 (Release)</p>
                    </div>
                </div>
            </div>
        </div>
    </div>
`;

// DOM Elements
const navItems = document.querySelectorAll('.nav-item');
const tabContents = document.querySelectorAll('.tab-content');
const statusBadge = document.getElementById('statusBadge');
const statusText = document.getElementById('statusText');
const toggleTunnelBtn = document.getElementById('toggleTunnelBtn');
const forwardedCard = document.getElementById('forwardedCard');
const forwardedUrl = document.getElementById('forwardedUrl');
const copyUrlBtn = document.getElementById('copyUrlBtn');
const protocolSelect = document.getElementById('protocol');
const portInput = document.getElementById('port');
const subdomainInput = document.getElementById('subdomain');
const authTokenInput = document.getElementById('authToken');
const tokenStatus = document.getElementById('tokenStatus');
const saveTokenBtn = document.getElementById('saveTokenBtn');
const consoleBody = document.getElementById('consoleBody');
const clearLogsBtn = document.getElementById('clearLogsBtn');
const tokenWarning = document.getElementById('tokenWarning');

// Tab Switching Logic
window.switchTab = function(tabId) {
    state.activeTab = tabId;
    navItems.forEach(item => {
        if (item.getAttribute('data-tab') === tabId) {
            item.classList.add('active');
        } else {
            item.classList.remove('active');
        }
    });
    tabContents.forEach(content => {
        if (content.id === `tab-${tabId}`) {
            content.classList.add('active');
        } else {
            content.classList.remove('active');
        }
    });
};

navItems.forEach(item => {
    item.addEventListener('click', () => {
        switchTab(item.getAttribute('data-tab'));
    });
});

// Write to Console
function appendLog(message, type = 'info') {
    const timestamp = new Date().toLocaleTimeString();
    const logDiv = document.createElement('div');
    logDiv.className = 'log-entry';
    
    let colorClass = 'log-info';
    if (type === 'success') colorClass = 'log-success';
    if (type === 'error') colorClass = 'log-error';
    if (type === 'warning') colorClass = 'log-warning';
    
    logDiv.innerHTML = `<span class="log-time">[${timestamp}]</span><span class="${colorClass}">${escapeHtml(message)}</span>`;
    consoleBody.appendChild(logDiv);
    consoleBody.scrollTop = consoleBody.scrollHeight;
}

function escapeHtml(text) {
    return text
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

clearLogsBtn.addEventListener('click', () => {
    consoleBody.innerHTML = '<div class="log-entry"><span class="log-time">[System]</span> Console cleared.</div>';
});

// Update Status Badges
function updateStatus(newStatus, extra = '') {
    state.status = newStatus;
    statusBadge.className = `status-badge ${newStatus}`;
    
    // Capitalize status text
    statusText.innerText = newStatus.charAt(0).toUpperCase() + newStatus.slice(1);
    
    if (newStatus === 'offline') {
        toggleTunnelBtn.className = 'btn btn-success';
        toggleTunnelBtn.innerHTML = '<span>▶</span> Start Tunnel';
        toggleTunnelBtn.disabled = false;
        forwardedCard.style.display = 'none';
        state.publicUrl = '';
    } else if (newStatus === 'connecting') {
        toggleTunnelBtn.className = 'btn btn-secondary';
        toggleTunnelBtn.innerHTML = '<span>⏳</span> Connecting...';
        toggleTunnelBtn.disabled = true;
        forwardedCard.style.display = 'none';
    } else if (newStatus === 'online') {
        toggleTunnelBtn.className = 'btn btn-danger';
        toggleTunnelBtn.innerHTML = '<span>⏹</span> Stop Tunnel';
        toggleTunnelBtn.disabled = false;
        
        state.publicUrl = extra;
        forwardedUrl.innerText = extra;
        forwardedCard.style.display = 'flex';
        appendLog(`Tunnel active: ${extra}`, 'success');
    } else if (newStatus === 'error') {
        toggleTunnelBtn.className = 'btn btn-success';
        toggleTunnelBtn.innerHTML = '<span>▶</span> Start Tunnel';
        toggleTunnelBtn.disabled = false;
        forwardedCard.style.display = 'none';
        appendLog(`Error: ${extra}`, 'error');
    }
}

// Load Config on Startup
async function loadConfig() {
    try {
        const token = await GetToken();
        state.token = token;
        if (token) {
            authTokenInput.value = token;
            tokenStatus.className = 'token-status saved';
            tokenStatus.innerHTML = '✓ Token loaded successfully';
        } else {
            tokenStatus.className = 'token-status saved';
            tokenStatus.innerHTML = '✓ Running in tokenless/optional mode';
        }
        tokenWarning.style.display = 'none';
    } catch (err) {
        console.error('Error loading config:', err);
        tokenStatus.className = 'token-status missing';
        tokenStatus.innerHTML = '✗ Error reading configuration';
    }
}

// Save Token
saveTokenBtn.addEventListener('click', async () => {
    const rawToken = authTokenInput.value.trim();
    saveTokenBtn.disabled = true;
    saveTokenBtn.innerHTML = 'Saving...';
    
    try {
        await SetToken(rawToken);
        state.token = rawToken;
        tokenStatus.className = 'token-status saved';
        if (rawToken) {
            tokenStatus.innerHTML = '✓ Token saved successfully';
            appendLog('Auth token updated in local configuration.', 'success');
        } else {
            tokenStatus.innerHTML = '✓ Token cleared (Optional)';
            appendLog('Auth token cleared from local configuration.', 'success');
        }
        tokenWarning.style.display = 'none';
    } catch (err) {
        tokenStatus.className = 'token-status missing';
        tokenStatus.innerHTML = `✗ Error saving: ${err}`;
        appendLog(`Failed to save auth token: ${err}`, 'error');
    } finally {
        saveTokenBtn.disabled = false;
        saveTokenBtn.innerHTML = '<span>💾</span> Save Token';
    }
});

// Toggle Tunnel (Start / Stop)
toggleTunnelBtn.addEventListener('click', async () => {
    if (state.status === 'offline' || state.status === 'error') {
        // Start Tunnel
        const protocol = protocolSelect.value;
        const port = portInput.value.trim();
        const subdomain = subdomainInput.value.trim();
        
        if (!port || parseInt(port) <= 0 || parseInt(port) > 65535) {
            appendLog('Cannot start tunnel: Invalid port number.', 'error');
            portInput.focus();
            return;
        }
        
        appendLog(`Starting ${protocol.toUpperCase()} tunnel on port ${port}...`, 'info');
        try {
            await StartTunnel(protocol, port, subdomain);
        } catch (err) {
            updateStatus('error', err.toString());
        }
    } else {
        // Stop Tunnel
        appendLog('Stopping active tunnel...', 'info');
        try {
            await StopTunnel();
        } catch (err) {
            appendLog(`Failed to stop tunnel: ${err}`, 'error');
        }
    }
});

// Copy URL to Clipboard
copyUrlBtn.addEventListener('click', () => {
    if (!state.publicUrl) return;
    
    navigator.clipboard.writeText(state.publicUrl)
        .then(() => {
            copyUrlBtn.innerText = 'Copied!';
            copyUrlBtn.classList.add('copied');
            setTimeout(() => {
                copyUrlBtn.innerText = 'Copy Link';
                copyUrlBtn.classList.remove('copied');
            }, 2000);
        })
        .catch(err => {
            appendLog(`Failed to copy link: ${err}`, 'error');
        });
});

// Wails Event Listeners
EventsOn('tunnel:log', (msg) => {
    // Determine log color based on content
    let type = 'info';
    if (msg.includes('Online') || msg.includes('Forwarded:') || msg.includes('Debugger:')) {
        type = 'success';
    } else if (msg.toLowerCase().includes('error') || msg.toLowerCase().includes('failed')) {
        type = 'error';
    } else if (msg.toLowerCase().includes('warning')) {
        type = 'warning';
    }
    appendLog(msg, type);
});

EventsOn('tunnel:status', (data) => {
    if (data.status === 'connecting') {
        updateStatus('connecting');
    } else if (data.status === 'online') {
        updateStatus('online', data.url);
    } else if (data.status === 'offline') {
        updateStatus('offline');
    } else if (data.status === 'error') {
        updateStatus('error', data.error);
    }
});

// Initialize Config
loadConfig();
