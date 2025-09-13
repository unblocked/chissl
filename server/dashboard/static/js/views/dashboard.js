// Dashboard Main View
// Handles the main dashboard page with stats and quick access

console.log('Dashboard view module loaded');

// Load dashboard data
function loadDashboard() {
    // Load the main dashboard content first
    var content = '<div class="row">' +
        '<div class="col-lg-3 col-6">' +
        '<div class="small-box bg-info">' +
        '<div class="inner"><h3 id="total-tunnels">0</h3><p>My Tunnels</p></div>' +
        '<div class="icon"><i class="fas fa-exchange-alt"></i></div>' +
        '</div></div>' +
        '<div class="col-lg-3 col-6">' +
        '<div class="small-box bg-success">' +
        '<div class="inner"><h3 id="active-tunnels">0</h3><p>Active</p></div>' +
        '<div class="icon"><i class="fas fa-plug"></i></div>' +
        '</div></div>' +
        '<div class="col-lg-3 col-6">' +
        '<div class="small-box bg-warning">' +
        '<div class="inner"><h3 id="total-listeners">0</h3><p>My Listeners</p></div>' +
        '<div class="icon"><i class="fas fa-satellite-dish"></i></div>' +
        '</div></div>' +
        '<div class="col-lg-3 col-6">' +
        '<div class="small-box bg-danger">' +
        '<div class="inner"><h3 id="active-listeners">0</h3><p>Active</p></div>' +
        '<div class="icon"><i class="fas fa-broadcast-tower"></i></div>' +
        '</div></div>' +
        '</div>' +
        '<div class="row">' +
        '<div class="col-12">' +
        '<div id="sso-banner"></div>' +
        '</div>' +
        '</div>' +
        '<div class="row">' +
        '<div class="col-12">' +
        '<div id="port-access-banner"></div>' +
        '</div>' +
        '</div>' +

        '<div class="row">' +
        '<div class="col-md-6">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title"><i class="fas fa-clock"></i> Recent Tunnels</h3>' +
        '</div>' +
        '<div class="card-body" id="recent-tunnels">' +
        '<div class="text-center text-muted">Loading recent tunnels...</div>' +
        '</div></div></div>' +
        '<div class="col-md-6">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title"><i class="fas fa-satellite-dish"></i> Recent Listeners</h3>' +
        '</div>' +
        '<div class="card-body" id="recent-listeners">' +
        '<div class="text-center text-muted">Loading recent listeners...</div>' +
        '</div></div></div>' +
        '</div>';

    $('#main-content').html(content);
    loadStats();
    loadQuickAccess();
    loadSSOBanner();
    loadPortAccessBanner();
}

function loadStats() {
    // Load user-specific stats for the dashboard
    $.get('/api/stats?user=true')
        .done(function(data) {
            $('#total-tunnels').text(data.total_tunnels || 0);
            $('#active-tunnels').text(data.active_tunnels || 0);
            $('#total-listeners').text(data.total_listeners || 0);
            $('#active-listeners').text(data.active_listeners || 0);
        })
        .fail(function() {
            console.error('Failed to load stats');
        });
}

