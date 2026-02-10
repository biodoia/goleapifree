/* ═══════════════════════════════════════════════════════════════════════
   GoLeapAI HTMX Configuration
   Auto-refresh, WebSocket, Form Handling
   ═══════════════════════════════════════════════════════════════════════ */

document.addEventListener('DOMContentLoaded', function() {
    console.log('GoLeapAI CP437 WebUI - HTMX Initialized');

    // HTMX Configuration
    htmx.config.defaultSwapStyle = 'innerHTML';
    htmx.config.defaultSwapDelay = 0;
    htmx.config.defaultSettleDelay = 100;

    // Global HTMX event handlers
    document.body.addEventListener('htmx:beforeRequest', function(evt) {
        // Add loading indicator
        const target = evt.detail.target;
        if (target) {
            target.classList.add('htmx-loading');
        }
    });

    document.body.addEventListener('htmx:afterRequest', function(evt) {
        // Remove loading indicator
        const target = evt.detail.target;
        if (target) {
            target.classList.remove('htmx-loading');
        }
    });

    document.body.addEventListener('htmx:responseError', function(evt) {
        console.error('HTMX Error:', evt.detail);
        // Show error in terminal style
        const target = evt.detail.target;
        if (target) {
            target.innerHTML = `
                <div class="error-message">
                    ╔═══════════════════════════════════════════╗<br>
                    ║           ERROR LOADING DATA              ║<br>
                    ║  ${evt.detail.xhr.status} ${evt.detail.xhr.statusText.padEnd(36)} ║<br>
                    ╚═══════════════════════════════════════════╝
                </div>
            `;
        }
    });

    // WebSocket for live updates
    setupWebSocket();

    // Keyboard shortcuts (F-keys)
    setupKeyboardShortcuts();

    // Auto-refresh stats counter
    setupStatsCounter();
});

/**
 * Setup WebSocket connection for live updates
 */
function setupWebSocket() {
    let ws;
    let reconnectInterval = 5000;

    function connect() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        ws = new WebSocket(wsUrl);

        ws.onopen = function() {
            console.log('WebSocket connected');
            reconnectInterval = 5000; // Reset reconnect interval
        };

        ws.onmessage = function(event) {
            // Update stats panel with WebSocket data
            const statsPanel = document.querySelector('#stats-panel .panel-content');
            if (statsPanel && event.data) {
                statsPanel.innerHTML = event.data;
            }
        };

        ws.onerror = function(error) {
            console.error('WebSocket error:', error);
        };

        ws.onclose = function() {
            console.log('WebSocket closed. Reconnecting...');
            setTimeout(connect, reconnectInterval);
            reconnectInterval = Math.min(reconnectInterval * 1.5, 30000); // Exponential backoff
        };
    }

    // Only connect if browser supports WebSocket
    if ('WebSocket' in window) {
        connect();
    }
}

/**
 * Setup keyboard shortcuts
 */
function setupKeyboardShortcuts() {
    document.addEventListener('keydown', function(e) {
        // F1: Dashboard
        if (e.key === 'F1') {
            e.preventDefault();
            window.location.href = '/';
        }

        // F2: Focus providers
        if (e.key === 'F2') {
            e.preventDefault();
            const providersPanel = document.querySelector('#providers-panel');
            if (providersPanel) {
                providersPanel.scrollIntoView({ behavior: 'smooth' });
            }
        }

        // F3: Focus stats
        if (e.key === 'F3') {
            e.preventDefault();
            const statsPanel = document.querySelector('#stats-panel');
            if (statsPanel) {
                statsPanel.scrollIntoView({ behavior: 'smooth' });
            }
        }

        // F4: Focus logs
        if (e.key === 'F4') {
            e.preventDefault();
            const logsPanel = document.querySelector('#logs-panel');
            if (logsPanel) {
                logsPanel.scrollIntoView({ behavior: 'smooth' });
            }
        }

        // F5: Force refresh (default behavior)
        // F10: Exit (close window)
        if (e.key === 'F10') {
            e.preventDefault();
            if (confirm('Exit GoLeapAI WebUI?')) {
                window.close();
            }
        }

        // Escape: Cancel/Clear
        if (e.key === 'Escape') {
            // Clear any active modals or selections
            document.querySelectorAll('.active').forEach(el => {
                el.classList.remove('active');
            });
        }
    });
}

/**
 * Setup live stats counter (requests per second)
 */
function setupStatsCounter() {
    let lastRequestCount = 0;
    let currentRequestCount = 0;

    setInterval(function() {
        // Get current request count from stats
        const statValue = document.querySelector('.stat-value');
        if (statValue) {
            const text = statValue.textContent.trim();
            // Parse number (handle K, M suffixes)
            if (text.includes('K')) {
                currentRequestCount = parseFloat(text) * 1000;
            } else if (text.includes('M')) {
                currentRequestCount = parseFloat(text) * 1000000;
            } else {
                currentRequestCount = parseInt(text) || 0;
            }

            // Calculate RPS
            const rps = Math.max(0, currentRequestCount - lastRequestCount);
            lastRequestCount = currentRequestCount;

            // Update footer RPS
            const rpsElement = document.querySelector('#rps');
            if (rpsElement) {
                rpsElement.textContent = rps;
            }
        }
    }, 1000);
}

/**
 * Format numbers with K/M suffixes
 */
function formatNumber(num) {
    if (num >= 1000000) {
        return (num / 1000000).toFixed(1) + 'M';
    }
    if (num >= 1000) {
        return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
}

/**
 * Terminal-style console log
 */
function terminalLog(message) {
    console.log(`[GoLeapAI] ${message}`);
}

/**
 * Show notification in terminal style
 */
function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.className = `terminal-notification notification-${type}`;
    notification.innerHTML = `
        ╔═══════════════════════════════════════════╗<br>
        ║ ${message.padEnd(41)} ║<br>
        ╚═══════════════════════════════════════════╝
    `;

    document.body.appendChild(notification);

    // Auto-remove after 3 seconds
    setTimeout(() => {
        notification.style.opacity = '0';
        setTimeout(() => notification.remove(), 300);
    }, 3000);
}

// Export for global use
window.GoLeapAI = {
    formatNumber,
    terminalLog,
    showNotification
};
