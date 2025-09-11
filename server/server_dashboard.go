package chserver

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// handleDashboard serves the AdminLTE dashboard
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle static assets first (no authentication required)
	if filepath.Ext(path) != "" || strings.Contains(path, "/static/") {
		s.serveDashboardAsset(w, r)
		return
	}

	// Check authentication for all other dashboard pages
	if !s.isAuthenticated(r) {
		s.serveDashboardLogin(w, r)
		return
	}

	if path == "/dashboard" || path == "/dashboard/" {
		s.serveDashboardIndex(w, r)
		return
	}

	if path == "/dashboard/logout" {
		s.handleDashboardLogout(w, r)
		return
	}

	// Default to index
	s.serveDashboardIndex(w, r)
}

// isAuthenticated checks if the request is authenticated
func (s *Server) isAuthenticated(r *http.Request) bool {
	// Check for session cookie first
	if cookie, err := r.Cookie("chissl_session"); err == nil && cookie.Value != "" {
		username := cookie.Value
		// Validate the session user exists (admin or regular user)
		_, found := s.users.Get(username)
		if found {
			return true
		}
		if s.db != nil {
			_, err := s.db.GetUser(username)
			if err == nil {
				return true
			}
		}
	}

	// Check for basic auth
	username, password, ok := s.decodeBasicAuthHeader(r.Header)
	if ok {
		// Check in-memory users first
		user, found := s.users.Get(username)
		if found && user.Name == username && user.Pass == password {
			return true
		}
		// Then check database
		if s.db != nil {
			dbUser, err := s.db.GetUser(username)
			if err == nil && dbUser.Password == password {
				return true
			}
		}
	}

	return false
}

// serveDashboardIndex serves the main dashboard page
func (s *Server) serveDashboardIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>chiSSL</title>

    <!-- AdminLTE CSS -->
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/admin-lte@3.2/dist/css/adminlte.min.css">
    <!-- Font Awesome -->
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css">
    <!-- Bootstrap 4.6 (AdminLTE 3 compatible) -->
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@4.6.2/dist/css/bootstrap.min.css" rel="stylesheet">
</head>
<body class="hold-transition sidebar-mini layout-fixed">
<div class="wrapper">
    <!-- Navbar -->
    <nav class="main-header navbar navbar-expand navbar-white navbar-light">
        <ul class="navbar-nav">
            <li class="nav-item">
                <a class="nav-link" data-widget="pushmenu" href="#" role="button"><i class="fas fa-bars"></i></a>
            </li>
        </ul>
        <ul class="navbar-nav ml-auto">
            <li class="nav-item">
                <a class="nav-link" href="/dashboard/logout">
                    <i class="fas fa-sign-out-alt"></i> Logout
                </a>
            </li>
        </ul>
    </nav>

    <!-- Sidebar -->
    <aside class="main-sidebar sidebar-dark-primary elevation-4">
        <a href="#" class="brand-link" onclick="showDashboard()">
            <img src="/dashboard/static/images/chissl-66-100.png" alt="chiSSL Logo" class="brand-image" style="width: 33px; height: auto; opacity: .8;">
            <span class="brand-text font-weight-light">chiSSL</span>
        </a>
        <div class="text-center" style="padding: 5px 10px; border-bottom: 1px solid #4f5962;">
            <small class="text-muted">brought to you by <a href="https://getunblocked.com" target="_blank" class="text-info">Unblocked</a></small>
        </div>

        <div class="sidebar">
            <nav class="mt-2">
                <ul class="nav nav-pills nav-sidebar flex-column" data-widget="treeview" role="menu">
                    <li class="nav-item">
                        <a href="#" class="nav-link active" onclick="showDashboard()">
                            <i class="nav-icon fas fa-tachometer-alt"></i>
                            <p>Dashboard</p>
                        </a>
                    </li>
                    <li class="nav-item">
                        <a href="#" class="nav-link" onclick="showTunnels()">
                            <i class="nav-icon fas fa-network-wired"></i>
                            <p>Tunnels</p>
                        </a>
                    </li>
                    <li class="nav-item admin-only" style="display: none;">
                        <a href="#" class="nav-link" onclick="showUsers()">
                            <i class="nav-icon fas fa-users"></i>
                            <p>Users</p>
                        </a>
                    </li>
                    <li class="nav-item">
                        <a href="#" class="nav-link" onclick="showListeners()">
                            <i class="nav-icon fas fa-satellite-dish"></i>
                            <p>Listeners</p>
                        </a>
                    </li>
                    <li class="nav-item admin-only" style="display: none;">
                        <a href="#" class="nav-link" onclick="showLogs()">
                            <i class="nav-icon fas fa-file-alt"></i>
                            <p>Logs</p>
                        </a>
                    </li>
                    <li class="nav-item">
                        <a href="#" class="nav-link" onclick="showUserSettings()">
                            <i class="nav-icon fas fa-user-cog"></i>
                            <p>User Settings</p>
                        </a>
                    </li>
                    <li class="nav-item admin-only" style="display: none;">
                        <a href="#" class="nav-link" onclick="showServerSettings()">
                            <i class="nav-icon fas fa-server"></i>
                            <p>Server Settings</p>
                        </a>
                    </li>
                </ul>
            </nav>
        </div>
    </aside>

    <!-- Content Wrapper -->
    <div class="content-wrapper">
        <div class="content-header">
            <div class="container-fluid">
                <div class="row mb-2">
                    <div class="col-sm-6">
                        <h1 class="m-0" id="page-title">Dashboard</h1>
                        <p class="text-muted" id="welcome-message" style="display: none;"></p>
                    </div>
                </div>
            </div>
        </div>

        <!-- Main content -->
        <section class="content">
            <div class="container-fluid" id="main-content">
                <!-- Dashboard content will be loaded here -->
                <div class="row">
                    <div class="col-lg-3 col-6">
                        <div class="small-box bg-info">
                            <div class="inner">
                                <h3 id="active-tunnels">0</h3>
                                <p>Active Tunnels</p>
                            </div>
                            <div class="icon">
                                <i class="fas fa-network-wired"></i>
                            </div>
                        </div>
                    </div>
                    <div class="col-lg-3 col-6">
                        <div class="small-box bg-success">
                            <div class="inner">
                                <h3 id="total-users">0</h3>
                                <p>Total Users</p>
                            </div>
                            <div class="icon">
                                <i class="fas fa-users"></i>
                            </div>
                        </div>
                    </div>
                    <div class="col-lg-3 col-6">
                        <div class="small-box bg-warning">
                            <div class="inner">
                                <h3 id="active-sessions">0</h3>
                                <p>Active Sessions</p>
                            </div>
                            <div class="icon">
                                <i class="fas fa-user-clock"></i>
                            </div>
                        </div>
                    </div>
                    <div class="col-lg-3 col-6">
                        <div class="small-box bg-danger">
                            <div class="inner">
                                <h3 id="total-connections">0</h3>
                                <p>Total Connections</p>
                            </div>
                            <div class="icon">
                                <i class="fas fa-plug"></i>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Charts and tables will be added here -->
                <div class="row">
                    <div class="col-md-12">
                        <div class="card">
                            <div class="card-header">
                                <h3 class="card-title">System Information</h3>
                            </div>
                            <div class="card-body">
                                <table class="table table-bordered">
                                    <tr>
                                        <td><strong>Server Uptime</strong></td>
                                        <td id="uptime">Loading...</td>
                                    </tr>
                                    <tr>
                                        <td><strong>Version</strong></td>
                                        <td id="version">Loading...</td>
                                    </tr>
                                    <tr>
                                        <td><strong>Database</strong></td>
                                        <td id="database">Loading...</td>
                                    </tr>
                                    <tr>
                                        <td><strong>Auth0 Integration</strong></td>
                                        <td id="auth0">Loading...</td>
                                    </tr>
                                </table>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </section>
    </div>

    <!-- Footer -->
    <footer class="main-footer">
        <strong>Copyright &copy; 2024 chiSSL.</strong> All rights reserved.
    </footer>
</div>

<!-- Scripts -->
<script src="https://code.jquery.com/jquery-3.6.0.min.js"></script>
<!-- jQuery must come first -->
<script src="https://code.jquery.com/jquery-3.6.0.min.js"></script>
<!-- Bootstrap 4.6 (AdminLTE 3 requires BS4) -->
<script src="https://cdn.jsdelivr.net/npm/bootstrap@4.6.2/dist/js/bootstrap.bundle.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/admin-lte@3.2/dist/js/adminlte.min.js"></script>

<!-- Dashboard JavaScript Modules -->
<script src="/dashboard/static/js/core/app.js"></script>
<script src="/dashboard/static/js/core/auto-refresh.js"></script>
<script src="/dashboard/static/js/core/navigation.js"></script>
<script src="/dashboard/static/js/views/dashboard.js"></script>
<script src="/dashboard/static/js/views/tunnels.js"></script>
<script src="/dashboard/static/js/views/users.js"></script>
<script src="/dashboard/static/js/views/listeners.js"></script>
<script src="/dashboard/static/js/views/logs.js"></script>
<script src="/dashboard/static/js/views/user-settings.js"></script>
<script src="/dashboard/static/js/views/server-settings.js"></script>
<script src="/dashboard/static/js/components/sso.js"></script>
<script src="/dashboard/static/js/components/port-reservations.js"></script>
<script src="/dashboard/static/js/utils/traffic-inspector.js"></script>

<script>
// Remaining inline JavaScript (to be modularized)
console.log('Dashboard legacy script loaded');

// Set up AJAX defaults with credentials
$.ajaxSetup({
    xhrFields: {
        withCredentials: true
    },
    beforeSend: function(xhr) {
        // Include session cookie automatically
        // For basic auth fallback, we could add Authorization header here
    }
});

// Load dashboard data
function loadDashboard() {
    // Load the main dashboard content first
    var content = '<div class="row">' +
        '<div class="col-lg-3 col-6">' +
        '<div class="small-box bg-info">' +
        '<div class="inner"><h3 id="total-tunnels">0</h3><p>My Tunnels</p></div>' +
        '<div class="icon"><i class="fas fa-list"></i></div>' +
        '</div></div>' +
        '<div class="col-lg-3 col-6">' +
        '<div class="small-box bg-success">' +
        '<div class="inner"><h3 id="active-tunnels">0</h3><p>Active Tunnels</p></div>' +
        '<div class="icon"><i class="fas fa-exchange-alt"></i></div>' +
        '</div></div>' +
        '<div class="col-lg-3 col-6">' +
        '<div class="small-box bg-warning">' +
        '<div class="inner"><h3 id="total-listeners">0</h3><p>My Listeners</p></div>' +
        '<div class="icon"><i class="fas fa-server"></i></div>' +
        '</div></div>' +
        '<div class="col-lg-3 col-6">' +
        '<div class="small-box bg-danger">' +
        '<div class="inner"><h3 id="active-listeners">0</h3><p>Active Listeners</p></div>' +
        '<div class="icon"><i class="fas fa-broadcast-tower"></i></div>' +
        '</div></div></div>' +
        '<div id="sso-banner-row" class="row">' +
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
        '<h3 class="card-title">Quick Access - Tunnels</h3>' +
        '<div class="card-tools">' +
        '<button class="btn btn-primary btn-sm" onclick="showTunnels()">' +
        '<i class="fas fa-list"></i> View All</button>' +
        '</div>' +
        '</div>' +
        '<div class="card-body" id="quick-tunnels">' +
        '<div class="text-center text-muted">Loading...</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="col-md-6">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title">Quick Access - Listeners</h3>' +
        '<div class="card-tools">' +
        '<button class="btn btn-primary btn-sm" onclick="showListeners()">' +
        '<i class="fas fa-list"></i> View All</button>' +
        '</div>' +
        '</div>' +
        '<div class="card-body" id="quick-listeners">' +
        '<div class="text-center text-muted">Loading...</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="row" id="port-reservations-row" style="display: none;">' +
        '<div class="col-md-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title">Port Reservations</h3>' +
        '</div>' +
        '<div class="card-body" id="port-reservations">' +
        '<div class="text-center text-muted">Loading...</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="row mt-4">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title"><i class="fas fa-download"></i> Resources</h3>' +
        '</div>' +
        '<div class="card-body">' +
        '<div class="row">' +
        '<div class="col-md-6">' +
        '<h6><i class="fab fa-github"></i> Open Source</h6>' +
        '<p class="text-muted">chiSSL is open source and available on GitHub</p>' +
        '<a href="https://github.com/unblocked/chissl" target="_blank" class="btn btn-outline-dark">' +
        '<i class="fab fa-github"></i> View on GitHub</a>' +
        '</div>' +
        '<div class="col-md-6">' +
        '<h6><i class="fas fa-download"></i> Client Downloads</h6>' +
        '<p class="text-muted">Download the chiSSL client for your platform</p>' +
        '<div class="btn-group-vertical btn-group-sm" style="width: 100%;">' +
        '<button class="btn btn-outline-primary mb-1" onclick="alert(\'Coming soon!\')">' +
        '<i class="fab fa-windows"></i> Windows (x64)</button>' +
        '<button class="btn btn-outline-primary mb-1" onclick="alert(\'Coming soon!\')">' +
        '<i class="fab fa-apple"></i> macOS (Intel/Apple Silicon)</button>' +
        '<button class="btn btn-outline-primary mb-1" onclick="alert(\'Coming soon!\')">' +
        '<i class="fab fa-linux"></i> Linux (x64)</button>' +
        '<button class="btn btn-outline-primary" onclick="alert(\'Coming soon!\')">' +
        '<i class="fas fa-code"></i> Source Code</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div></div></div>';

    $('#main-content').html(content);
    loadStats();
    loadQuickAccess();
    loadSSOBanner();
    loadPortAccessBanner();

    // Start component-based auto-refresh for dashboard
    startDashboardAutoRefresh();
}

