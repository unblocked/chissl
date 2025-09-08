// New Tunnels Management View
console.log('🚨🚨🚨 NEW TUNNELS VIEW MODULE LOADED - VERSION 2024-09-07-01:35 - HOSTNAME + AUTO-REFRESH 🚨🚨🚨');

function loadNewTunnelsView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title">Active Tunnels</h3>' +
        '<div class="card-tools">' +
        '<button class="btn btn-secondary btn-sm" onclick="loadNewTunnelsData()" title="Refresh">' +
        '<i class="fas fa-sync-alt"></i> Refresh</button>' +
        '</div>' +
        '</div>' +
        '<div class="card-body">' +
        '<div class="table-responsive">' +
        '<table class="table table-bordered table-striped">' +
        '<thead><tr><th>URL</th><th>Local Port</th><th>Remote Port</th><th>User</th><th>Status</th><th>Connected</th><th>Actions</th></tr></thead>' +
        '<tbody id="new-tunnels-tbody"><tr><td colspan="7" class="text-center">Loading tunnels...</td></tr></tbody>' +
        '</table>' +
        '<div id="new-tunnels-info" class="mt-2 text-muted small"></div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('#main-content').html(content);
    loadNewTunnelsData();

    // Start auto-refresh every 10 seconds
    startNewTunnelsAutoRefresh();
}

// Auto-refresh functionality
var newTunnelsRefreshInterval = null;
// Track client-side hidden tunnels (soft-hide)
window.deletedTunnelIds = window.deletedTunnelIds || new Set();
var newTunnelsShowAll = false;


function startNewTunnelsAutoRefresh() {
    // Clear any existing interval
    if (newTunnelsRefreshInterval) {
        clearInterval(newTunnelsRefreshInterval);
    }

    // Start new interval
    newTunnelsRefreshInterval = setInterval(function() {
        loadNewTunnelsData();
    }, 10000); // 10 seconds

    console.log('🔄 New Tunnels auto-refresh started (10 seconds)');
}

function stopNewTunnelsAutoRefresh() {
    if (newTunnelsRefreshInterval) {
        clearInterval(newTunnelsRefreshInterval);
        newTunnelsRefreshInterval = null;
        console.log('⏹️ New Tunnels auto-refresh stopped');
    }
}

function loadNewTunnelsData() {
    $.get('/api/tunnels')
        .done(function(data) {
            var tbody = '';

            if (data && data.length > 0) {
                // Filter out client-side hidden tunnels
                var dataFiltered = data.filter(function(t){ return !(window.deletedTunnelIds && window.deletedTunnelIds.has(t.id)); });

                // Sort with active/open first, then by most recent updated/created time
                var order = { 'open': 0, 'active': 0, 'error': 1, 'closed': 2, 'inactive': 3 };
                var sorted = dataFiltered.slice().sort(function(a, b) {
                    var ra = order[(a.status || '').toLowerCase()] ?? 99;
                    var rb = order[(b.status || '').toLowerCase()] ?? 99;
                    if (ra !== rb) return ra - rb;
                    var ta = new Date(a.updated_at || a.created_at || 0).getTime();
                    var tb = new Date(b.updated_at || b.created_at || 0).getTime();
                    return tb - ta; // most recent first
                });

                // Limit to 15 rows (older/inactive implicitly hidden) unless 'Show all' is enabled
                var totalCount = sorted.length;
                var rows = newTunnelsShowAll ? sorted : sorted.slice(0, 15);

                rows.forEach(function(tunnel) {
                    var status = (tunnel.status || '').toLowerCase();
                    var statusClass = (status === 'open' || status === 'active') ? 'badge-success' :
                                      (status === 'error' ? 'badge-warning' : 'badge-secondary');
                    var statusText = (tunnel.status || 'Unknown');
                    statusText = statusText.charAt(0).toUpperCase() + statusText.slice(1);

                    var connectedTime = (tunnel.updated_at || tunnel.created_at) ? new Date(tunnel.updated_at || tunnel.created_at).toLocaleString() : 'Never';

                    var url = 'http://' + window.location.hostname + ':' + (tunnel.local_port || '?');

                    tbody += '<tr>' +
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
                        '<td><code class="text-dark">' + (tunnel.local_port || '?') + '</code></td>' +
                        '<td><code class="text-dark">' + (tunnel.remote_port || '?') + '</code></td>' +
                        '<td>' + escapeHtml(tunnel.username || 'Unknown') + '</td>' +
                        '<td><span class="badge ' + statusClass + '">' + statusText + '</span></td>' +
                        '<td><small>' + connectedTime + '</small></td>' +
                        '<td>' +
                        '<div class="btn-group btn-group-sm" role="group">' +
                        '<button class="btn btn-outline-info" onclick="showTrafficPayloads(\'' + tunnel.id + '\', \'tunnel\')" title="View Traffic">' +
                        '<i class="fas fa-eye"></i></button>' +
                        '<button class="btn btn-outline-danger" onclick="deleteNewTunnel(\'' + tunnel.id + '\', this)" title="Close Tunnel">' +
                        '<i class="fas fa-times"></i></button>' +
                        '</div>' +
                        '</td>' +
                        '</tr>';
                });
            } else {
                tbody = '<tr><td colspan="7" class="text-center text-muted">No active tunnels found</td></tr>';
                $('#new-tunnels-info').html('');
            }
            $('#new-tunnels-tbody').html(tbody);
            // Info footer with toggle
            try {
                var infoHtml = '';
                var total = (sorted && sorted.length) ? sorted.length : (data ? data.length : 0);
                if (total > 15) {
                    if (newTunnelsShowAll) {
                        infoHtml = '<span>Showing all ' + total + ' tunnels.</span> ' +
                                   '<a href="#" onclick="return toggleNewTunnelsShowAll(false)">Show less</a>';
                    } else {
                        var shown = Math.min(15, total);
                        infoHtml = '<span>Showing ' + shown + ' of ' + total + ' (older inactive hidden).</span> ' +
                                   '<a href="#" onclick="return toggleNewTunnelsShowAll(true)">Show all</a>';
                    }
                } else if (total > 0) {
                    infoHtml = '<span>Showing all ' + total + ' tunnels.</span>';
                }
                $('#new-tunnels-info').html(infoHtml);
            } catch(e) { /* no-op */ }
        })
        .fail(function() {
            $('#new-tunnels-tbody').html('<tr><td colspan="7" class="text-center text-danger">Failed to load tunnels</td></tr>');
        });
}
// Toggle show-all for new tunnels view
function toggleNewTunnelsShowAll(showAll) {
    if (typeof showAll === 'boolean') {
        newTunnelsShowAll = showAll;
    } else {
        newTunnelsShowAll = !newTunnelsShowAll;
    }
    loadNewTunnelsData();
    return false;
}


function deleteNewTunnel(tunnelId, btn) {
    if (confirm('Are you sure you want to close this tunnel? This will terminate the connection.')) {
        $.ajax({
            url: '/api/tunnels/' + encodeURIComponent(tunnelId),
            method: 'DELETE'
        })
        .done(function() {
            // Optimistically hide this tunnel in client and refresh table
            try {
                if (window.deletedTunnelIds) { window.deletedTunnelIds.add(tunnelId); }
                if (btn) $(btn).closest('tr').remove();
            } catch(e){}
            loadNewTunnelsData();
        })
        .fail(function(xhr) {
            alert('Failed to close tunnel: ' + (xhr.responseText || 'Unknown error'));
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
