// Dashboard Application Core
// Main application initialization and global state management

console.log('Dashboard app core loaded');

// Global application state
window.dashboardApp = {
    currentView: 'dashboard',
    currentUser: null,
    isAdmin: false,
    logsRefreshInterval: null,
    isNavigatingFromURL: false
};

// Set up AJAX defaults with credentials
$.ajaxSetup({
    xhrFields: {
        withCredentials: true
    },
    beforeSend: function(xhr) {
        // For basic auth fallback, we could add Authorization header here
    }
});

// Initialize dashboard when DOM is ready
$(document).ready(function() {
    console.log('Dashboard initialized');
    window.dashboardApp.currentView = 'dashboard';
    
    // Load user info first, then initialize views
    loadUserInfo().then(function() {
        // Load initial dashboard view
        loadDashboard();
        
        // Set up periodic refresh for current view (every 30 seconds)
        setInterval(function(){
            switch (window.dashboardApp.currentView) {
                case 'dashboard': return loadDashboard();
                case 'tunnels': return loadTunnelsView();
                case 'users': return loadUsersView();
                case 'listeners': return loadListenersData();
                case 'logs': return loadLogsView();
            }
        }, 30000);
        
        // Set up keyboard shortcuts
        $(document).keydown(function(e) {
            // Ctrl+R or F5 to refresh current view
            if ((e.ctrlKey && e.keyCode === 82) || e.keyCode === 116) {
                e.preventDefault();
                refreshCurrentView();
            }
        });
    });
});

// Load user info and set permissions
function loadUserInfo() {
    return $.get('/api/user/info')
        .done(function(data) {
            window.dashboardApp.currentUser = data;
            window.currentUser = data; // Backward compatibility
            window.dashboardApp.isAdmin = data.is_admin;
            window.isAdmin = data.is_admin; // Backward compatibility
            
            console.log('User loaded:', data.username, 'Admin:', data.is_admin);
            
            // Update UI based on user permissions
            updateUIForUser(data);
        })
        .fail(function() {
            console.error('Failed to load user info');
            // Redirect to login if user info fails
            window.location.href = '/dashboard';
        });
}

// Update UI elements based on user permissions
function updateUIForUser(user) {
    if (user.is_admin) {
        $('.admin-only').show();
        $('.user-only').hide();
    } else {
        $('.admin-only').hide();
        $('.user-only').show();
    }
    
    // Update user display
    $('#user-display-name').text(user.display_name || user.username);
    $('#user-email').text(user.email || '');
}

// Refresh current view
function refreshCurrentView() {
    switch (window.dashboardApp.currentView) {
        case 'dashboard':
            loadDashboard();
            break;
        case 'tunnels':
            loadTunnelsView();
            break;
        case 'users':
            loadUsersView();
            break;
        case 'listeners':
            loadListenersView();
            break;
        case 'logs':
            loadLogsView();
            break;
        case 'user-settings':
            loadUserSettingsView();
            break;
        case 'server-settings':
            loadServerSettingsView();
            break;
    }
}

// Clear logs refresh interval
function clearLogsRefreshInterval() {
    if (window.dashboardApp.logsRefreshInterval) {
        clearInterval(window.dashboardApp.logsRefreshInterval);
        window.dashboardApp.logsRefreshInterval = null;
    }
}

// Utility function for formatting uptime
function formatUptime(seconds) {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return days + 'd ' + hours + 'h ' + minutes + 'm';
}

// Utility function for formatting bytes
function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    var k = 1024;
    var sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    var i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// HTML escape utility
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

// Copy to clipboard utility
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(function() {
        // Show a brief success message
        var toast = $('<div class="toast" style="position: fixed; top: 20px; right: 20px; z-index: 9999; background: #28a745; color: white; padding: 10px 15px; border-radius: 4px;">Copied to clipboard!</div>');
        $('body').append(toast);
        setTimeout(function() {
            toast.fadeOut(function() {
                toast.remove();
            });
        }, 2000);
    }).catch(function(err) {
        console.error('Failed to copy to clipboard:', err);
        // Fallback for older browsers
        var textArea = document.createElement("textarea");
        textArea.value = text;
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
    });
}
