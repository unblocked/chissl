// Dashboard Navigation System
// Handles view switching and URL routing

console.log('Navigation module loaded');

// URL routing and history management
function updateURL(view) {
    // Don't update URL if we're navigating from URL (prevents loops)
    if (window.dashboardApp.isNavigatingFromURL) {
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
    window.dashboardApp.isNavigatingFromURL = true;
    
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
            if (window.dashboardApp.currentUser && window.dashboardApp.currentUser.is_admin) {
                showUsersFromURL();
            } else {
                showDashboardFromURL();
            }
            break;
        case 'logs':
            if (window.dashboardApp.currentUser && window.dashboardApp.currentUser.is_admin) {
                showLogsFromURL();
            } else {
                showDashboardFromURL();
            }
            break;
        case 'user-settings':
            showUserSettingsFromURL();
            break;
        case 'server-settings':
            if (window.dashboardApp.currentUser && window.dashboardApp.currentUser.is_admin) {
                showServerSettingsFromURL();
            } else {
                showDashboardFromURL();
            }
            break;
        default:
            showDashboardFromURL();
    }
    
    // Clear flag after navigation
    window.dashboardApp.isNavigatingFromURL = false;
}

// Main navigation functions (called by menu clicks)
function showDashboard() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'dashboard';
    $('#page-title').text('Dashboard');
    $('.nav-link').removeClass('active');
    $('[onclick="showDashboard()"]').addClass('active');
    loadDashboard();
}

function showTunnels() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'tunnels';
    $('#page-title').text('Tunnels');
    $('.nav-link').removeClass('active');
    $('[onclick="showTunnels()"]').addClass('active');
    loadTunnelsView();
}

function showUsers() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'users';
    $('#page-title').text('Users');
    $('.nav-link').removeClass('active');
    $('[onclick="showUsers()"]').addClass('active');
    loadUsersView();
}

function showListeners() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'listeners';
    $('#page-title').text('Listeners');
    $('.nav-link').removeClass('active');
    $('[onclick="showListeners()"]').addClass('active');
    loadListenersView();
}

function showLogs() {
    // Don't clear interval for logs page - we want auto-refresh here
    window.dashboardApp.currentView = 'logs';
    $('#page-title').text('Logs');
    $('.nav-link').removeClass('active');
    $('[onclick="showLogs()"]').addClass('active');
    loadLogsView();
}

function showUserSettings() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'user-settings';
    $('#page-title').text('User Settings');
    $('.nav-link').removeClass('active');
    $('[onclick="showUserSettings()"]').addClass('active');
    loadUserSettingsView();
}

function showServerSettings() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'server-settings';
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
    window.dashboardApp.currentView = 'dashboard';
    $('#page-title').text('Dashboard');
    $('.nav-link').removeClass('active');
    $('[onclick="showDashboard()"]').addClass('active');
    loadDashboard();
}

function showTunnelsFromURL() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'tunnels';
    $('#page-title').text('Tunnels');
    $('.nav-link').removeClass('active');
    $('[onclick="showTunnels()"]').addClass('active');
    loadTunnelsView();
}

function showListenersFromURL() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'listeners';
    $('#page-title').text('Listeners');
    $('.nav-link').removeClass('active');
    $('[onclick="showListeners()"]').addClass('active');
    loadListenersView();
}

function showUsersFromURL() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'users';
    $('#page-title').text('Users');
    $('.nav-link').removeClass('active');
    $('[onclick="showUsers()"]').addClass('active');
    loadUsersView();
}

function showLogsFromURL() {
    // Don't clear interval for logs page - we want auto-refresh here
    window.dashboardApp.currentView = 'logs';
    $('#page-title').text('Logs');
    $('.nav-link').removeClass('active');
    $('[onclick="showLogs()"]').addClass('active');
    loadLogsView();
}

function showUserSettingsFromURL() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'user-settings';
    $('#page-title').text('User Settings');
    $('.nav-link').removeClass('active');
    $('[onclick="showUserSettings()"]').addClass('active');
    loadUserSettingsView();
}

function showServerSettingsFromURL() {
    clearLogsRefreshInterval();
    window.dashboardApp.currentView = 'server-settings';
    $('#page-title').text('Server Settings');
    $('.nav-link').removeClass('active');
    $('[onclick="showServerSettings()"]').addClass('active');
    loadServerSettingsView();
}