function startDashboardAutoRefresh() {
    // Stop any existing auto-refresh
    if (typeof AutoRefresh !== 'undefined') {
        AutoRefresh.stopAll();

        // Start auto-refresh for dashboard stats (every 15 seconds)
        AutoRefresh.start('dashboard-stats', function() {
            loadStats();
        }, 15000);

        // Start auto-refresh for quick access tables (every 20 seconds)
        AutoRefresh.start('dashboard-tables', function() {
            loadQuickAccess();
        }, 20000);
    }
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
            var quickTunnels = '';
            if (data && data.length > 0) {
                var recentTunnels = data.slice(0, 3); // Show last 3
                recentTunnels.forEach(function(tunnel) {
                    var statusBadge = tunnel.status === 'open' ? 'success' : 'secondary';
                    quickTunnels += '<div class="d-flex justify-content-between align-items-center mb-2">' +
                        '<div>' +
                        '<strong>' + tunnel.id.substring(0, 8) + '...</strong><br>' +
                        '<small class="text-muted">Port ' + tunnel.local_port + ' → ' + tunnel.remote_port + '</small>' +
                        '</div>' +
                        '<div>' +
                        '<span class="badge badge-' + statusBadge + '">' + tunnel.status + '</span>' +
                        '<button class="btn btn-sm btn-outline-info ml-1" onclick="showTunnelPayloads(\'' + tunnel.id + '\')" title="Inspect Traffic">' +
                        '<i class="fas fa-eye"></i></button>' +
                        '</div>' +
                        '</div>';
                });
            } else {
                quickTunnels = '<div class="text-center text-muted">No tunnels found</div>';
            }
            $('#quick-tunnels').html(quickTunnels);
        })
        .fail(function() {
            $('#quick-tunnels').html('<div class="text-center text-danger">Failed to load tunnels</div>');
        });

    // Load recent listeners
    $.get('/api/listeners')
        .done(function(data) {
            var quickListeners = '';
            if (data && data.length > 0) {
                var recentListeners = data.slice(0, 3); // Show last 3
                recentListeners.forEach(function(listener) {
                    var statusBadge = listener.status === 'open' ? 'success' : 'secondary';
                    var protocol = listener.use_tls ? 'https' : 'http';
                    var displayName = listener.name || ('Listener ' + listener.port);
                    quickListeners += '<div class="d-flex justify-content-between align-items-center mb-2">' +
                        '<div>' +
                        '<strong>' + displayName + '</strong><br>' +
                        '<small class="text-muted">' + protocol + '://...:'+ listener.port + '</small>' +
                        '</div>' +
                        '<div>' +
                        '<span class="badge badge-' + statusBadge + '">' + listener.status + '</span>' +
                        '<button class="btn btn-sm btn-outline-info ml-1" onclick="showTrafficPayloads(\'' + listener.id + '\', \'listener\')" title="Inspect Traffic">' +
                        '<i class="fas fa-eye"></i></button>' +
                        '</div>' +
                        '</div>';
                });
            } else {
                quickListeners = '<div class="text-center text-muted">No listeners found</div>';
            }
            $('#quick-listeners').html(quickListeners);
        })
        .fail(function() {
            $('#quick-listeners').html('<div class="text-center text-danger">Failed to load listeners</div>');
        });
}

function loadPortReservations() {
    // Show port information banner on main dashboard
    $.get('/api/user/port-reservations')
        .done(function(data) {
            var bannerHtml = '';
            if (data && data.length > 0) {
                bannerHtml = '<div class="alert alert-success alert-dismissible">' +
                    '<button type="button" class="close" data-dismiss="alert" aria-hidden="true">&times;</button>' +
                    '<h6><i class="fas fa-check-circle"></i> Reserved Ports Available</h6>' +
                    '<div class="row">';
                data.forEach(function(reservation) {
                    bannerHtml += '<div class="col-md-4 mb-1">' +
                        '<strong>Ports ' + reservation.start_port + '-' + reservation.end_port + '</strong>';
                    if (reservation.description) {
                        bannerHtml += '<br><small class="text-muted">' + reservation.description + '</small>';
                    }
                    bannerHtml += '</div>';
                });
                bannerHtml += '</div>' +
                    '<small>You can create tunnels and listeners on these reserved ports in addition to unreserved ports.</small>' +
                    '</div>';
            } else {
                // Show general port info for users without reservations
                $.get('/api/user/reserved-ports-threshold')
                    .done(function(thresholdData) {
                        var threshold = thresholdData.threshold || 10000;
                        bannerHtml = '<div class="alert alert-info alert-dismissible">' +
                            '<button type="button" class="close" data-dismiss="alert" aria-hidden="true">&times;</button>' +
                            '<h6><i class="fas fa-info-circle"></i> Available Ports</h6>' +
                            '<div><strong>You can use ports ' + threshold + ' and above</strong></div>' +
                            '<small class="text-muted">Ports 0-' + (threshold-1) + ' are reserved for admin assignment. Contact admin for reserved port access.</small>' +
                            '</div>';
                        $('#port-info-banner').html(bannerHtml);
                    })
                    .fail(function() {
                        // Don't show anything if we can't get threshold
                    });
                return; // Exit early for users without reservations
            }
            $('#port-info-banner').html(bannerHtml);
        })
        .fail(function() {
            // Don't show error for port reservations as it's optional
        });
}

function formatUptime(seconds) {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return days + 'd ' + hours + 'h ' + minutes + 'm';
}

// URL routing and history management
function updateURL(view) {
    // Don't update URL if we're navigating from URL (prevents loops)
    if (window.isNavigatingFromURL) {
        return;
    }

    if (history.pushState) {
        var newUrl = window.location.pathname + '#' + view;
        history.pushState({view: view}, '', newUrl);
    } else {
        window.location.hash = view;
    }
}

function loadViewFromURL() {
    var hash = window.location.hash.substring(1); // Remove #
    if (!hash) {
        hash = 'dashboard'; // Default view
    }

    // Set flag to prevent URL updates during URL-based navigation
    window.isNavigatingFromURL = true;

    switch(hash) {
        case 'dashboard':
            showDashboardFromURL();
            break;
        case 'tunnels':
            showTunnelsFromURL();
            break;
        case 'listeners':
            showListenersFromURL();
            break;
        case 'users':
            if (window.currentUser && window.currentUser.is_admin) {
                showUsersFromURL();
            } else {
                showDashboardFromURL();
            }
            break;
        case 'logs':
            if (window.currentUser && window.currentUser.is_admin) {
                showLogsFromURL();
            } else {
                showDashboardFromURL();
            }
            break;
        case 'user-settings':
            showUserSettingsFromURL();
            break;
        case 'server-settings':
            if (window.currentUser && window.currentUser.is_admin) {
                showServerSettingsFromURL();
            } else {
                showDashboardFromURL();
            }
            break;
        default:
            showDashboardFromURL();
    }

    // Clear flag after navigation
    window.isNavigatingFromURL = false;
}

// Navigation functions
function clearLogsRefreshInterval() {
    if (window.logsRefreshInterval) {
        clearInterval(window.logsRefreshInterval);
        window.logsRefreshInterval = null;
    }
}

function stopAllAutoRefresh() {
    if (typeof AutoRefresh !== 'undefined') {
        AutoRefresh.stopAll();
    }
}

function showDashboard() {
    clearLogsRefreshInterval();
    stopAllAutoRefresh();
    window.currentView = 'dashboard';
    $('#page-title').text('Dashboard');
    $('.nav-link').removeClass('active');
    $('[onclick="showDashboard()"]').addClass('active');
    loadDashboard();
}

function showTunnels() {
    clearLogsRefreshInterval();
    stopAllAutoRefresh();
    window.currentView = 'tunnels';
    $('#page-title').text('Tunnels');
    $('.nav-link').removeClass('active');
    $('[onclick="showTunnels()"]').addClass('active');
    loadTunnelsView();
}

function showUsers() {
    clearLogsRefreshInterval();
    window.currentView = 'users';
    $('#page-title').text('Users');
    $('.nav-link').removeClass('active');
    $('[onclick="showUsers()"]').addClass('active');
    loadUsersView();
}

function showListeners() {
    clearLogsRefreshInterval();
    stopAllAutoRefresh();
    window.currentView = 'listeners';
    $('#page-title').text('Listeners');
    $('.nav-link').removeClass('active');
    $('[onclick="showListeners()"]').addClass('active');
    loadListenersView();
}

function showLogs() {
    // Don't clear interval for logs page - we want auto-refresh here
    window.currentView = 'logs';
    $('#page-title').text('Logs');
    $('.nav-link').removeClass('active');
    $('[onclick="showLogs()"]').addClass('active');
    loadLogsView();
}

function showUserSettings() {
    clearLogsRefreshInterval();
    window.currentView = 'user-settings';
    $('#page-title').text('User Settings');
    $('.nav-link').removeClass('active');
    $('[onclick="showUserSettings()"]').addClass('active');
    loadUserSettingsView();
}

function showServerSettings() {
    clearLogsRefreshInterval();
    window.currentView = 'server-settings';
    $('#page-title').text('Server Settings');
    $('.nav-link').removeClass('active');
    $('[onclick="showServerSettings()"]').addClass('active');
    loadServerSettingsView();
}

// Legacy function for backward compatibility
function showSettings() {
    showServerSettings();
}

// URL-based navigation functions (don't update URL to prevent loops)
function showDashboardFromURL() {
    clearLogsRefreshInterval();
    window.currentView = 'dashboard';
    $('#page-title').text('Dashboard');
    $('.nav-link').removeClass('active');
    $('[onclick="showDashboard()"]').addClass('active');
    loadDashboard();
}

function showTunnelsFromURL() {
    clearLogsRefreshInterval();
    window.currentView = 'tunnels';
    $('#page-title').text('Tunnels');
    $('.nav-link').removeClass('active');
    $('[onclick="showTunnels()"]').addClass('active');
    loadTunnelsView();
}

function showListenersFromURL() {
    clearLogsRefreshInterval();
    window.currentView = 'listeners';
    $('#page-title').text('Listeners');
    $('.nav-link').removeClass('active');
    $('[onclick="showListeners()"]').addClass('active');
    loadListenersView();
}

function showUsersFromURL() {
    clearLogsRefreshInterval();
    window.currentView = 'users';
    $('#page-title').text('Users');
    $('.nav-link').removeClass('active');
    $('[onclick="showUsers()"]').addClass('active');
    loadUsersView();
}

function showLogsFromURL() {
    // Don't clear interval for logs page - we want auto-refresh here
    window.currentView = 'logs';
    $('#page-title').text('Logs');
    $('.nav-link').removeClass('active');
    $('[onclick="showLogs()"]').addClass('active');
    loadLogsView();
}

function showUserSettingsFromURL() {
    clearLogsRefreshInterval();
    window.currentView = 'user-settings';
    $('#page-title').text('User Settings');
    $('.nav-link').removeClass('active');
    $('[onclick="showUserSettings()"]').addClass('active');
    loadUserSettingsView();
}

function showServerSettingsFromURL() {
    clearLogsRefreshInterval();
    window.currentView = 'server-settings';
    $('#page-title').text('Server Settings');
    $('.nav-link').removeClass('active');
    $('[onclick="showServerSettings()"]').addClass('active');
    loadServerSettingsView();
}

// Load different views
function loadTunnelsView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">'
        + '<h3 class="card-title">Active Tunnels</h3>'
        + '<div class="card-tools">'
        + '<button class="btn btn-secondary btn-sm" onclick="loadTunnelsData()">'
        + '<i class="fas fa-sync"></i> Refresh</button>'
        + '</div>'
        + '</div>' +
        '<div class="card-body">' +
        '<table class="table table-bordered table-striped" id="tunnels-table">' +
        '<thead><tr>' +
        '<th>ID</th><th>User</th><th>Local Port</th><th>Remote Port</th>' +
        '<th>Status</th><th>Created</th><th>Actions</th>' +
        '</tr></thead>' +
        '<tbody id="tunnels-tbody">' +
        '<tr><td colspan="7" class="text-center">Loading...</td></tr>' +
        '</tbody></table>' +
        '</div></div></div></div>' +
        '<div id="sso-banner-tunnels-row" class="row">' +
        '<div class="col-12">' +
        '<div id="sso-banner-tunnels"></div>' +
        '</div>' +
        '</div>';
    $('#main-content').html(content);
    loadTunnelsData();
    loadSSOBannerForTunnels();

    // Start auto-refresh for tunnels table
    startTunnelsAutoRefresh();
}

function startTunnelsAutoRefresh() {
    // Stop any existing auto-refresh
    if (typeof AutoRefresh !== 'undefined') {
        AutoRefresh.stopAll();

        // Start auto-refresh for tunnels table (every 10 seconds)
        AutoRefresh.start('tunnels-table', function() {
            loadTunnelsData();
        }, 10000);
    }
}

function loadUsersView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title">Users</h3>' +
        '<div class="card-tools">' +
        '<button id="add-user-btn" class="btn btn-primary btn-sm" onclick="return false;">' +
        '<i class="fas fa-plus"></i> Add User</button>' +
        '</div></div>' +
        '<div class="card-body">' +
        '<table class="table table-bordered table-striped" id="users-table">' +
        '<thead><tr><th>Username</th><th>Admin</th><th>Auth Source</th><th>Port Reservations</th><th>Created</th><th>Actions</th></tr></thead>' +
        '<tbody id="users-tbody">' +
        '<tr><td colspan="6" class="text-center">Loading...</td></tr>' +
        '</tbody></table>' +
        '</div></div></div></div>';
    $('#main-content').html(content);
    loadUsersData();
}

// Legacy loadListenersView function removed - now handled by listeners.js module

function startListenersAutoRefresh() {
    // Stop any existing auto-refresh
    if (typeof AutoRefresh !== 'undefined') {
        AutoRefresh.stopAll();

        // Start auto-refresh for listeners table (every 10 seconds)
        AutoRefresh.start('listeners-table', function() {
            loadListenersData();
        }, 10000);
    }
}

function loadLogsView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title"><i class="fas fa-file-alt"></i> Server Logs</h3>' +
        '<div class="card-tools">' +
        '<button class="btn btn-info btn-sm mr-2" onclick="showLogFiles()">' +
        '<i class="fas fa-folder"></i> Log Files</button>' +
        '<button class="btn btn-warning btn-sm mr-2" onclick="clearLogs()">' +
        '<i class="fas fa-trash"></i> Clear</button>' +
        '<button class="btn btn-secondary btn-sm" onclick="refreshLogs()">' +
        '<i class="fas fa-sync"></i> Refresh</button>' +
        '</div>' +
        '</div>' +
        '<div class="card-body">' +
        '<div class="row mb-3">' +
        '<div class="col-md-6">' +
        '<label for="logLimit">Show last:</label>' +
        '<select class="form-control form-control-sm" id="logLimit" onchange="loadLogsData()">' +
        '<option value="50">50 entries</option>' +
        '<option value="100" selected>100 entries</option>' +
        '<option value="200">200 entries</option>' +
        '<option value="500">500 entries</option>' +
        '</select>' +
        '</div>' +
        '<div class="col-md-6">' +
        '<label for="logLevel">Filter by level:</label>' +
        '<select class="form-control form-control-sm" id="logLevel" onchange="loadLogsData()">' +
        '<option value="">All levels</option>' +
        '<option value="error">Error</option>' +
        '<option value="warning">Warning</option>' +
        '<option value="info">Info</option>' +
        '<option value="debug">Debug</option>' +
        '</select>' +
        '</div>' +
        '</div>' +
        '<div id="logs-container" style="height: 500px; overflow-y: auto; background: #1e1e1e; color: #f8f9fa; padding: 15px; font-family: \'Courier New\', monospace; border-radius: 5px;">' +
        '<div class="text-center text-muted">Loading logs...</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';
    $('#main-content').html(content);
    loadLogsData();

    // Auto-refresh logs every 10 seconds (less aggressive)
    if (window.logsRefreshInterval) {
        clearInterval(window.logsRefreshInterval);
    }
    window.logsRefreshInterval = setInterval(function() {
        loadLogsDataSilently();
    }, 10000);
}

