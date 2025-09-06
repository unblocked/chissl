// Tunnels View
// Handles the tunnels management page

console.log('Tunnels view module loaded');

function loadTunnelsView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">'
        + '<h3 class="card-title">Active Tunnels</h3>'
        + '<div class="card-tools">'
        + '<button class="btn btn-primary btn-sm" onclick="loadTunnelsData()">'
        + '<i class="fas fa-sync"></i> Refresh'
        + '</button>'
        + '</div>'
        + '</div>'
        + '<div class="card-body">'
        + '<div id="sso-banner"></div>'
        + '<div id="tunnels-list">'
        + '<div class="text-center text-muted">Loading tunnels...</div>'
        + '</div>'
        + '</div>'
        + '</div>'
        + '</div>'
        + '</div>';

    $('#main-content').html(content);
    loadTunnelsData();
    loadSSOBannerForTunnels();
}

function loadTunnelsData() {
    $.get('/api/tunnels')
        .done(function(data) {
            var tunnelsHtml = '';
            if (data && data.length > 0) {
                tunnelsHtml = '<div class="table-responsive">' +
                    '<table class="table table-striped">' +
                    '<thead>' +
                    '<tr>' +
                    '<th>Local</th>' +
                    '<th>Remote</th>' +
                    '<th>Status</th>' +
                    '<th>Connected</th>' +
                    '<th>Bytes Sent</th>' +
                    '<th>Bytes Received</th>' +
                    '<th>Actions</th>' +
                    '</tr>' +
                    '</thead>' +
                    '<tbody>';

                data.forEach(function(tunnel) {
                    var statusClass = tunnel.connected ? 'badge-success' : 'badge-danger';
                    var statusText = tunnel.connected ? 'Connected' : 'Disconnected';
                    var connectedTime = tunnel.connected_at ? new Date(tunnel.connected_at).toLocaleString() : 'Never';
                    
                    tunnelsHtml += '<tr>' +
                        '<td><code>' + tunnel.local + '</code></td>' +
                        '<td><code>' + tunnel.remote + '</code></td>' +
                        '<td><span class="badge ' + statusClass + '">' + statusText + '</span></td>' +
                        '<td><small>' + connectedTime + '</small></td>' +
                        '<td>' + formatBytes(tunnel.bytes_sent || 0) + '</td>' +
                        '<td>' + formatBytes(tunnel.bytes_recv || 0) + '</td>' +
                        '<td>' +
                        '<div class="btn-group btn-group-sm" role="group">' +
                        '<button class="btn btn-outline-info" onclick="showTrafficPayloads(\'' + tunnel.id + '\', \'tunnel\')" title="View Traffic">' +
                        '<i class="fas fa-eye"></i>' +
                        '</button>' +
                        '<button class="btn btn-outline-danger" onclick="deleteTunnel(\'' + tunnel.id + '\')" title="Close Tunnel">' +
                        '<i class="fas fa-times"></i>' +
                        '</button>' +
                        '</div>' +
                        '</td>' +
                        '</tr>';
                });

                tunnelsHtml += '</tbody></table></div>';
            } else {
                tunnelsHtml = '<div class="text-center text-muted py-4">' +
                    '<i class="fas fa-exchange-alt fa-3x mb-3"></i>' +
                    '<h5>No Active Tunnels</h5>' +
                    '<p>Connect using the chiSSL client to see tunnels here.</p>' +
                    '</div>';
            }
            $('#tunnels-list').html(tunnelsHtml);
        })
        .fail(function() {
            $('#tunnels-list').html('<div class="alert alert-danger">Failed to load tunnels</div>');
        });
}

// Tunnel management functions
function deleteTunnel(tunnelId) {
    if (confirm('Are you sure you want to close this tunnel? This will terminate the connection.')) {
        $.ajax({
            url: '/api/tunnel/' + tunnelId,
            method: 'DELETE'
        })
        .done(function() {
            // Reload tunnels data
            loadTunnelsData();
            // Show success message
            var toast = $('<div class="toast" style="position: fixed; top: 20px; right: 20px; z-index: 9999; background: #28a745; color: white; padding: 10px 15px; border-radius: 4px;">Tunnel closed successfully!</div>');
            $('body').append(toast);
            setTimeout(function() {
                toast.fadeOut(function() {
                    toast.remove();
                });
            }, 3000);
        })
        .fail(function(xhr) {
            var errorMsg = 'Failed to close tunnel';
            if (xhr.responseJSON && xhr.responseJSON.error) {
                errorMsg = xhr.responseJSON.error;
            }
            alert('Error: ' + errorMsg);
        });
    }
}

// Backward compatibility function for tunnels
function showTunnelPayloads(tunnelId) {
    return showTrafficPayloads(tunnelId, 'tunnel');
}
