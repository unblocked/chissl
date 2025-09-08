// AI Mock API Management View
console.log('🚨🚨🚨 AI MOCK API VIEW MODULE LOADED - VERSION 2024-09-07-01:40 - FIXED HOSTNAME 🚨🚨🚨');

function loadAIMockView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title">AI Mock API Endpoints</h3>' +
        '<div class="card-tools">' +
        '<button class="btn btn-success btn-sm" onclick="showAIResponsePreview(null, null, \'create\', null)" title="Create AI Endpoint">' +
        '<i class="fas fa-plus"></i> Create AI Endpoint</button>' +
        '<button class="btn btn-secondary btn-sm ml-2" onclick="loadAIMockData()" title="Refresh">' +
        '<i class="fas fa-sync-alt"></i> Refresh</button>' +
        '</div>' +
        '</div>' +
        '<div class="card-body">' +
        '<div class="table-responsive">' +
        '<table class="table table-bordered table-striped">' +
        '<thead><tr><th>Name</th><th>URL</th><th>Port</th><th>Status</th><th>AI Provider</th><th>Generation Status</th><th>Actions</th></tr></thead>' +
        '<tbody id="ai-mock-tbody"><tr><td colspan="7" class="text-center">Loading AI endpoints...</td></tr></tbody>' +
        '</table>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('#main-content').html(content);
    // Start auto-refresh every 10 seconds
    startAIMockAutoRefresh();

    loadAIMockData();
}

// Auto-refresh for AI Mock view
var aiMockRefreshInterval = null;
function startAIMockAutoRefresh() {
    if (aiMockRefreshInterval) clearInterval(aiMockRefreshInterval);
    aiMockRefreshInterval = setInterval(function() {
        try { loadAIMockData(); } catch (e) { console.warn('ai-mock auto-refresh error', e); }
    }, 10000);
    console.log('4 AI Mock auto-refresh started (10 seconds)');
}
function stopAIMockAutoRefresh() {
    if (aiMockRefreshInterval) {
        clearInterval(aiMockRefreshInterval);
        aiMockRefreshInterval = null;
        console.log(' AI Mock auto-refresh stopped');
    }
}

function loadAIMockData() {
    $.get('/api/listeners')
        .done(function(data) {
            var tbody = '';
            var aiListeners = data.filter(function(listener) {
                return listener.mode === 'ai-mock';
            });

            if (aiListeners && aiListeners.length > 0) {
                aiListeners.forEach(function(listener) {
                    var statusClass = listener.active ? 'badge-success' : 'badge-danger';
                    var statusText = listener.active ? 'Active' : 'Inactive';

                    var aiStatus = listener.ai_generation_status || 'pending';
                    var aiStatusClass = aiStatus === 'success' ? 'badge-success' :
                                       aiStatus === 'failed' ? 'badge-danger' : 'badge-warning';

                    var protocol = listener.use_tls ? 'https' : 'http';
                    var url = protocol + '://' + window.location.hostname + ':' + listener.port;

                    tbody += '<tr>' +
                        '<td><strong>' + escapeHtml(listener.name || 'Unnamed') + '</strong><br>' +
                        '<small class="text-muted">' + listener.id.substring(0, 8) + '...</small></td>' +
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
                        '<td><code class="text-dark">' + listener.port + '</code></td>' +
                        '<td><span class="badge ' + statusClass + '">' + statusText + '</span></td>' +
                        '<td>' + escapeHtml(listener.ai_provider_name || 'N/A') + '</td>' +
                        '<td><span class="badge ' + aiStatusClass + '">' + aiStatus + '</span></td>' +
                        '<td>' +
                        '<div class="btn-group btn-group-sm" role="group">' +
                        '<button class="btn btn-outline-info" onclick="showTrafficPayloads(\'' + listener.id + '\', \'listener\')" title="Inspect Traffic">' +
                        '<i class="fas fa-eye"></i></button>' +
                        '<button class="btn btn-outline-success" onclick="showAIChat(\'' + listener.id + '\')" title="Chat with AI">' +
                        '<i class="fas fa-comments"></i></button>' +
                        '<button class="btn btn-outline-warning" onclick="editAIListener(\'' + listener.id + '\')" title="Edit AI Endpoint">' +
                        '<i class="fas fa-robot"></i></button>' +
                        '<button class="btn btn-outline-danger" onclick="deleteAIListener(\'' + listener.id + '\')" title="Delete">' +
                        '<i class="fas fa-trash"></i></button>' +
                        '</div>' +
                        '</td>' +
                        '</tr>';
                });
            } else {
                tbody = '<tr><td colspan="7" class="text-center text-muted">No AI Mock API endpoints found</td></tr>';
            }
            $('#ai-mock-tbody').html(tbody);
        })
        .fail(function() {
            $('#ai-mock-tbody').html('<tr><td colspan="7" class="text-center text-danger">Failed to load AI endpoints</td></tr>');
        });
}

function deleteAIListener(listenerId) {
    if (confirm('Are you sure you want to delete this AI Mock API endpoint?')) {
        $.ajax({
            url: '/api/listeners/' + listenerId,
            method: 'DELETE'
        })
        .done(function() {
            // Refresh the AI Mock API table
            loadAIMockData();
        })
        .fail(function(xhr) {
            alert('Failed to delete AI endpoint: ' + (xhr.responseText || 'Unknown error'));
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