function loadSettingsView() {
    var content = '<div class="row">' +
        '<div class="col-md-6"><div class="card">' +
        '<div class="card-header"><h3 class="card-title">Server Configuration</h3></div>' +
        '<div class="card-body"><table class="table">' +
        '<tr><td><strong>Version</strong></td><td id="settings-version">Loading...</td></tr>' +
        '<tr><td><strong>Uptime</strong></td><td id="settings-uptime">Loading...</td></tr>' +
        '<tr><td><strong>Database</strong></td><td id="settings-database">Loading...</td></tr>' +
        '<tr><td><strong>Auth0</strong></td><td id="settings-auth0">Loading...</td></tr>' +
        '<tr><td><strong>Dashboard</strong></td><td id="settings-dashboard">Enabled</td></tr>' +
        '</table></div></div></div>' +
        '<div class="col-md-6"><div class="card">' +
        '<div class="card-header"><h3 class="card-title">Port Reservations</h3></div>' +
        '<div class="card-body">' +
        '<div class="form-group">' +
        '<label for="reservedPortsThreshold">Reserved Ports Range</label>' +
        '<div class="input-group">' +
        '<div class="input-group-prepend">' +
        '<span class="input-group-text">0 -</span>' +
        '</div>' +
        '<input type="number" class="form-control" id="reservedPortsThreshold" min="1" max="65535" placeholder="10000">' +
        '<div class="input-group-append">' +
        '<button class="btn btn-primary" onclick="updateReservedPortsThreshold()">Update</button>' +
        '</div>' +
        '</div>' +
        '<small class="form-text text-muted">Ports 0 to <span id="thresholdDisplay">10000</span> are reserved for admin assignment (max: 65535)</small>' +
        '</div>' +
        '</div></div></div>' +
        '<div class="col-md-6">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title">SSO Configuration</h3>' +
        '<div class="card-tools">' +
        '<button class="btn btn-primary btn-sm" onclick="showSSOConfigModal()">' +
        '<i class="fas fa-plus"></i> Add Provider</button>' +
        '<button class="btn btn-secondary btn-sm ml-2" onclick="loadSSOConfigs()">' +
        '<i class="fas fa-sync"></i> Refresh</button>' +
        '</div>' +
        '</div>' +
        '<div class="card-body">' +
        '<div id="sso-configs-container">' +
        '<div class="text-center text-muted">Loading SSO configurations...</div>' +
        '</div>' +
        '</div>' +
        '</div></div>' +
        '</div>'; // Close the row

    $('#main-content').html(content);
    loadSettingsSystemInfo();
    loadReservedPortsThreshold();
    loadSSOConfigs();
}

function loadUserSettingsView() {
    var content = '<div class="row">' +
        '<div class="col-md-6">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title"><i class="fas fa-user"></i> Profile Settings</h3>' +
        '</div>' +
        '<div class="card-body">' +
        '<form id="profileForm">' +
        '<div class="form-group">' +
        '<label for="displayName">Display Name</label>' +
        '<input type="text" class="form-control" id="displayName" placeholder="Enter display name">' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="email">Email Address</label>' +
        '<input type="email" class="form-control" id="email" placeholder="Enter email address">' +
        '<small class="form-text text-muted" id="emailHelp">Used for notifications and account recovery</small>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="currentPassword">Current Password</label>' +
        '<input type="password" class="form-control" id="currentPassword" placeholder="Enter current password">' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="newPassword">New Password</label>' +
        '<input type="password" class="form-control" id="newPassword" placeholder="Enter new password">' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="confirmPassword">Confirm New Password</label>' +
        '<input type="password" class="form-control" id="confirmPassword" placeholder="Confirm new password">' +
        '</div>' +
        '<button type="submit" class="btn btn-primary">Update Profile</button>' +
        '</form>' +
        '</div></div></div>' +
        '<div class="col-md-6">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title"><i class="fas fa-key"></i> API Tokens</h3>' +
        '<button class="btn btn-primary btn-sm float-right" onclick="showGenerateTokenModal()">' +
        '<i class="fas fa-plus"></i> Generate Token' +
        '</button>' +
        '</div>' +
        '<div class="card-body">' +
        '<div id="api-token-info-banner"></div>' +
        '<div id="tokens-list">' +
        '<p class="text-muted">Loading tokens...</p>' +
        '</div>' +
        '</div></div></div></div>' +
        '<div class="row">' +
        '<div class="col-md-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title"><i class="fas fa-network-wired"></i> Port Access</h3>' +
        '</div>' +
        '<div class="card-body" id="user-port-info">' +
        '<div class="text-center text-muted">Loading port information...</div>' +
        '</div></div></div></div>';

    $('#main-content').html(content);
    loadUserProfile();
    loadAPITokenBanner();
    loadUserTokens();
    loadUserPortInfo();

    // Handle profile form submission
    $('#profileForm').on('submit', function(e) {
        e.preventDefault();
        updateUserProfile();
    });
}

// loadServerSettingsView is defined in server-settings.js

function loadSettingsSystemInfo() {
    $.get('/api/system')
        .done(function(data) {
            $('#settings-uptime').text(formatUptime(data.uptime || 0));
            $('#settings-version').text(data.version || 'Unknown');
            $('#settings-database').text(data.database ? 'Enabled' : 'Disabled');
            $('#settings-auth0').text(data.auth0 ? 'Enabled' : 'Disabled');
        })
        .fail(function() {
            $('#settings-uptime').text('Error loading');
            $('#settings-version').text('Error loading');
            $('#settings-database').text('Error loading');
            $('#settings-auth0').text('Error loading');
        });
}

function loadReservedPortsThreshold() {
    $.get('/api/user/reserved-ports-threshold')
        .done(function(data) {
            var threshold = data.threshold || 10000;
            $('#reservedPortsThreshold').val(threshold);
            $('#thresholdDisplay').text(threshold);
        })
        .fail(function() {
            $('#reservedPortsThreshold').val(10000);
            $('#thresholdDisplay').text(10000);
        });
}

function updateReservedPortsThreshold() {
    var threshold = parseInt($('#reservedPortsThreshold').val());
    if (isNaN(threshold) || threshold < 1 || threshold > 65535) {
        alert('Please enter a valid port number between 1 and 65535');
        return;
    }

    $.ajax({
        url: '/api/settings/reserved-ports-threshold',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify({threshold: threshold})
    })
    .done(function() {
        $('#thresholdDisplay').text(threshold);
        alert('Reserved ports range updated successfully! Ports 0-' + threshold + ' are now reserved.');
    })
    .fail(function(xhr) {
        alert('Failed to update threshold: ' + (xhr.responseText || 'Unknown error'));
    });
}

function loadAdminPortReservations() {
    // Only load if user is admin
    if (!window.isAdmin) {
        return;
    }

    $.get('/api/port-reservations')
        .done(function(data) {
            var tbody = '';
            if (data && data.length > 0) {
                data.forEach(function(reservation) {
                    tbody += '<tr>' +
                        '<td>' + reservation.username + '</td>' +
                        '<td>' + reservation.start_port + '-' + reservation.end_port + '</td>' +
                        '<td>' + (reservation.description || 'No description') + '</td>' +
                        '<td>' + new Date(reservation.created_at).toLocaleString() + '</td>' +
                        '<td>' +
                        '<button class="btn btn-sm btn-outline-danger" onclick="deletePortReservation(\'' + reservation.id + '\')">' +
                        '<i class="fas fa-trash"></i> Remove</button>' +
                        '</td>' +
                        '</tr>';
                });
            } else {
                tbody = '<tr><td colspan="5" class="text-center text-muted">No port reservations found</td></tr>';
            }
            $('#port-reservations-tbody').html(tbody);
        })
        .fail(function() {
            $('#port-reservations-tbody').html('<tr><td colspan="5" class="text-center text-danger">Failed to load port reservations</td></tr>');
        });
}

function loadUserPortInfo() {
    // Load user's port reservations and general port info
    $.get('/api/user/port-reservations')
        .done(function(data) {
            var portInfoHtml = '';
            if (data && data.length > 0) {
                portInfoHtml = '<div class="alert alert-success">' +
                    '<h6><i class="fas fa-check-circle"></i> You have reserved ports assigned</h6>';
                data.forEach(function(reservation) {
                    portInfoHtml += '<div class="mb-2">' +
                        '<strong>Ports ' + reservation.start_port + '-' + reservation.end_port + '</strong>';
                    if (reservation.description) {
                        portInfoHtml += '<br><small class="text-muted">' + reservation.description + '</small>';
                    }
                    portInfoHtml += '</div>';
                });
                portInfoHtml += '<small>You can create tunnels and listeners on these reserved ports in addition to unreserved ports.</small>' +
                    '</div>';
            } else {
                // Show general port info for users without reservations
                $.get('/api/user/reserved-ports-threshold')
                    .done(function(thresholdData) {
                        var threshold = thresholdData.threshold || 10000;
                        portInfoHtml = '<div class="alert alert-info">' +
                            '<h6><i class="fas fa-info-circle"></i> Available Ports</h6>' +
                            '<div><strong>You can use ports ' + threshold + ' and above</strong></div>' +
                            '<div class="mt-2"><small class="text-muted">' +
                            'Ports 0-' + (threshold-1) + ' are reserved for admin assignment. ' +
                            'Contact your administrator if you need access to reserved ports.' +
                            '</small></div>' +
                            '</div>';
                        $('#user-port-info').html(portInfoHtml);
                    })
                    .fail(function() {
                        $('#user-port-info').html('<div class="alert alert-warning">Unable to load port information</div>');
                    });
                return; // Exit early for users without reservations
            }
            $('#user-port-info').html(portInfoHtml);
        })
        .fail(function() {
            $('#user-port-info').html('<div class="alert alert-warning">Unable to load port information</div>');
        });
}

function loadSSOBanner() {
    // First check if user is SSO user, then check dismissal
    $.get('/api/user/info')
        .done(function(data) {
            // Only check dismissal for SSO users
            if (data.sso_enabled || data.auth_method === 'auth0' || data.auth_method === 'sso') {
                // Check if user has dismissed the SSO banner
                $.get('/api/user/preferences/sso_banner_dismissed')
                    .done(function(prefData) {
                        if (prefData && prefData.preference_value === 'true') {
                            return; // Banner was dismissed, don't show it
                        }
                        showSSOBannerContent(data);
                    })
                    .fail(function() {
                        // Preference not found, show banner
                        showSSOBannerContent(data);
                    });
            }
            // For non-SSO users, do nothing (no banner needed)
        })
        .fail(function() {
            // Don't show error for SSO banner as it's optional
        });
}

function showSSOBannerContent(data) {
    // Show SSO banner content (data already contains user info)
    var bannerHtml = '<div class="alert alert-info alert-dismissible" id="sso-info-banner">' +
        '<button type="button" class="close" onclick="dismissSSOBanner()" aria-hidden="true">&times;</button>' +
        '<h5><i class="fas fa-key"></i> API Token Required for Client Connections</h5>' +
        '<p>Since you\'re using SSO authentication, you cannot use your SSO password for client connections. ' +
        'You must generate an API token and use it as the password when connecting with the chiSSL client.</p>' +
        '<div class="mt-2">' +
        '<button class="btn btn-sm btn-primary" onclick="showUserSettings()">' +
        '<i class="fas fa-plus"></i> Generate API Token</button>' +
        '<small class="ml-2 text-muted">Go to User Settings → API Tokens</small>' +
        '</div>' +
        '</div>';
    $('#sso-banner').html(bannerHtml);
}

function dismissSSOBanner() {
    // Store dismissal in database
    $.ajax({
        url: '/api/user/preferences/sso_banner_dismissed',
        method: 'PUT',
        contentType: 'application/json',
        data: JSON.stringify({value: 'true'})
    })
    .done(function() {
        // Hide the banner
        $('#sso-info-banner').fadeOut();
    })
    .fail(function() {
        console.error('Failed to save banner dismissal preference');
        // Still hide the banner for this session
        $('#sso-info-banner').fadeOut();
    });
}

function loadAPITokenBanner() {
    // Show informative banner about API tokens in User Settings
    $.get('/api/user/info')
        .done(function(data) {
            var bannerHtml = '<div class="alert alert-info mb-3">' +
                '<i class="fas fa-key mr-2"></i>';

            // Add specific note for SSO users
            if (data.sso_enabled || data.auth_method === 'auth0' || data.auth_method === 'sso') {
                bannerHtml += '<strong>SSO users:</strong> Use API tokens as passwords for client connections.';
            } else {
                bannerHtml += '<strong>API tokens</strong> provide secure authentication for client connections.';
            }

            bannerHtml += '</div>';
            $('#api-token-info-banner').html(bannerHtml);
        })
        .fail(function() {
            // Show generic banner if user info fails
            var bannerHtml = '<div class="alert alert-info mb-3">' +
                '<i class="fas fa-key mr-2"></i>' +
                '<strong>API tokens</strong> provide secure authentication for client connections.' +
                '</div>';
            $('#api-token-info-banner').html(bannerHtml);
        });
}

function loadSSOBannerForTunnels() {
    // Check if user is authenticated via SSO (Auth0/SCIM) - for tunnels page
    $.get('/api/user/info')
        .done(function(data) {
            // Check if user has SSO authentication method
            if (data.auth_method === 'auth0' || data.auth_method === 'sso') {
                var bannerHtml = '<div class="alert alert-info alert-dismissible">' +
                    '<button type="button" class="close" data-dismiss="alert" aria-hidden="true">&times;</button>' +
                    '<h6><i class="fas fa-info-circle"></i> Using SSO? Generate a Token for Tunnel Client</h6>' +
                    '<div>To create tunnels with the chissl client, you\'ll need an API token since you\'re using SSO authentication.</div>' +
                    '<div class="mt-2">' +
                    '<strong>Usage:</strong> <code>chissl client username:token server:port local:remote</code>' +
                    '<button class="btn btn-sm btn-primary ml-2" onclick="showUserSettings()">' +
                    '<i class="fas fa-key"></i> Get Token</button>' +
                    '</div>' +
                    '</div>';
                $('#sso-banner-tunnels').html(bannerHtml);
            }
        })
        .fail(function() {
            // Don't show error for SSO banner as it's optional
        });
}



