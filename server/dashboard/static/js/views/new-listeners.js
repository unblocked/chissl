// New Listeners Management View (Non-AI only)
console.log('🚨🚨🚨 NEW LISTENERS VIEW MODULE LOADED - VERSION 2024-09-07-01:35 - FIXED HOSTNAME 🚨🚨🚨');

function loadNewListenersView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title">Listeners (Non-AI)</h3>' +
        '<div class="card-tools">' +
        '<button class="btn btn-success btn-sm" onclick="showAddListenerModal()" title="Create Listener">' +
        '<i class="fas fa-plus"></i> Add Listener</button>' +
        '<button class="btn btn-secondary btn-sm ml-2" onclick="loadNewListenersData()" title="Refresh">' +
        '<i class="fas fa-sync-alt"></i> Refresh</button>' +
        '</div>' +
        '</div>' +
        '<div class="card-body">' +
        '<div class="table-responsive">' +
        '<table class="table table-bordered table-striped">' +
        '<thead><tr><th>Name</th><th>URL</th><th>User</th><th>Port</th><th>Mode</th><th>Target/Response</th><th>Status</th><th>Actions</th></tr></thead>' +
        '<tbody id="new-listeners-tbody"><tr><td colspan="8" class="text-center">Loading listeners...</td></tr></tbody>' +
        '</table>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('#main-content').html(content);
    // Start auto-refresh every 10 seconds
    startNewListenersAutoRefresh();

// Auto-refresh for new listeners view
var newListenersRefreshInterval = null;
function startNewListenersAutoRefresh() {
    if (newListenersRefreshInterval) clearInterval(newListenersRefreshInterval);
    newListenersRefreshInterval = setInterval(function() {
        try { loadNewListenersData(); } catch (e) { console.warn('new-listeners auto-refresh error', e); }
    }, 10000);
    console.log('🔄 New Listeners auto-refresh started (10 seconds)');
}
function stopNewListenersAutoRefresh() {
    if (newListenersRefreshInterval) {
        clearInterval(newListenersRefreshInterval);
        newListenersRefreshInterval = null;
        console.log('⏹️ New Listeners auto-refresh stopped');
    }
}

    loadNewListenersData();
}

function loadNewListenersData() {
    $.get('/api/listeners')
        .done(function(data) {
            var tbody = '';
            var nonAIListeners = data.filter(function(listener) {
                return listener.mode !== 'ai-mock';
            });

            if (nonAIListeners && nonAIListeners.length > 0) {
                nonAIListeners.forEach(function(listener) {
                    var statusClass = listener.active ? 'badge-success' : 'badge-danger';
                    var statusText = listener.active ? 'Active' : 'Inactive';

                    var tlsBadge = listener.use_tls ?
                        '<span class="badge badge-success">TLS</span>' :
                        '<span class="badge badge-secondary">Plain</span>';

                    var targetDisplay = '';
                    if (listener.mode === 'proxy') {
                        targetDisplay = '<small class="text-muted">→ ' + (listener.target_url || 'N/A') + '</small>';
                    } else if (listener.mode === 'static') {
                        var responsePreview = listener.response ?
                            (listener.response.length > 50 ?
                                listener.response.substring(0, 50) + '...' :
                                listener.response) : 'Empty';
                        targetDisplay = '<small class="text-muted">' + escapeHtml(responsePreview) + '</small>';
                    } else if (listener.mode === 'sink') {
                        targetDisplay = '<small class="text-muted">Default Response</small>';
                    }

                    // Mode badge colors
                    var modeBadgeClass = 'badge-info'; // default blue
                    if (listener.mode === 'sink') {
                        modeBadgeClass = 'badge-warning'; // yellow for sink
                    } else if (listener.mode === 'proxy') {
                        modeBadgeClass = 'badge-info'; // blue for proxy
                    } else if (listener.mode === 'static') {
                        modeBadgeClass = 'badge-secondary'; // gray for static
                    }

                    var protocol = listener.use_tls ? 'https' : 'http';
                    var url = protocol + '://' + window.location.hostname + ':' + listener.port;
                    var createdDate = listener.created_at ? new Date(listener.created_at).toLocaleString() : 'Unknown';

                    tbody += '<tr>' +
                        '<td><strong>' + escapeHtml(listener.name || 'Unnamed') + '</strong><br>' +
                        '<small class="text-muted">' + (listener.id ? listener.id.substring(0, 8) + '...' : 'N/A') + '</small></td>' +
                        '<td>' +
                        '<div class="input-group input-group-sm">' +
                        '<input type="text" class="form-control form-control-sm" value="' + url + '" readonly style="font-size: 11px;">' +
                        '<div class="input-group-append">' +
                        '<button class="btn btn-outline-secondary btn-sm" type="button" onclick="copyToClipboard(\'' + url + '\')" title="Copy URL">' +
                        '<i class="fas fa-copy"></i>' +
                        '</button>' +
                        '</div>' +
                        '</div>' +
                        '</td>' +
                        '<td>' + escapeHtml(listener.username || 'Unknown') + '</td>' +
                        '<td><code class="text-dark">' + listener.port + '</code></td>' +
                        '<td><span class="badge ' + modeBadgeClass + '">' + listener.mode + '</span></td>' +
                        '<td>' + targetDisplay + '</td>' +
                        '<td><span class="badge ' + statusClass + '">' + statusText + '</span></td>' +
                        '<td>' +
                        '<div class="btn-group btn-group-sm" role="group">' +
                        '<button class="btn btn-outline-info" onclick="showTrafficPayloads(\'' + listener.id + '\', \'listener\')" title="Inspect Traffic">' +
                        '<i class="fas fa-eye"></i></button>' +
                        '<button class="btn btn-outline-warning" onclick="editListener(\'' + listener.id + '\')" title="Edit Listener">' +
                        '<i class="fas fa-edit"></i></button>' +
                        '<button class="btn btn-outline-danger" onclick="deleteNewListener(\'' + listener.id + '\')" title="Delete">' +
                        '<i class="fas fa-trash"></i></button>' +
                        '</div>' +
                        '</td>' +
                        '</tr>';
                });
            } else {
                tbody = '<tr><td colspan="8" class="text-center text-muted">No non-AI listeners found</td></tr>';
            }
            $('#new-listeners-tbody').html(tbody);
        })
        .fail(function() {
            $('#new-listeners-tbody').html('<tr><td colspan="8" class="text-center text-danger">Failed to load listeners</td></tr>');
        });
}

function deleteNewListener(listenerId) {
    if (confirm('Are you sure you want to delete this listener?')) {
        $.ajax({
            url: '/api/listener/' + listenerId,
            method: 'DELETE'
        })
        .done(function() {
            // Refresh the New Listeners table
            loadNewListenersData();
        })
        .fail(function(xhr) {
            alert('Failed to delete listener: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(function() {
        // Could add a toast notification here
        console.log('URL copied to clipboard:', text);
    }).catch(function(err) {
        console.error('Failed to copy URL:', err);
        // Fallback for older browsers
        var textArea = document.createElement('textarea');
        textArea.value = text;
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
    });
}

function escapeHtml(text) {
    if (!text) return '';
    var map = {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#039;'
    };
    return text.replace(/[&<>"']/g, function(m) { return map[m]; });
}