function loadQuickAccess() {
    // Load recent tunnels
    $.get('/api/tunnels')
        .done(function(data) {
            var tunnelsHtml = '';
            if (data && data.length > 0) {
                data.slice(0, 5).forEach(function(tunnel) {
                    var statusClass = tunnel.connected ? 'text-success' : 'text-danger';
                    var statusIcon = tunnel.connected ? 'fas fa-check-circle' : 'fas fa-times-circle';
                    tunnelsHtml += '<div class="d-flex justify-content-between align-items-center border-bottom py-2">' +
                        '<div>' +
                        '<strong>' + tunnel.remote + '</strong><br>' +
                        '<small class="text-muted">' + tunnel.local + '</small>' +
                        '</div>' +
                        '<span class="' + statusClass + '"><i class="' + statusIcon + '"></i></span>' +
                        '</div>';
                });
            } else {
                tunnelsHtml = '<div class="text-center text-muted">No recent tunnels</div>';
            }
            $('#recent-tunnels').html(tunnelsHtml);
        })
        .fail(function() {
            $('#recent-tunnels').html('<div class="text-center text-danger">Failed to load tunnels</div>');
        });

    // Load recent listeners
    $.get('/api/listeners')
        .done(function(data) {
            var listenersHtml = '';
            if (data && data.length > 0) {
                data.slice(0, 5).forEach(function(listener) {
                    var statusClass = listener.active ? 'text-success' : 'text-danger';
                    var statusIcon = listener.active ? 'fas fa-check-circle' : 'fas fa-times-circle';
                    listenersHtml += '<div class="d-flex justify-content-between align-items-center border-bottom py-2">' +
                        '<div>' +
                        '<strong>' + listener.name + '</strong><br>' +
                        '<small class="text-muted">Port ' + listener.port + '</small>' +
                        '</div>' +
                        '<span class="' + statusClass + '"><i class="' + statusIcon + '"></i></span>' +
                        '</div>';
                });
            } else {
                listenersHtml = '<div class="text-center text-muted">No recent listeners</div>';
            }
            $('#recent-listeners').html(listenersHtml);
        })
        .fail(function() {
            $('#recent-listeners').html('<div class="text-center text-danger">Failed to load listeners</div>');
        });
}

function loadPortAccessBanner() {
    // Load user's port access information as a permanent banner
    $.get('/api/user/port-reservations')
        .done(function(data) {
            var bannerHtml = '';
            if (data && data.length > 0) {
                bannerHtml = '<div class="alert alert-success mb-3">' +
                    '<h6><i class="fas fa-check-circle"></i> You have reserved ports assigned</h6>' +
                    '<div class="row">';
                data.forEach(function(reservation, index) {
                    if (index > 0 && index % 3 === 0) {
                        bannerHtml += '</div><div class="row">';
                    }
                    bannerHtml += '<div class="col-md-4 mb-2">' +
                        '<strong>Ports ' + reservation.start_port + '-' + reservation.end_port + '</strong>';
                    if (reservation.description) {
                        bannerHtml += '<br><small class="text-muted">' + reservation.description + '</small>';
                    }
                    bannerHtml += '</div>';
                });
                bannerHtml += '</div>' +
                    '<small class="text-success d-block mt-2">' +
                    '<i class="fas fa-info-circle"></i> You can create tunnels and listeners on these reserved ports in addition to unreserved ports.' +
                    '</small>' +
                    '</div>';
            } else {
                // Show general port info for users without reservations
                loadGeneralPortBanner();
                return;
            }
            $('#port-access-banner').html(bannerHtml);
        })
        .fail(function() {
            // If we can't load user reservations, still show general port info
            loadGeneralPortBanner();
        });
}

function loadGeneralPortBanner() {
    // Show general port info as a banner for users without reservations
    $.get('/api/user/reserved-ports-threshold')
        .done(function(thresholdData) {
            var threshold = thresholdData.threshold || 10000;
            var bannerHtml = '<div class="alert alert-info mb-3">' +
                '<h6><i class="fas fa-info-circle"></i> Available Ports</h6>' +
                '<div><strong>You can use ports ' + threshold + ' and above</strong></div>' +
                '<small class="text-info d-block mt-2">' +
                '<i class="fas fa-info-circle"></i> Ports 0-' + (threshold-1) + ' are reserved for admin assignment. ' +
                'Contact your administrator if you need access to reserved ports.' +
                '</small>' +
                '</div>';
            $('#port-access-banner').html(bannerHtml);
        })
        .fail(function() {
            var bannerHtml = '<div class="alert alert-info mb-3">' +
                '<h6><i class="fas fa-info-circle"></i> Available Ports</h6>' +
                '<div><strong>You can use ports 10000 and above</strong></div>' +
                '<small class="text-info d-block mt-2">' +
                '<i class="fas fa-info-circle"></i> Ports 0-9999 are reserved for admin assignment. ' +
                'Contact your administrator if you need access to reserved ports.' +
                '</small>' +
                '</div>';
            $('#port-access-banner').html(bannerHtml);
        });
}