// Data loading functions
function loadTunnelsData() {
    $.get('/api/tunnels')
        .done(function(data) {
            var tbody = '';
            if (data && data.length > 0) {
                data.forEach(function(tunnel) {
                    var statusBadge = tunnel.status === 'open' ? 'success' : (tunnel.status === 'closed' ? 'secondary' : 'danger');
                    tbody += '<tr>' +
                        '<td>' + tunnel.id.substring(0, 8) + '...' +
                        '<br><small><a href="#" onclick="showTunnelPayloads(\'' + tunnel.id + '\')"><i class="fas fa-eye"></i> Inspect Traffic</a></small></td>' +
                        '<td>' + tunnel.username + '</td>' +
                        '<td>' + tunnel.local_port + '</td>' +
                        '<td>' + tunnel.remote_port + '</td>' +
                        '<td><span class="badge badge-' + statusBadge + '">' + tunnel.status + '</span></td>' +
                        '<td>' + new Date(tunnel.created_at).toLocaleString() + '</td>' +
                        '<td>' +
                        '<button class="btn btn-sm btn-outline-danger" onclick="deleteTunnel(\'' + tunnel.id + '\')">' +
                        '<i class="fas fa-trash"></i> Close</button>' +
                        '</td>' +
                        '</tr>';
                });
            } else {
                tbody = '<tr><td colspan="7" class="text-center">No tunnels found</td></tr>';
            }
            $('#tunnels-tbody').html(tbody);
        })
        .fail(function() {
            $('#tunnels-tbody').html('<tr><td colspan="7" class="text-center text-danger">Failed to load tunnels</td></tr>');
        });
}

function loadUsersData() {
    // Only load if user is admin
    if (!window.isAdmin) {
        $('#users-tbody').html('<tr><td colspan="6" class="text-center text-warning">Admin access required</td></tr>');
        return;
    }

    // Load users, port reservations, and auth sources in parallel
    $.when(
        $.get('/users'),
        $.get('/api/port-reservations'),
        $.get('/api/sso/user-sources')
    ).done(function(usersResponse, reservationsResponse, authSourcesResponse) {
        var users = usersResponse[0];
        var reservations = reservationsResponse[0];
        var authSources = authSourcesResponse[0];

        // Create a map of username to reservations
        var userReservations = {};
        if (reservations && reservations.length > 0) {
            reservations.forEach(function(reservation) {
                if (!userReservations[reservation.username]) {
                    userReservations[reservation.username] = [];
                }
                userReservations[reservation.username].push(reservation);
            });
        }

        // Create a map of username to auth source
        var userAuthSources = {};
        if (authSources && authSources.length > 0) {
            authSources.forEach(function(source) {
                userAuthSources[source.username] = source;
            });
        }

        var tbody = '';
        if (users && users.length > 0) {
            users.forEach(function(user) {
                var username = user.username || user.name;
                var isAdmin = user.is_admin || user.IsAdmin;
                var adminBadge = isAdmin ? 'success' : 'secondary';
                var adminText = isAdmin ? 'Yes' : 'No';
                var createdAt = user.created_at ? new Date(user.created_at).toLocaleString() : 'N/A';

                // Build auth source display
                var authSource = userAuthSources[username];
                var authSourceHtml = '';
                if (authSource) {
                    var sourceText = authSource.auth_source.toUpperCase();
                    var sourceBadge = authSource.auth_source === 'local' ? 'secondary' : 'primary';
                    var sourceIcon = authSource.auth_source === 'local' ? 'fas fa-user' : 'fas fa-cloud';
                    authSourceHtml = '<span class="badge badge-' + sourceBadge + '">' +
                        '<i class="' + sourceIcon + '"></i> ' + sourceText + '</span>';
                } else {
                    authSourceHtml = '<span class="badge badge-secondary">' +
                        '<i class="fas fa-user"></i> LOCAL</span>';
                }

                // Build port reservations display
                var portReservationsHtml = '';
                var hasReservations = userReservations[username] && userReservations[username].length > 0;
                if (hasReservations) {
                    var ranges = userReservations[username].map(function(res) {
                        return res.start_port + '-' + res.end_port;
                    });
                    portReservationsHtml = '<span class="badge badge-info">' + ranges.join(', ') + '</span>';
                } else {
                    portReservationsHtml = '<span class="text-muted">None</span>';
                }

                // Build action buttons
                var actionButtons = '<button class="btn btn-sm btn-info mr-1" onclick="showAddPortReservationModal(\'' + username + '\')" title="Assign Ports">' +
                    '<i class="fas fa-network-wired"></i></button>';

                // Add remove ports button if user has reservations
                if (hasReservations) {
                    actionButtons += '<button class="btn btn-sm btn-outline-warning mr-1" onclick="removeUserPortReservations(\'' + username + '\')" title="Remove Port Assignments">' +
                        '<i class="fas fa-times"></i></button>';
                }

                actionButtons += '<button class="btn btn-sm btn-warning mr-1 edit-user-btn" data-username="' + username + '" title="Edit User">' +
                    '<i class="fas fa-edit"></i></button>' +
                    '<button class="btn btn-sm btn-danger delete-user-btn" data-username="' + username + '" title="Delete User">' +
                    '<i class="fas fa-trash"></i></button>';

                tbody += '<tr>' +
                    '<td>' + username + '</td>' +
                    '<td><span class="badge badge-' + adminBadge + '">' + adminText + '</span></td>' +
                    '<td>' + authSourceHtml + '</td>' +
                    '<td>' + portReservationsHtml + '</td>' +
                    '<td>' + createdAt + '</td>' +
                    '<td>' + actionButtons + '</td></tr>';
            });
        } else {
            tbody = '<tr><td colspan="6" class="text-center">No users found</td></tr>';
        }
        $('#users-tbody').html(tbody);
    }).fail(function() {
        $('#users-tbody').html('<tr><td colspan="6" class="text-center text-danger">Failed to load users</td></tr>');
    });
}

function loadLogsData() {
    $.get('/api/logs?limit=50')
        .done(function(data) {
            var logsHtml = '';
            if (data && data.length > 0) {
                data.forEach(function(log) {
                    var levelClass = log.level === 'error' ? 'text-danger' :
                                   log.level === 'warning' ? 'text-warning' : 'text-info';
                    logsHtml += '<div class="' + levelClass + '">[' + log.timestamp + '] [' +
                               log.level.toUpperCase() + '] ' + log.message + '</div>';
                });
            } else {
                logsHtml = '<div class="text-muted">No logs available</div>';
            }
            $('#logs-container').html(logsHtml);
        })
        .fail(function() {
            $('#logs-container').html('<div class="text-danger">Failed to load logs</div>');
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

function loadLogsDataSilently() {
    // Silent refresh that only updates if there are changes
    $.get('/api/logs?limit=50')
        .done(function(response) {
            if (response && response.logs && response.logs.length > 0) {
                // Check if we have new logs by comparing with current content
                var currentLogCount = $('.log-entry').length;
                if (response.logs.length !== currentLogCount) {
                    // Only update if log count changed
                    loadLogsData();
                }
            }
        })
        .fail(function() {
            // Silently fail - don't show errors for background refresh
        });
}

// Data loading functions for listeners
function loadListenersData() {
    $.get('/api/listeners')
        .done(function(data) {
            var tbody = '';
            if (data && data.length > 0) {
                data.forEach(function(listener) {
                    var statusBadge = listener.status === 'open' ? 'success' : (listener.status === 'closed' ? 'secondary' : 'danger');

                    // Create truncated target/response with info icon
                    var fullTargetOrResponse = listener.mode === 'proxy' ?
                        (listener.target_url || 'N/A') :
                        (listener.response || 'Default response');
                    var truncatedTargetOrResponse = fullTargetOrResponse.length > 30 ?
                        fullTargetOrResponse.substring(0, 30) + '...' : fullTargetOrResponse;
                    var targetOrResponseCell = truncatedTargetOrResponse;
                    if (fullTargetOrResponse.length > 30) {
                        targetOrResponseCell += ' <i class="fas fa-info-circle text-info ml-1" ' +
                            'style="cursor: pointer;" ' +
                            'onclick="showTargetResponseModal(\'' + listener.id + '\', \'' +
                            fullTargetOrResponse.replace(/'/g, '\\\'').replace(/"/g, '&quot;') + '\', \'' +
                            listener.mode + '\')" ' +
                            'title="Click to view full content"></i>';
                    }

                    var protocol = listener.use_tls ? 'https' : 'http';
                    var displayName = listener.name || ('Listener ' + listener.port);
                    var fullUrl = protocol + '://' + window.location.hostname + ':' + listener.port;

                    tbody += '<tr>' +
                        '<td>' + displayName + '</td>' +
                        '<td>' +
                        '<div class="input-group input-group-sm">' +
                        '<input type="text" class="form-control form-control-sm" value="' + fullUrl + '" readonly style="font-size: 11px;">' +
                        '<div class="input-group-append">' +
                        '<button class="btn btn-outline-secondary btn-sm" type="button" onclick="copyToClipboard(\'' + fullUrl + '\')" title="Copy URL">' +
                        '<i class="fas fa-copy"></i>' +
                        '</button>' +
                        '</div>' +
                        '</div>' +
                        '</td>' +
                        '<td>' + listener.username + '</td>' +
                        '<td>' + listener.port + '</td>' +
                        '<td><span class="badge badge-' + (listener.mode === 'proxy' ? 'info' : 'warning') + '">' + listener.mode + '</span></td>' +
                        '<td>' + targetOrResponseCell + '</td>' +
                        '<td><span class="badge badge-' + statusBadge + '">' + listener.status + '</span></td>' +
                        '<td>' + new Date(listener.created_at).toLocaleString() + '</td>' +
                        '<td>' +
                        '<button class="btn btn-sm btn-info" onclick="showTrafficPayloads(\'' + listener.id + '\', \'listener\')" title="Inspect Traffic">' +
                        '<i class="fas fa-eye"></i></button> ' +
                        '<button class="btn btn-sm btn-warning" onclick="editListener(\'' + listener.id + '\')" title="Edit">' +
                        '<i class="fas fa-edit"></i></button> ' +
                        '<button class="btn btn-sm btn-danger" onclick="deleteListener(\'' + listener.id + '\')" title="Delete">' +
                        '<i class="fas fa-trash"></i></button>' +
                        '</td>' +
                        '</tr>';
                });
            } else {
                tbody = '<tr><td colspan="9" class="text-center text-muted">No listeners found</td></tr>';
            }
            $('#listeners-tbody').html(tbody);
        })
        .fail(function() {
            $('#listeners-tbody').html('<tr><td colspan="9" class="text-center text-danger">Failed to load listeners</td></tr>');
        });
}

// User profile management functions
function loadUserProfile() {
    if (window.currentUser) {
        $('#displayName').val(window.currentUser.display_name || '');
        $('#email').val(window.currentUser.email || '');

        // Disable profile editing for CLI admin users
        if (!window.currentUser.can_edit_profile) {
            $('#profileForm input, #profileForm button').prop('disabled', true);
            $('#profileForm').prepend('<div class="alert alert-info">' +
                '<i class="fas fa-info-circle"></i> ' +
                'CLI admin users cannot edit their profile. Profile settings are managed via command line configuration.' +
                '</div>');
            return;
        }

        // Disable email field if using SSO
        if (window.currentUser.sso_enabled) {
            $('#email').prop('disabled', true);
            $('#emailHelp').text('Email cannot be changed when using SSO');

            // Disable password fields for SSO users
            $('#currentPassword, #newPassword, #confirmPassword').prop('disabled', true);
            $('#currentPassword, #newPassword, #confirmPassword').addClass('bg-light');

            // Add info message about SSO password management
            $('#profileForm').append('<div class="alert alert-info mt-3">' +
                '<i class="fas fa-info-circle"></i> ' +
                'Password management is handled by your SSO provider. Use API tokens for programmatic access.' +
                '</div>');
        }
    }
}

function loadUserTokens() {
    $.get('/api/user/tokens')
    .done(function(data) {
        var tokensList = '';
        if (data && data.length > 0) {
            data.forEach(function(token) {
                var createdDate = new Date(token.created_at).toLocaleDateString();
                var lastUsed = token.last_used ? new Date(token.last_used).toLocaleDateString() : 'Never';
                tokensList += '<div class="border rounded p-3 mb-2">' +
                    '<div class="d-flex justify-content-between align-items-center">' +
                    '<div>' +
                    '<strong>' + (token.name || 'Unnamed Token') + '</strong><br>' +
                    '<small class="text-muted">Created: ' + createdDate + ' | Last used: ' + lastUsed + '</small>' +
                    '</div>' +
                    '<button class="btn btn-sm btn-danger" onclick="revokeToken(\'' + token.id + '\')">' +
                    '<i class="fas fa-trash"></i> Revoke' +
                    '</button>' +
                    '</div>' +
                    '</div>';
            });
        } else {
            tokensList = '<p class="text-muted">No API tokens found. Generate one to connect tunnels without using your password.</p>';
        }
        $('#tokens-list').html(tokensList);
    })
    .fail(function() {
        $('#tokens-list').html('<p class="text-danger">Failed to load tokens</p>');
    });
}

function showGenerateTokenModal() {
    var modalHtml = '<div class="modal fade" id="generateTokenModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog" role="document">' +
        '<div class="modal-content">' +
        '<div class="modal-header">' +
        '<h5 class="modal-title">Generate API Token</h5>' +
        '<button type="button" class="close" data-dismiss="modal">&times;</button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<form id="generateTokenForm">' +
        '<div class="form-group">' +
        '<label for="tokenName">Token Name</label>' +
        '<input type="text" class="form-control" id="tokenName" placeholder="e.g., My Laptop, Production Server" required>' +
        '<small class="form-text text-muted">Choose a descriptive name to identify this token</small>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="tokenExpiry">Expiry (optional)</label>' +
        '<select class="form-control" id="tokenExpiry">' +
        '<option value="">Never expires</option>' +
        '<option value="30">30 days</option>' +
        '<option value="90">90 days</option>' +
        '<option value="365">1 year</option>' +
        '</select>' +
        '</div>' +
        '</form>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        '<button type="button" class="btn btn-primary" onclick="generateToken()">Generate Token</button>' +
        '</div>' +
        '</div></div></div>';

    $('body').append(modalHtml);
    $('#generateTokenModal').modal('show');
    $('#generateTokenModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });
}

function generateToken() {
    var tokenData = {
        name: $('#tokenName').val().trim(),
        expiry_days: $('#tokenExpiry').val() ? parseInt($('#tokenExpiry').val()) : null
    };

    $.ajax({
        url: '/api/user/tokens',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify(tokenData)
    })
    .done(function(data) {
        $('#generateTokenModal').modal('hide');

        // Show the generated token
        var tokenModalHtml = '<div class="modal fade" id="showTokenModal" tabindex="-1" role="dialog">' +
            '<div class="modal-dialog" role="document">' +
            '<div class="modal-content">' +
            '<div class="modal-header bg-success text-white">' +
            '<h5 class="modal-title">Token Generated Successfully</h5>' +
            '<button type="button" class="close text-white" data-dismiss="modal">&times;</button>' +
            '</div>' +
            '<div class="modal-body">' +
            '<div class="alert alert-warning">' +
            '<i class="fas fa-exclamation-triangle"></i> ' +
            '<strong>Important:</strong> This token will only be shown once. Copy it now and store it securely.' +
            '</div>' +
            '<div class="form-group">' +
            '<label>Your API Token:</label>' +
            '<div class="input-group">' +
            '<input type="text" class="form-control" value="' + data.token + '" readonly>' +
            '<div class="input-group-append">' +
            '<button class="btn btn-outline-secondary" onclick="copyToClipboard(\'' + data.token + '\')">' +
            '<i class="fas fa-copy"></i> Copy' +
            '</button>' +
            '</div>' +
            '</div>' +
            '</div>' +
            '<p class="text-muted">Use this token as the password when connecting tunnels.</p>' +
            '</div>' +
            '<div class="modal-footer">' +
            '<button type="button" class="btn btn-primary" data-dismiss="modal">I\'ve Copied It</button>' +
            '</div>' +
            '</div></div></div>';

        $('body').append(tokenModalHtml);
        $('#showTokenModal').modal('show');
        $('#showTokenModal').on('hidden.bs.modal', function() {
            $(this).remove();
            loadUserTokens(); // Refresh the tokens list
        });
    })
    .fail(function() {
        alert('Failed to generate token');
    });
}

function revokeToken(tokenId) {
    if (confirm('Are you sure you want to revoke this token? This action cannot be undone.')) {
        $.ajax({
            url: '/api/user/tokens/' + tokenId,
            method: 'DELETE'
        })
        .done(function() {
            loadUserTokens(); // Refresh the tokens list
        })
        .fail(function() {
            alert('Failed to revoke token');
        });
    }
}

function updateUserProfile() {
    var profileData = {
        display_name: $('#displayName').val().trim(),
        email: $('#email').val().trim()
    };

    // Add password fields if provided (skip for SSO users)
    if (!window.currentUser.sso_enabled) {
        var currentPassword = $('#currentPassword').val();
        var newPassword = $('#newPassword').val();
        var confirmPassword = $('#confirmPassword').val();

        if (newPassword) {
            if (!currentPassword) {
                alert('Current password is required to change password');
                return;
            }
            if (newPassword !== confirmPassword) {
                alert('New passwords do not match');
                return;
            }
            if (newPassword.length < 6) {
                alert('New password must be at least 6 characters long');
                return;
            }
            profileData.current_password = currentPassword;
            profileData.new_password = newPassword;
        }
    }

    $.ajax({
        url: '/api/user/profile',
        method: 'PUT',
        contentType: 'application/json',
        data: JSON.stringify(profileData)
    })
    .done(function() {
        alert('Profile updated successfully!');
        // Clear password fields
        $('#currentPassword').val('');
        $('#newPassword').val('');
        $('#confirmPassword').val('');
        // Reload user info to update welcome message
        loadUserInfo();
    })
    .fail(function(xhr) {
        var errorMsg = 'Failed to update profile';
        if (xhr.responseText) {
            errorMsg += ': ' + xhr.responseText;
        }
        alert(errorMsg);
    });
}

// Copy to clipboard function
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(function() {
        // Show a brief success message
        var btn = event.target.closest('button');
        var originalHtml = btn.innerHTML;
        btn.innerHTML = '<i class="fas fa-check"></i>';
        btn.classList.add('btn-success');
        btn.classList.remove('btn-outline-secondary');
        setTimeout(function() {
            btn.innerHTML = originalHtml;
            btn.classList.remove('btn-success');
            btn.classList.add('btn-outline-secondary');
        }, 1000);
    }).catch(function(err) {
        alert('Failed to copy: ' + err);
    });
}

// Listener management functions
function showAddListenerModal() {
    var modalHtml = '<div class="modal fade" id="addListenerModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog" role="document"><div class="modal-content">' +
        '<div class="modal-header">' +
        '<h4 class="modal-title">Add New Listener</h4>' +
        '<button type="button" class="close" data-dismiss="modal">&times;</button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<form id="addListenerForm">' +
        '<div class="form-group">' +
        '<label for="newName">Name (optional)</label>' +
        '<input type="text" class="form-control" id="newName" placeholder="e.g., API Proxy, Test Server">' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="newPort">Port</label>' +
        '<input type="number" class="form-control" id="newPort" min="1" max="65535" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="newMode">Mode</label>' +
        '<select class="form-control" id="newMode" onchange="toggleModeFields()">' +
        '<option value="sink">Sink (Log only)</option>' +
        '<option value="proxy">Proxy (Forward to target)</option>' +
        '</select>' +
        '</div>' +
        '<div class="form-group" id="targetUrlGroup" style="display:none;">' +
        '<label for="newTargetUrl">Target URL</label>' +
        '<input type="url" class="form-control" id="newTargetUrl" placeholder="https://example.com">' +
        '</div>' +
        '<div class="form-group" id="responseGroup">' +
        '<label for="newResponse">Response (JSON)</label>' +
        '<textarea class="form-control" id="newResponse" rows="3" placeholder=\'{"status": "ok", "message": "Request logged"}\'></textarea>' +
        '</div>' +
        '<div class="form-group">' +
        '<div class="form-check">' +
        '<input type="checkbox" class="form-check-input" id="newUseTLS" checked>' +
        '<label class="form-check-label" for="newUseTLS">Use HTTPS/TLS</label>' +
        '</div>' +
        '</div>' +
        '</form>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        '<button type="button" class="btn btn-primary" onclick="createListener()">Create Listener</button>' +
        '</div>' +
        '</div></div></div>';

    $('body').append(modalHtml);
    $('#addListenerModal').modal('show');

    $('#addListenerModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });
}

function toggleModeFields() {
    var mode = $('#newMode').val();
    if (mode === 'proxy') {
        $('#targetUrlGroup').show();
        $('#responseGroup').hide();
        $('#newTargetUrl').prop('required', true);
        $('#newResponse').prop('required', false);
    } else {
        $('#targetUrlGroup').hide();
        $('#responseGroup').show();
        $('#newTargetUrl').prop('required', false);
        $('#newResponse').prop('required', false);
    }
}

function createListener() {
    var listenerData = {
        name: $('#newName').val().trim(),
        port: parseInt($('#newPort').val()),
        mode: $('#newMode').val(),
        target_url: $('#newTargetUrl').val(),
        response: $('#newResponse').val(),
        use_tls: $('#newUseTLS').is(':checked')
    };

    $.ajax({
        url: '/api/listeners',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify(listenerData)
    })
    .done(function(data) {
        $('#addListenerModal').modal('hide');
        loadListenersData();
        alert('Listener created successfully');
    })
    .fail(function(xhr) {
        alert('Failed to create listener: ' + (xhr.responseText || 'Unknown error'));
    });
}

function editListener(listenerID) {
    // Get listener details first
    $.get('/api/listener/' + listenerID)
        .done(function(listener) {
            var modalHtml = '<div class="modal fade" id="editListenerModal" tabindex="-1" role="dialog">' +
                '<div class="modal-dialog" role="document"><div class="modal-content">' +
                '<div class="modal-header">' +
                '<h4 class="modal-title">Edit Listener: ' + listener.id.substring(0, 8) + '...</h4>' +
                '<button type="button" class="close" data-dismiss="modal">&times;</button>' +
                '</div>' +
                '<div class="modal-body">' +
                '<form id="editListenerForm">' +
                '<div class="form-group">' +
                '<label for="editName">Name</label>' +
                '<input type="text" class="form-control" id="editName" value="' + (listener.name || '') + '">' +
                '</div>' +
                '<div class="form-group">' +
                '<label>Port</label>' +
                '<input type="text" class="form-control" value="' + listener.port + '" readonly>' +
                '</div>' +
                '<div class="form-group">' +
                '<label>Mode</label>' +
                '<input type="text" class="form-control" value="' + listener.mode + '" readonly>' +
                '</div>' +
                (listener.mode === 'proxy' ?
                    '<div class="form-group">' +
                    '<label for="editTargetUrl">Target URL</label>' +
                    '<input type="url" class="form-control" id="editTargetUrl" value="' + (listener.target_url || '') + '">' +
                    '</div>' :
                    '<div class="form-group">' +
                    '<label for="editResponse">Response (JSON)</label>' +
                    '<textarea class="form-control" id="editResponse" rows="3">' + (listener.response || '') + '</textarea>' +
                    '</div>'
                ) +
                '<div class="form-group">' +
                '<div class="form-check">' +
                '<input type="checkbox" class="form-check-input" id="editUseTLS" ' + (listener.use_tls ? 'checked' : '') + '>' +
                '<label class="form-check-label" for="editUseTLS">Use HTTPS/TLS</label>' +
                '</div>' +
                '</div>' +
                '</form>' +
                '</div>' +
                '<div class="modal-footer">' +
                '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
                '<button type="button" class="btn btn-primary" onclick="updateListener(\'' + listener.id + '\')">Update Listener</button>' +
                '</div>' +
                '</div></div></div>';

            $('body').append(modalHtml);
            $('#editListenerModal').modal('show');

            $('#editListenerModal').on('hidden.bs.modal', function() {
                $(this).remove();
            });
        })
        .fail(function() {
            alert('Failed to load listener details');
        });
}

function updateListener(listenerID) {
    var updateData = {
        name: $('#editName').val().trim(),
        target_url: $('#editTargetUrl').val(),
        response: $('#editResponse').val(),
        use_tls: $('#editUseTLS').is(':checked')
    };

    $.ajax({
        url: '/api/listener/' + listenerID,
        method: 'PUT',
        contentType: 'application/json',
        data: JSON.stringify(updateData)
    })
    .done(function(data) {
        $('#editListenerModal').modal('hide');
        loadListenersData();
        alert('Listener updated successfully');
    })
    .fail(function(xhr) {
        alert('Failed to update listener: ' + (xhr.responseText || 'Unknown error'));
    });
}

function deleteListener(listenerID) {
    if (confirm('Are you sure you want to delete this listener? This will stop the listener and remove all associated data.')) {
        $.ajax({
            url: '/api/listener/' + listenerID,
            method: 'DELETE'
        })
        .done(function() {
            loadListenersData();
            alert('Listener deleted successfully');
        })
        .fail(function(xhr) {
            alert('Failed to delete listener: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

function loadLogsData() {
    var limit = $('#logLimit').val() || 100;
    var level = $('#logLevel').val() || '';

    var url = '/api/logs?limit=' + limit;
    if (level) {
        url += '&level=' + level;
    }

    $.get(url)
        .done(function(response) {
            var logsHtml = '';
            if (response && response.logs && response.logs.length > 0) {
                // Reverse logs to show newest first
                var reversedLogs = response.logs.slice().reverse();
                reversedLogs.forEach(function(log) {
                    var levelClass = '';
                    var levelIcon = '';
                    switch(log.level.toLowerCase()) {
                        case 'error':
                            levelClass = 'text-danger';
                            levelIcon = 'fas fa-times-circle';
                            break;
                        case 'warning':
                        case 'warn':
                            levelClass = 'text-warning';
                            levelIcon = 'fas fa-exclamation-triangle';
                            break;
                        case 'info':
                            levelClass = 'text-info';
                            levelIcon = 'fas fa-info-circle';
                            break;
                        case 'debug':
                            levelClass = 'text-muted';
                            levelIcon = 'fas fa-bug';
                            break;
                        default:
                            levelClass = 'text-light';
                            levelIcon = 'fas fa-circle';
                    }

                    var timestamp = new Date(log.timestamp).toLocaleString();
                    var source = log.source ? ' [' + log.source + ']' : '';

                    logsHtml += '<div class="log-entry mb-1" style="border-left: 3px solid ' +
                        (levelClass.includes('danger') ? '#dc3545' :
                         levelClass.includes('warning') ? '#ffc107' :
                         levelClass.includes('info') ? '#17a2b8' : '#6c757d') + '; padding-left: 10px;">' +
                        '<span class="' + levelClass + '"><i class="' + levelIcon + '"></i> [' + log.level.toUpperCase() + ']</span> ' +
                        '<span style="color: #adb5bd;">' + timestamp + source + '</span><br>' +
                        '<span style="color: #f8f9fa;">' + escapeHtml(log.message) + '</span>' +
                        '</div>';
                });

                // Add summary info
                var summary = '<div class="text-muted mb-3 pb-2" style="border-bottom: 1px solid #495057;">' +
                    'Showing ' + response.logs.length + ' log entries (newest first)';
                if (response.has_more) {
                    summary += ' - more available';
                }
                summary += '</div>';
                logsHtml = summary + logsHtml;
            } else {
                logsHtml = '<div class="text-muted text-center">No logs available</div>';
            }
            $('#logs-container').html(logsHtml);

            // Keep scroll at top to show newest logs
            var container = document.getElementById('logs-container');
            container.scrollTop = 0;
        })
        .fail(function() {
            $('#logs-container').html('<div class="text-danger text-center">Failed to load logs</div>');
        });
}

function refreshLogs() {
    loadLogsData();
}

function clearLogs() {
    if (confirm('Are you sure you want to clear recent logs from memory? This will not delete log files.')) {
        $.ajax({
            url: '/api/logs/clear',
            method: 'POST'
        })
        .done(function() {
            alert('Recent logs cleared successfully');
            loadLogsData();
        })
        .fail(function(xhr) {
            alert('Failed to clear logs: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

function showLogFiles() {
    $.get('/api/logs/files')
        .done(function(response) {
            var modalHtml = '<div class="modal fade" id="logFilesModal" tabindex="-1" role="dialog">' +
                '<div class="modal-dialog modal-lg" role="document">' +
                '<div class="modal-content">' +
                '<div class="modal-header">' +
                '<h5 class="modal-title">Log Files</h5>' +
                '<button type="button" class="close" data-dismiss="modal">' +
                '<span>&times;</span>' +
                '</button>' +
                '</div>' +
                '<div class="modal-body">' +
                '<table class="table table-striped">' +
                '<thead><tr><th>File</th><th>Size</th><th>Modified</th><th>Actions</th></tr></thead>' +
                '<tbody>';

            if (response.files && response.files.length > 0) {
                response.files.forEach(function(file) {
                    modalHtml += '<tr>' +
                        '<td>' + file.name + '</td>' +
                        '<td>' + formatBytes(file.size) + '</td>' +
                        '<td>' + file.modified_time + '</td>' +
                        '<td>' +
                        '<button class="btn btn-sm btn-info mr-1" onclick="viewLogFile(\'' + file.name + '\')">' +
                        '<i class="fas fa-eye"></i> View</button>' +
                        '<button class="btn btn-sm btn-success" onclick="downloadLogFile(\'' + file.name + '\')">' +
                        '<i class="fas fa-download"></i> Download</button>' +
                        '</td>' +
                        '</tr>';
                });
            } else {
                modalHtml += '<tr><td colspan="4" class="text-center text-muted">No log files found</td></tr>';
            }

            modalHtml += '</tbody></table>' +
                '</div>' +
                '<div class="modal-footer">' +
                '<button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>' +
                '</div>' +
                '</div>' +
                '</div>' +
                '</div>';

            $('body').append(modalHtml);
            $('#logFilesModal').modal('show');

            // Remove modal from DOM when closed
            $('#logFilesModal').on('hidden.bs.modal', function() {
                $(this).remove();
            });
        })
        .fail(function() {
            alert('Failed to load log files');
        });
}

function viewLogFile(filename) {
    window.open('/api/logs/files/' + filename + '?lines=1000', '_blank');
}

function downloadLogFile(filename) {
    window.open('/api/logs/files/' + filename + '/download', '_blank');
}

function escapeHtml(text) {
    var map = {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#039;'
    };
    return text.replace(/[&<>"']/g, function(m) { return map[m]; });
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    var k = 1024;
    var sizes = ['Bytes', 'KB', 'MB', 'GB'];
    var i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// SSO Configuration Functions
function loadSSOConfigs() {
    $.get('/api/sso/configs')
        .done(function(configs) {
            var html = '';
            if (configs && configs.length > 0) {
                configs.forEach(function(config) {
                    var statusBadge = config.enabled ? 'success' : 'secondary';
                    var statusText = config.enabled ? 'Enabled' : 'Disabled';
                    var providerIcon = getProviderIcon(config.provider);
                    var providerName = config.provider.charAt(0).toUpperCase() + config.provider.slice(1);

                    html += '<div class="border rounded p-3 mb-3">' +
                        '<div class="d-flex justify-content-between align-items-center">' +
                        '<div>' +
                        '<h6 class="mb-1">' +
                        '<i class="' + providerIcon + '"></i> ' + providerName + ' SSO' +
                        '</h6>' +
                        '<span class="badge badge-' + statusBadge + '">' + statusText + '</span>' +
                        '</div>' +
                        '<div class="btn-group btn-group-sm">';

                    if (config.enabled && config.test_url) {
                        html += '<button class="btn btn-info" onclick="testSSOConfig(\'' + config.provider + '\')">' +
                            '<i class="fas fa-vial"></i> Test</button>';
                    }

                    html += '<button class="btn btn-warning" onclick="editSSOConfig(\'' + config.provider + '\')">' +
                        '<i class="fas fa-edit"></i> Edit</button>' +
                        '<button class="btn btn-danger" onclick="deleteSSOConfig(\'' + config.provider + '\')">' +
                        '<i class="fas fa-trash"></i> Delete</button>' +
                        '</div>' +
                        '</div>';

                    // Show basic config info
                    if (config.config) {
                        html += '<div class="mt-2 text-muted small">';
                        if (config.provider === 'scim' || config.provider === 'okta') {
                            html += 'Client ID: ' + (config.config.client_id || 'Not configured');
                        } else if (config.provider === 'auth0') {
                            html += 'Domain: ' + (config.config.domain || 'Not configured');
                        }
                        html += '</div>';
                    }

                    html += '</div>';
                });
            } else {
                html = '<div class="text-center text-muted py-4">' +
                    '<i class="fas fa-shield-alt fa-3x mb-3"></i>' +
                    '<p>No SSO providers configured</p>' +
                    '<button class="btn btn-primary" onclick="showSSOConfigModal()">' +
                    '<i class="fas fa-plus"></i> Add Your First Provider</button>' +
                    '</div>';
            }
            $('#sso-configs-container').html(html);
        })
        .fail(function() {
            $('#sso-configs-container').html('<div class="text-danger text-center">Failed to load SSO configurations</div>');
        });
}

function getProviderIcon(provider) {
    switch(provider) {
        case 'okta': return 'fab fa-okta';
        case 'auth0': return 'fas fa-shield-alt';
        case 'scim': return 'fas fa-users-cog';
        case 'azure': return 'fab fa-microsoft';
        default: return 'fas fa-cloud';
    }
}

function showSSOConfigModal() {
    var modalHtml = '<div class="modal fade" id="ssoConfigModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog modal-lg" role="document">' +
        '<div class="modal-content">' +
        '<div class="modal-header">' +
        '<h5 class="modal-title">Configure SSO Provider</h5>' +
        '<button type="button" class="close" data-dismiss="modal">' +
        '<span>&times;</span>' +
        '</button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<form id="ssoConfigForm">' +
        '<div class="form-group">' +
        '<label for="ssoProvider">Provider</label>' +
        '<select class="form-control" id="ssoProvider" onchange="updateSSOConfigForm()">' +
        '<option value="">Select a provider...</option>' +
        '<option value="okta">Okta</option>' +
        '<option value="scim">Generic SCIM</option>' +
        '<option value="auth0">Auth0</option>' +
        '</select>' +
        '</div>' +
        '<div id="ssoConfigFields"></div>' +
        '<div class="form-group">' +
        '<div class="form-check">' +
        '<input type="checkbox" class="form-check-input" id="ssoEnabled">' +
        '<label class="form-check-label" for="ssoEnabled">Enable this SSO provider</label>' +
        '</div>' +
        '</div>' +
        '</form>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        '<button type="button" class="btn btn-primary" onclick="saveSSOConfig()">Save Configuration</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('body').append(modalHtml);
    $('#ssoConfigModal').modal('show');

    // Remove modal from DOM when closed
    $('#ssoConfigModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });
}

function updateSSOConfigForm() {
    var provider = $('#ssoProvider').val();
    var fieldsHtml = '';

    if (provider === 'okta' || provider === 'scim') {
        fieldsHtml = '<div class="alert alert-info">' +
            '<strong>Step-by-step Setup:</strong><br>' +
            '1. Create an OAuth2 application in your ' + provider.charAt(0).toUpperCase() + provider.slice(1) + ' admin console<br>' +
            '2. Set the redirect URI to: <code>' + window.location.origin + '/auth/scim/callback</code><br>' +
            '3. Copy the configuration details below' +
            '</div>' +
            '<div class="form-group">' +
            '<label for="clientId">Client ID *</label>' +
            '<input type="text" class="form-control" id="clientId" placeholder="Your OAuth2 Client ID" required>' +
            '</div>' +
            '<div class="form-group">' +
            '<label for="clientSecret">Client Secret *</label>' +
            '<input type="password" class="form-control" id="clientSecret" placeholder="Your OAuth2 Client Secret" required>' +
            '</div>' +
            '<div class="form-group">' +
            '<label for="authUrl">Authorization URL *</label>' +
            '<input type="url" class="form-control" id="authUrl" placeholder="https://your-domain.okta.com/oauth2/v1/authorize" required>' +
            '</div>' +
            '<div class="form-group">' +
            '<label for="tokenUrl">Token URL *</label>' +
            '<input type="url" class="form-control" id="tokenUrl" placeholder="https://your-domain.okta.com/oauth2/v1/token" required>' +
            '</div>' +
            '<div class="form-group">' +
            '<label for="userInfoUrl">User Info URL *</label>' +
            '<input type="url" class="form-control" id="userInfoUrl" placeholder="https://your-domain.okta.com/oauth2/v1/userinfo" required>' +
            '</div>' +
            '<div class="form-group">' +
            '<label for="scopes">Scopes</label>' +
            '<input type="text" class="form-control" id="scopes" value="openid profile email" placeholder="openid profile email">' +
            '<small class="form-text text-muted">Space-separated list of OAuth2 scopes</small>' +
            '</div>';

        if (provider === 'okta') {
            fieldsHtml += '<div class="form-group">' +
                '<label for="tenantId">Tenant ID (Optional)</label>' +
                '<input type="text" class="form-control" id="tenantId" placeholder="Your Okta tenant ID">' +
                '</div>';
        }
    } else if (provider === 'auth0') {
        fieldsHtml = '<div class="alert alert-info">' +
            '<strong>Auth0 Setup:</strong><br>' +
            '1. Create an application in your Auth0 dashboard<br>' +
            '2. Set the callback URL to: <code>' + window.location.origin + '/auth/auth0/callback</code><br>' +
            '3. Copy your application details below' +
            '</div>' +
            '<div class="form-group">' +
            '<label for="auth0Domain">Domain *</label>' +
            '<input type="text" class="form-control" id="auth0Domain" placeholder="your-tenant.auth0.com" required>' +
            '</div>' +
            '<div class="form-group">' +
            '<label for="auth0ClientId">Client ID *</label>' +
            '<input type="text" class="form-control" id="auth0ClientId" placeholder="Your Auth0 Client ID" required>' +
            '</div>' +
            '<div class="form-group">' +
            '<label for="auth0ClientSecret">Client Secret *</label>' +
            '<input type="password" class="form-control" id="auth0ClientSecret" placeholder="Your Auth0 Client Secret" required>' +
            '</div>';
    }

    $('#ssoConfigFields').html(fieldsHtml);
}

function saveSSOConfig() {
    var provider = $('#ssoProvider').val();
    if (!provider) {
        alert('Please select a provider');
        return;
    }

    var enabled = $('#ssoEnabled').is(':checked');
    var config = {};

    if (provider === 'okta' || provider === 'scim') {
        var clientId = $('#clientId').val();
        var clientSecret = $('#clientSecret').val();
        var authUrl = $('#authUrl').val();
        var tokenUrl = $('#tokenUrl').val();
        var userInfoUrl = $('#userInfoUrl').val();

        if (!clientId || !clientSecret || !authUrl || !tokenUrl || !userInfoUrl) {
            alert('Please fill in all required fields');
            return;
        }

        config = {
            client_id: clientId,
            client_secret: clientSecret,
            auth_url: authUrl,
            token_url: tokenUrl,
            user_info_url: userInfoUrl,
            redirect_url: window.location.origin + '/auth/scim/callback',
            scopes: $('#scopes').val().split(' ').filter(s => s.length > 0)
        };

        if (provider === 'okta' && $('#tenantId').val()) {
            config.tenant_id = $('#tenantId').val();
        }
    } else if (provider === 'auth0') {
        var domain = $('#auth0Domain').val();
        var clientId = $('#auth0ClientId').val();
        var clientSecret = $('#auth0ClientSecret').val();

        if (!domain || !clientId || !clientSecret) {
            alert('Please fill in all required fields');
            return;
        }

        config = {
            domain: domain,
            client_id: clientId,
            client_secret: clientSecret,
            redirect_url: window.location.origin + '/auth/auth0/callback',
            scopes: ['openid', 'profile', 'email']
        };
    }

    var requestData = {
        provider: provider,
        enabled: enabled,
        config: config
    };

    $.ajax({
        url: '/api/sso/configs',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify(requestData)
    })
    .done(function() {
        $('#ssoConfigModal').modal('hide');
        alert('SSO configuration saved successfully!');
        loadSSOConfigs();
    })
    .fail(function(xhr) {
        alert('Failed to save SSO configuration: ' + (xhr.responseText || 'Unknown error'));
    });
}

function editSSOConfig(provider) {
    $.get('/api/sso/configs/' + provider)
        .done(function(config) {
            showSSOConfigModal();

            // Wait for modal to be shown, then populate fields
            $('#ssoConfigModal').on('shown.bs.modal', function() {
                $('#ssoProvider').val(config.provider).trigger('change');
                $('#ssoEnabled').prop('checked', config.enabled);

                // Populate provider-specific fields
                setTimeout(function() {
                    if (config.provider === 'okta' || config.provider === 'scim') {
                        $('#clientId').val(config.config.client_id || '');
                        $('#clientSecret').val(config.config.client_secret || '');
                        $('#authUrl').val(config.config.auth_url || '');
                        $('#tokenUrl').val(config.config.token_url || '');
                        $('#userInfoUrl').val(config.config.user_info_url || '');
                        $('#scopes').val((config.config.scopes || []).join(' '));
                        if (config.provider === 'okta') {
                            $('#tenantId').val(config.config.tenant_id || '');
                        }
                    } else if (config.provider === 'auth0') {
                        $('#auth0Domain').val(config.config.domain || '');
                        $('#auth0ClientId').val(config.config.client_id || '');
                        $('#auth0ClientSecret').val(config.config.client_secret || '');
                    }
                }, 100);
            });
        })
        .fail(function() {
            alert('Failed to load SSO configuration');
        });
}

function deleteSSOConfig(provider) {
    if (confirm('Are you sure you want to delete the ' + provider.toUpperCase() + ' SSO configuration?')) {
        $.ajax({
            url: '/api/sso/configs/' + provider,
            method: 'DELETE'
        })
        .done(function() {
            alert('SSO configuration deleted successfully');
            loadSSOConfigs();
        })
        .fail(function(xhr) {
            alert('Failed to delete SSO configuration: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

function testSSOConfig(provider) {
    if (confirm('This will open a new window to test the SSO login. Continue?')) {
        window.open('/api/sso/configs/' + provider + '/test', '_blank');
    }
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// User management functions
function showAddUserModal() {
    console.log('showAddUserModal called');
    var modalHtml = '<div class="modal fade" id="addUserModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog" role="document"><div class="modal-content">' +
        '<div class="modal-header">' +
        '<h4 class="modal-title">Add New User</h4>' +
        '<button type="button" class="close" data-dismiss="modal" aria-label="Close">' +
        '<span aria-hidden="true">&times;</span></button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<form id="addUserForm">' +
        '<div class="form-group">' +
        '<label>Username:</label>' +
        '<input type="text" class="form-control" id="newUsername" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label>Password:</label>' +
        '<input type="password" class="form-control" id="newPassword" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label><input type="checkbox" id="newIsAdmin"> Admin User</label>' +
        '</div>' +
        '</form>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        '<button type="button" class="btn btn-primary" onclick="createUser()">Create User</button>' +
        '</div>' +
        '</div></div></div>';

    // Remove any existing modal first
    $('#addUserModal').remove();
    $('body').append(modalHtml);

    // Initialize and show modal
    $('#addUserModal').modal({
        backdrop: true,
        keyboard: true,
        show: true
    });

    // Clean up when modal is hidden
    $('#addUserModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });
}

function createUser() {
    var userData = {
        username: $('#newUsername').val(),
        password: $('#newPassword').val(),
        is_admin: $('#newIsAdmin').is(':checked')
    };

    $.ajax({
        url: '/api/users',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify(userData)
    })
    .done(function() {
        $('#addUserModal').modal('hide');
        setTimeout(function() {
            loadUsersData(); // Refresh the users list
            alert('User created successfully');
        }, 300); // Wait for modal to close
    })
    .fail(function(xhr) {
        alert('Failed to create user: ' + (xhr.responseText || 'Unknown error'));
    });
}

function editUser(username) {
    console.log('editUser called for:', username);
    var modalHtml = '<div class="modal fade" id="editUserModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog" role="document"><div class="modal-content">' +
        '<div class="modal-header">' +
        '<h4 class="modal-title">Edit User: ' + username + '</h4>' +
        '<button type="button" class="close" data-dismiss="modal" aria-label="Close">' +
        '<span aria-hidden="true">&times;</span></button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<form id="editUserForm">' +
        '<div class="form-group">' +
        '<label>Username:</label>' +
        '<input type="text" class="form-control" id="editUsername" value="' + username + '" readonly>' +
        '</div>' +
        '<div class="form-group">' +
        '<label>New Password (leave blank to keep current):</label>' +
        '<input type="password" class="form-control" id="editPassword">' +
        '</div>' +
        '<div class="form-group">' +
        '<label><input type="checkbox" id="editIsAdmin"> Admin User</label>' +
        '</div>' +
        '</form>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        '<button type="button" class="btn btn-primary" onclick="updateUser(\'' + username + '\')">Update User</button>' +
        '</div>' +
        '</div></div></div>';

    // Remove any existing modal first
    $('#editUserModal').remove();
    $('body').append(modalHtml);

    // Initialize and show modal
    $('#editUserModal').modal({
        backdrop: true,
        keyboard: true,
        show: true
    });

    // Clean up when modal is hidden
    $('#editUserModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });

    // Load current user data
    $.get('/user/' + username)
        .done(function(user) {
            $('#editIsAdmin').prop('checked', user.is_admin || user.IsAdmin);
        });
}

function updateUser(username) {
    var userData = {
        username: username,
        is_admin: $('#editIsAdmin').is(':checked')
    };

    if ($('#editPassword').val()) {
        userData.password = $('#editPassword').val();
    }

    $.ajax({
        url: '/api/users',
        method: 'PUT',
        contentType: 'application/json',
        data: JSON.stringify(userData)
    })
    .done(function() {
        $('#editUserModal').modal('hide');
        setTimeout(function() {
            loadUsersData(); // Refresh the users list
            alert('User updated successfully');
        }, 300); // Wait for modal to close
    })
    .fail(function(xhr) {
        alert('Failed to update user: ' + (xhr.responseText || 'Unknown error'));
    });
}

function deleteUser(username) {
    if (confirm('Are you sure you want to delete user: ' + username + '?')) {
        $.ajax({
            url: '/user/' + username,
            method: 'DELETE'
        })
        .done(function() {
            loadUsersData(); // Refresh the users list
            alert('User deleted successfully');
        })
        .fail(function(xhr) {
            alert('Failed to delete user: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

// Tunnel management functions
function deleteTunnel(tunnelId) {
    if (confirm('Are you sure you want to close this tunnel? This will terminate the connection.')) {
        $.ajax({
            url: '/api/tunnels/' + tunnelId,
            method: 'DELETE'
        })
        .done(function() {
            loadTunnelsData();
            if (window.currentView === 'dashboard') {
                loadStats();
                loadQuickAccess();
            }
            alert('Tunnel closed successfully');
        })
        .fail(function(xhr) {
            alert('Failed to close tunnel: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

// Target/Response modal functions
function showTargetResponseModal(listenerId, content, mode) {
    var title = mode === 'proxy' ? 'Target URL Details' : 'Response Content Details';
    var label = mode === 'proxy' ? 'Target URL:' : 'Response Content:';

    $('#targetResponseModalTitle').text(title);
    $('#targetResponseLabel').text(label);
    $('#targetResponseContent').val(content);
    $('#targetResponseModal').modal('show');
}

function copyTargetResponseContent() {
    var content = $('#targetResponseContent').val();
    navigator.clipboard.writeText(content).then(function() {
        // Show success feedback
        var btn = $('button[onclick="copyTargetResponseContent()"]');
        var originalText = btn.text();
        btn.text('Copied!').addClass('btn-success').removeClass('btn-primary');
        setTimeout(function() {
            btn.text(originalText).addClass('btn-primary').removeClass('btn-success');
        }, 2000);
    }).catch(function() {
        alert('Failed to copy to clipboard');
    });
}

function showAddPortReservationModal(preselectedUser) {
    var modalHtml = '<div class="modal fade" id="addPortReservationModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog" role="document">' +
        '<div class="modal-content">' +
        '<div class="modal-header">' +
        '<h5 class="modal-title">Assign Port Range to User</h5>' +
        '<button type="button" class="close" data-dismiss="modal">' +
        '<span>&times;</span>' +
        '</button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<form id="addPortReservationForm">' +
        '<div class="form-group">' +
        '<label for="reservationUsername">Username</label>' +
        '<select class="form-control" id="reservationUsername" required>' +
        '<option value="">Loading users...</option>' +
        '</select>' +
        '</div>' +
        '<div class="form-row">' +
        '<div class="form-group col-md-6">' +
        '<label for="reservationStartPort">Start Port</label>' +
        '<input type="number" class="form-control" id="reservationStartPort" min="1" max="65535" required>' +
        '</div>' +
        '<div class="form-group col-md-6">' +
        '<label for="reservationEndPort">End Port</label>' +
        '<input type="number" class="form-control" id="reservationEndPort" min="1" max="65535" required>' +
        '</div>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="reservationDescription">Description (optional)</label>' +
        '<input type="text" class="form-control" id="reservationDescription" placeholder="e.g., Development ports for user">' +
        '</div>' +
        '</form>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        '<button type="button" class="btn btn-primary" onclick="createPortReservation()">Assign Ports</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('body').append(modalHtml);

    // Load users for dropdown
    $.get('/users')
        .done(function(data) {
            var options = '<option value="">Select a user...</option>';
            if (data && data.length > 0) {
                data.forEach(function(user) {
                    var username = user.username || user.name;
                    var selected = (preselectedUser && username === preselectedUser) ? ' selected' : '';
                    options += '<option value="' + username + '"' + selected + '>' + username + '</option>';
                });
            }
            $('#reservationUsername').html(options);

            // Make it searchable with Select2 if available, otherwise use regular select
            if (typeof $.fn.select2 !== 'undefined') {
                $('#reservationUsername').select2({
                    placeholder: 'Select or type to search...',
                    allowClear: true
                });
            }
        })
        .fail(function() {
            $('#reservationUsername').html('<option value="">Failed to load users</option>');
        });

    $('#addPortReservationModal').modal('show');

    // Remove modal from DOM when closed
    $('#addPortReservationModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });
}

function createPortReservation() {
    // Only allow if user is admin
    if (!window.isAdmin) {
        alert('Admin access required');
        return;
    }

    var username = $('#reservationUsername').val().trim();
    var startPort = parseInt($('#reservationStartPort').val());
    var endPort = parseInt($('#reservationEndPort').val());
    var description = $('#reservationDescription').val().trim();

    if (!username) {
        alert('Username is required');
        return;
    }
    if (isNaN(startPort) || isNaN(endPort)) {
        alert('Please enter valid port numbers');
        return;
    }
    if (startPort > endPort) {
        alert('Start port must be less than or equal to end port');
        return;
    }

    $.ajax({
        url: '/api/port-reservations',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify({
            username: username,
            start_port: startPort,
            end_port: endPort,
            description: description
        })
    })
    .done(function() {
        $('#addPortReservationModal').modal('hide');
        // Refresh users data to show updated port reservations
        if (window.currentView === 'users') {
            loadUsersData();
        }
        alert('Port reservation created successfully!');
    })
    .fail(function(xhr) {
        alert('Failed to create port reservation: ' + (xhr.responseText || 'Unknown error'));
    });
}

function deletePortReservation(reservationId) {
    // Only allow if user is admin
    if (!window.isAdmin) {
        alert('Admin access required');
        return;
    }

    if (confirm('Are you sure you want to remove this port reservation? The user will lose access to these ports.')) {
        $.ajax({
            url: '/api/port-reservations/' + reservationId,
            method: 'DELETE'
        })
        .done(function() {
            loadAdminPortReservations();
            alert('Port reservation removed successfully');
        })
        .fail(function(xhr) {
            alert('Failed to remove port reservation: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

function removeUserPortReservations(username) {
    // Only allow if user is admin
    if (!window.isAdmin) {
        alert('Admin access required');
        return;
    }

    if (confirm('Are you sure you want to remove ALL port reservations for user "' + username + '"? This will revoke their access to all reserved ports.')) {
        // Get all reservations for this user and delete them
        $.get('/api/port-reservations')
            .done(function(data) {
                var userReservations = data.filter(function(res) {
                    return res.username === username;
                });

                if (userReservations.length === 0) {
                    alert('No port reservations found for this user.');
                    return;
                }

                // Delete all reservations for this user
                var deletePromises = userReservations.map(function(reservation) {
                    return $.ajax({
                        url: '/api/port-reservations/' + reservation.id,
                        method: 'DELETE'
                    });
                });

                // Wait for all deletions to complete
                Promise.all(deletePromises)
                    .then(function() {
                        // Refresh users data to show updated port reservations
                        if (window.currentView === 'users') {
                            loadUsersData();
                        }
                        alert('All port reservations removed successfully for user "' + username + '"');
                    })
                    .catch(function(xhr) {
                        alert('Failed to remove some port reservations: ' + (xhr.responseText || 'Unknown error'));
                    });
            })
            .fail(function() {
                alert('Failed to load port reservations');
            });
    }
}

// Backward compatibility function for tunnels
function showTunnelPayloads(tunnelId) {
    return showTrafficPayloads(tunnelId, 'tunnel');
}

// Traffic inspection function with Live SSE, Recent, filters, and pretty/raw toggle
// Works for both tunnels and listeners
function showTrafficPayloads(entityId, entityType) {
    entityType = entityType || 'tunnel'; // default to tunnel for backward compatibility
    var modalHtml = '<div class="modal fade" id="payloadsModal" tabindex="-1" style="z-index: 1060;">' +
        '<div class="modal-dialog modal-xl"><div class="modal-content">' +
        '<div class="modal-header">' +
        '<h4 class="modal-title">Traffic: ' + entityId.substring(0, 8) + '... (' + entityType + ')</h4>' + '<small class="text-muted ml-2">Live capture</small>' +
        '<button type="button" class="close" data-dismiss="modal">&times;</button>' +
        '</div>' +
        '<div class="modal-body" style="max-height: 70vh; overflow-y: auto;">' +
        '<div class="mb-2 d-flex align-items-center sticky-top bg-white pt-2" style="z-index: 1020; border-bottom: 1px solid #eee;">' +
        '<div class="btn-group btn-group-sm mr-2" role="group">' +
        '<button id="btn-live" type="button" class="btn btn-primary">Live</button>' +
        '<button id="btn-recent" type="button" class="btn btn-secondary">Recent</button>' +
        '</div>' +
        '<select id="filter-type" class="form-control form-control-sm mr-2" style="width:auto;">' +
        '<option value="">All types</option>' +
        '<option>conn_open</option><option>conn_close</option>' +
        '<option>metric</option><option>req_headers</option><option>req_body</option>' +
        '<option>res_headers</option><option>res_body</option>' +
        '</select>' +
        '<input id="filter-text" class="form-control form-control-sm mr-2" style="width:200px;" placeholder="contains..." />' +
        '<div class="form-check form-check-inline">' +
        '<input class="form-check-input" type="checkbox" id="pretty-toggle" checked>' +
        '<label class="form-check-label" for="pretty-toggle">Pretty</label>' +
        '</div>' +
        '</div>' +
        '<div id="live-view" style="display:block"><div id="live-events"></div></div>' +
        '<div id="recent-view" style="display:none"><div id="recent-events">Loading...</div></div>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<small class="text-muted mr-auto">Bodies are truncated; headers parsed when possible.</small>' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>' +
        '</div>' +
        '</div></div></div>';

    // Remove existing modal and append
    $('#payloadsModal').remove();
    $('body').append(modalHtml);
    var es = null;
    var prettyMode = true;
    var typeFilter = '';
    var textFilter = '';
    var liveBuffer = [];

    // Helpers
    function decodeData(b64) {
        if (!b64) return '';
        try { return atob(b64); } catch(e) { return ''; }
    }
    function looksLikeJSON(s){ return s && (s.trim().startsWith('{') || s.trim().startsWith('[')); }
    function looksLikeXML(s){ return s && s.trim().startsWith('<') && s.indexOf('>')>0; }
    function fmtBody(text){
        if (!text) return '';
        if (!prettyMode) return text;
        if (looksLikeJSON(text)) { try { return JSON.stringify(JSON.parse(text), null, 2); } catch(e){} }
        // Simple XML pretty: fallback to original for now
        return text;
    }
    function passFilters(e, dataText){
        var t = (e.type || e.Type || '').toLowerCase();
        if (typeFilter && t !== typeFilter) return false;
        if (textFilter && dataText.toLowerCase().indexOf(textFilter) === -1) return false;
        return true;
    }
    function renderEvent(container, e) {
        var ts = new Date(e.time || e.Time || Date.now()).toLocaleString();
        var dataText = decodeData(e.data || e.Data);
        if (!passFilters(e, (dataText||'').toLowerCase())) return;
        var meta = e.meta || e.Meta || {};
        var conn = meta.conn_id || e.conn_id || e.ConnID || '';
        var safe = $('<div>').text(fmtBody(dataText)).html();
        var badgeClass = 'badge-info';
        var et = (e.type || e.Type);
        if (et === 'metric') badgeClass = 'badge-secondary';
        if (et === 'conn_open') badgeClass = 'badge-success';
        if (et === 'conn_close') badgeClass = 'badge-dark';
        var card = '<div class="card mb-2">' +
            '<div class="card-header py-1"><span class="badge ' + badgeClass + ' mr-2">' + et + '</span>' +
            '<small class="text-muted">' + ts + (conn ? ' · ' + conn : '') + '</small></div>' +
            '<div class="card-body py-2">' +
            (dataText ? ('<pre style="white-space:pre-wrap; font-size:12px; max-height:200px; overflow:auto;">' + safe + '</pre>') : '<span class="text-muted">(no body)</span>') +
            '</div></div>';
        $(container).prepend(card);
    }
    function rerender(container, list){
        $(container).empty();
        list.forEach(function(e){ renderEvent(container, e); });
    }

    // Live
    function startLive() {
        if (es) return;
        var streamUrl = '/api/capture/' + entityType + 's/' + entityId + '/stream';
        es = new EventSource(streamUrl);
        es.onmessage = function(ev) {
            try { var e = JSON.parse(ev.data); liveBuffer.push(e); renderEvent('#live-events', e); if (liveBuffer.length>500) liveBuffer.shift(); } catch(_) {}
        };
        es.onerror = function() { /* keep trying */ };
    }
    function stopLive() { if (es) { es.close(); es = null; } }

    // Recent
    var recentBuffer = [];
    function loadRecent() {
        $('#recent-events').text('Loading...');
        var recentUrl = '/api/capture/' + entityType + 's/' + entityId + '/recent';
        $.get(recentUrl)
          .done(function(list){ recentBuffer = list || []; rerender('#recent-events', recentBuffer); })
          .fail(function(){ $('#recent-events').html('<div class="text-danger">Failed to load recent events</div>'); });
    }

    // Show modal
    $('#payloadsModal').modal({ backdrop: true, keyboard: true, show: true });
    $('#payloadsModal').on('hidden.bs.modal', function(){ stopLive(); $(this).remove(); });

    // Toggle buttons
    $(document).off('click', '#btn-live').on('click', '#btn-live', function(){
        $('#btn-live').addClass('btn-primary').removeClass('btn-secondary');
        $('#btn-recent').addClass('btn-secondary').removeClass('btn-primary');
        $('#live-view').show(); $('#recent-view').hide(); startLive();
    });
    $(document).off('click', '#btn-recent').on('click', '#btn-recent', function(){
        $('#btn-recent').addClass('btn-primary').removeClass('btn-secondary');
        $('#btn-live').addClass('btn-secondary').removeClass('btn-primary');
        $('#recent-view').show(); $('#live-view').hide(); stopLive(); loadRecent();
    });

    // Filters
    $(document).off('change', '#filter-type').on('change', '#filter-type', function(){ typeFilter = (this.value||'').toLowerCase(); rerender('#live-events', liveBuffer); rerender('#recent-events', recentBuffer); });
    $(document).off('input', '#filter-text').on('input', '#filter-text', function(){ textFilter = (this.value||'').toLowerCase(); rerender('#live-events', liveBuffer); rerender('#recent-events', recentBuffer); });
    $(document).off('change', '#pretty-toggle').on('change', '#pretty-toggle', function(){ prettyMode = this.checked; rerender('#live-events', liveBuffer); rerender('#recent-events', recentBuffer); });

    // Default to live
    startLive();
}

// Initialize dashboard
$(document).ready(function() {
    console.log('Dashboard initialized');
    window.currentView = 'dashboard';
    loadUserInfo();

    // Load initial dashboard view
    loadDashboard();

    // Test if functions are available
    console.log('showAddUserModal function:', typeof showAddUserModal);
    console.log('editUser function:', typeof editUser);

    // Bind click handler safely (works even if inline onclick fails)
    $(document).on('click', '#add-user-btn', function(e) {
        e.preventDefault();
        console.log('Add User button clicked via delegated handler');
        showAddUserModal();
    });

    // Test the functions directly
    console.log('Testing showAddUserModal function...');
    if (typeof window.showAddUserModal === 'function') {
        console.log('showAddUserModal is available in window');
    } else {
        console.log('showAddUserModal is NOT available in window');
    }

    // Delegated handlers for dynamic elements
    $(document).on('click', '.edit-user-btn', function(e) {
        e.preventDefault();
        var username = $(this).data('username');
        console.log('Edit button clicked for', username);
        editUser(username);
    });
    $(document).on('click', '.delete-user-btn', function(e) {
        e.preventDefault();
        var username = $(this).data('username');
        console.log('Delete button clicked for', username);
        deleteUser(username);
    });

    // Disable auto-refresh to prevent page jumping
    // TODO: Implement proper data-only refresh without view changes
    /*
    setInterval(function(){
        switch (window.currentView) {
            case 'dashboard': return loadDashboard();
            case 'tunnels': return loadTunnelsView();
            case 'users': return loadUsersView();
            case 'listeners': return loadListenersData();
            case 'logs': return loadLogsView();
        }
    }, 30000);
    */
    // Ensure nav click handlers are global before the script ends
    window.showDashboard = showDashboard;
    window.showTunnels = showTunnels;
    window.showUsers = showUsers;
    window.showListeners = showListeners;
    window.showLogs = showLogs;
    window.showSettings = showSettings;
    window.showUserSettings = showUserSettings;
    window.showServerSettings = showServerSettings;
});

// Load user info and set permissions
function loadUserInfo() {
    return $.get('/api/user/info')
    .done(function(data) {
        window.currentUser = data;
        window.isAdmin = data.admin || false;

        // Show/hide admin-only menu items
        if (window.isAdmin) {
            $('.admin-only').show();
        } else {
            $('.admin-only').hide();
        }

        // Update user display
        if (data.username) {
            $('#user-name').text(data.username);
        }

        // Show welcome message if display name is set
        if (data.display_name) {
            $('#welcome-message').text('Welcome back, ' + data.display_name + '!').show();
        } else {
            $('#welcome-message').hide();
        }
    })
    .fail(function() {
        console.log('Failed to load user info');
        // Default to non-admin if API fails
        window.isAdmin = false;
        $('.admin-only').hide();
    });
}

</script>
</body>
</html>`

	t, err := template.New("dashboard").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, nil)
}

// serveDashboardLogin serves the login page
func (s *Server) serveDashboardLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		s.handleDashboardLogin(w, r)
		return
	}

	loginHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>chiSSL - Login</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/admin-lte@3.2/dist/css/adminlte.min.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css">
</head>
<body class="hold-transition login-page">
<div class="login-box">
    <div class="login-logo">
        <img src="/dashboard/static/images/chissl-132-100.png" alt="chiSSL" style="max-width: 132px; height: auto; display: block; margin: 0 auto 8px;">
        <b>chiSSL</b> Dashboard
    </div>
    <div class="card">
        <div class="card-body login-card-body">
            <p class="login-box-msg">Sign in to access the dashboard</p>
            <div id="error-message" class="alert alert-danger" style="display: none;"></div>

            <!-- SSO Login Options -->
            <div id="sso-login-options" class="mb-3"></div>

            <!-- Traditional Login Form -->
            <form method="post">
                <div class="input-group mb-3">
                    <input type="text" name="username" class="form-control" placeholder="Username" required>
                    <div class="input-group-append">
                        <div class="input-group-text">
                            <span class="fas fa-user"></span>
                        </div>
                    </div>
                </div>
                <div class="input-group mb-3">
                    <input type="password" name="password" class="form-control" placeholder="Password" required>
                    <div class="input-group-append">
                        <div class="input-group-text">
                            <span class="fas fa-lock"></span>
                        </div>
                    </div>
                </div>
                <div class="row">
                    <div class="col-12">
                        <button type="submit" class="btn btn-primary btn-block">Sign In</button>
                    </div>
                </div>
            </form>
        </div>
    </div>
</div>
<script src="https://code.jquery.com/jquery-3.6.0.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/js/bootstrap.bundle.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/admin-lte@3.2/dist/js/adminlte.min.js"></script>
<script>
$(document).ready(function() {
    // Check for error parameter in URL
    const urlParams = new URLSearchParams(window.location.search);
    const error = urlParams.get('error');
    if (error === 'invalid_credentials') {
        $('#error-message').text('Invalid username or password').show();
    } else if (error === 'rate_limited') {
        var retry = parseInt(urlParams.get('retry') || '0', 10);
        var msg = retry > 0 ? ('Too many login attempts. Try again in ' + retry + 's.') : 'Too many login attempts. Please try again later.';
        $('#error-message').text(msg).show();
    }

    // Load SSO login options
    loadSSOLoginOptions();
});

function loadSSOLoginOptions() {
    $.get('/api/sso/enabled')
        .done(function(configs) {
            var html = '';
            if (configs && configs.length > 0) {
                var enabledConfigs = configs.filter(function(config) {
                    return config.enabled;
                });

                if (enabledConfigs.length > 0) {
                    html += '<div class="text-center mb-3">';
                    enabledConfigs.forEach(function(config) {
                        var providerIcon = getProviderIcon(config.provider);
                        var providerName = config.provider.charAt(0).toUpperCase() + config.provider.slice(1);
                        var loginUrl = getSSOLoginUrl(config.provider);

                        html += '<a href="' + loginUrl + '" class="btn btn-outline-primary btn-block mb-2">' +
                            '<i class="' + providerIcon + '"></i> Sign in with ' + providerName +
                            '</a>';
                    });
                    html += '<hr class="my-3"><div class="text-muted text-center small">Or sign in with your account</div></div>';
                }
            }
            $('#sso-login-options').html(html);
        })
        .fail(function() {
            // Don't show error for SSO options as it's optional
        });
}

function getProviderIcon(provider) {
    switch(provider) {
        case 'okta': return 'fab fa-okta';
        case 'auth0': return 'fas fa-shield-alt';
        case 'scim': return 'fas fa-users-cog';
        case 'azure': return 'fab fa-microsoft';
        default: return 'fas fa-cloud';
    }
}

function getSSOLoginUrl(provider) {
    switch(provider) {
        case 'okta':
        case 'scim':
            return '/auth/scim/login';
        case 'auth0':
            return '/auth/auth0/login';
        case 'azure':
            return '/auth/azure/login';
        default:
            return '/auth/' + provider + '/login';
    }
}
</script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(loginHTML))
}

// handleDashboardLogin processes login form submission
func (s *Server) handleDashboardLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	s.Debugf("Login attempt: username=%s", username)

	// Global IP-only rate limiter first
	ip := s.clientIP(r)
	if locked, retry := s.ipRateCheck(ip); locked {
		if retry > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retry.Seconds())))
		}
		http.Redirect(w, r, "/dashboard?error=rate_limited&retry="+fmt.Sprintf("%d", int(retry.Seconds())), http.StatusSeeOther)
		return
	}
	// Apply per-username backoff (optionally bucketed per-IP)
	if username != "" {
		locked, retryAfter, delay := s.nextLoginDelayFor(username, ip)
		if locked {
			if retryAfter > 0 {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
			}
			// Redirect back to login with a friendly message instead of raw 429
			http.Redirect(w, r, "/dashboard?error=rate_limited&retry="+fmt.Sprintf("%d", int(retryAfter.Seconds())), http.StatusSeeOther)
			return
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	// Validate credentials - check both database and in-memory users
	authenticated := false

	// First check in-memory users (for --auth flag users)
	user, found := s.users.Get(username)
	if found && user.Name == username && user.Pass == password {
		s.Debugf("Authentication successful via in-memory users")
		authenticated = true
	}

	// If not found in memory and database is available, check database
	if !authenticated && s.db != nil {
		dbUser, err := s.db.GetUser(username)
		if err == nil && dbUser.Password == password {
			s.Debugf("Authentication successful via database")
			authenticated = true
		}
	}

	if authenticated {
		// reset backoff state on success
		ip := s.clientIP(r)
		s.resetLoginBackoffFor(username, ip)
		// record IP attempt
		s.ipRateRecord(ip)
		s.Debugf("Setting session cookie for user: %s", username)
		// Set session cookie (simplified)
		http.SetCookie(w, &http.Cookie{
			Name:     "chissl_session",
			Value:    username, // In production, use a secure session token
			Path:     "/",
			HttpOnly: true,
			Secure:   false, // Set to true in production with HTTPS
			MaxAge:   3600,  // 1 hour
		})
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	// record failure for backoff
	if username != "" {
		ip := s.clientIP(r)
		s.recordLoginFailureFor(username, ip)
		// record IP attempt
		s.ipRateRecord(ip)
	}

	s.Debugf("Authentication failed for user: %s", username)
	// Login failed - redirect back to login with error
	http.Redirect(w, r, "/dashboard?error=invalid_credentials", http.StatusSeeOther)
}

// handleDashboardLogout handles logout
func (s *Server) handleDashboardLogout(w http.ResponseWriter, r *http.Request) {
	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "chissl_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1, // Delete cookie
	})
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// serveDashboardAsset serves static assets
func (s *Server) serveDashboardAsset(w http.ResponseWriter, r *http.Request) {
	// Remove /dashboard prefix from path
	assetPath := r.URL.Path
	if len(assetPath) > 10 && assetPath[:10] == "/dashboard" {
		assetPath = assetPath[10:]
	}

	// Remove leading slash
	if len(assetPath) > 0 && assetPath[0] == '/' {
		assetPath = assetPath[1:]
	}

	// Build full file path
	fullPath := filepath.Join("server", "dashboard", assetPath)

	// Check if file exists and read it
	// Try embedded assets first
	embeddedPath := filepath.Join("dashboard", assetPath)
	content, err := embeddedDashboardFS.ReadFile(embeddedPath)
	if err != nil {
		// Fallback to disk in case of dev mode or missing embed
		content, err = os.ReadFile(fullPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	// Set appropriate content type based on file extension
	ext := filepath.Ext(assetPath)
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".html":
		w.Header().Set("Content-Type", "text/html")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".woff":
		w.Header().Set("Content-Type", "font/woff")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	w.Write(content)
}
