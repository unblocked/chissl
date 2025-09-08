// Dashboard Navigation System
// Handles view switching and URL routing

console.log('🚨🚨🚨 NAVIGATION MODULE LOADED - VERSION 2024-09-07-01:40 - AI MOCK HOSTNAME FIX 🚨🚨🚨');

// Inject menu items dynamically with feature flags
$(document).ready(function() {
    window.dashboardApp = window.dashboardApp || {};
    window.dashboardApp.features = window.dashboardApp.features || {};

    // Always inject new Listeners and Tunnels
    var newListenersMenuItem = '<li class="nav-item">' +
        '<a href="#" class="nav-link" onclick="showNewListeners()">' +
        '<i class="nav-icon fas fa-server"></i>' +
        '<p>Listeners</p>' +
        '</a>' +
        '</li>';

    var newTunnelsMenuItem = '<li class="nav-item">' +
        '<a href="#" class="nav-link" onclick="showNewTunnels()">' +
        '<i class="nav-icon fas fa-exchange-alt"></i>' +
        '<p>Tunnels</p>' +
        '</a>' +
        '</li>';

    $('a[onclick="showListeners()"]').closest('li').after(newListenersMenuItem);
    $('a[onclick="showTunnels()"]').closest('li').after(newTunnelsMenuItem);
    console.log('🆕 New Listeners menu item injected dynamically');
    console.log('🆕 New Tunnels menu item injected dynamically');

    // Hide legacy (old) menu items for Tunnels and Listeners
    $('a[onclick="showTunnels()"]').closest('li').hide();
    $('a[onclick="showListeners()"]').closest('li').hide();

    // Load New Listeners and New Tunnels scripts
    var newListenersScript = document.createElement('script');
    newListenersScript.src = '/dashboard/static/js/views/new-listeners.js';
    newListenersScript.onload = function() { console.log('🆕 New Listeners JavaScript loaded dynamically'); };
    document.head.appendChild(newListenersScript);

    var newTunnelsScript = document.createElement('script');
    newTunnelsScript.src = '/dashboard/static/js/views/new-tunnels.js';
    newTunnelsScript.onload = function() { console.log('🆕 New Tunnels JavaScript loaded dynamically'); };
    document.head.appendChild(newTunnelsScript);

    // Fetch experimental feature flag for AI Mock visibility
    $.get('/api/settings/feature/ai-mock-visible')
      .done(function(res) {
          var enabled = !!(res && res.enabled);
          window.dashboardApp.features.aiMockVisible = enabled;
          if (enabled) {
              var aiMockMenuItem = '<li class="nav-item">' +
                  '<a href="#" class="nav-link" onclick="showAIMock()">' +
                  '<i class="nav-icon fas fa-robot"></i>' +
                  '<p>AI Mock API</p>' +
                  '</a>' +
                  '</li>';
              $('a[onclick="showListeners()"]').closest('li').after(aiMockMenuItem);
              console.log('🤖 AI Mock API menu item injected dynamically');
              // Load AI Mock view script lazily
              var aiScript = document.createElement('script');
              aiScript.src = '/dashboard/static/js/views/ai-mock.js';
              aiScript.onload = function() { console.log('🤖 AI Mock API JavaScript loaded dynamically'); };
              document.head.appendChild(aiScript);
          } else {
              console.log('🤖 AI Mock API hidden by feature flag');
          }
      })
      .fail(function() {
          window.dashboardApp.features.aiMockVisible = false;
          console.log('🤖 AI Mock API visibility check failed; defaulting to hidden');
      });
});

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
        case 'ai-mock':
            showAIMockFromURL();
            break;
        case 'new-listeners':
            showNewListenersFromURL();
            break;
        case 'new-tunnels':
            showNewTunnelsFromURL();
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
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'dashboard';
    $('#page-title').text('Dashboard');
    $('.nav-link').removeClass('active');
    $('[onclick="showDashboard()"]').addClass('active');
    loadDashboard();
}

function showTunnels() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'tunnels';
    $('#page-title').text('Tunnels');
    $('.nav-link').removeClass('active');
    $('[onclick="showTunnels()"]').addClass('active');
    loadTunnelsView();
}

function showUsers() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'users';
    $('#page-title').text('Users');
    $('.nav-link').removeClass('active');
    $('[onclick="showUsers()"]').addClass('active');
    loadUsersView();
}

function showListeners() {
    console.log('🔥🔥🔥 NAVIGATION: showListeners() called');
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
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
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'user-settings';
    $('#page-title').text('User Settings');
    $('.nav-link').removeClass('active');
    $('[onclick="showUserSettings()"]').addClass('active');
    loadUserSettingsView();
}

function showServerSettings() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
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
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'dashboard';
    $('#page-title').text('Dashboard');
    $('.nav-link').removeClass('active');
    $('[onclick="showDashboard()"]').addClass('active');
    loadDashboard();
}

function showTunnelsFromURL() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'tunnels';
    $('#page-title').text('Tunnels');
    $('.nav-link').removeClass('active');
    $('[onclick="showTunnels()"]').addClass('active');
    loadTunnelsView();
}

function showListenersFromURL() {
    console.log('🔥🔥🔥 NAVIGATION: showListenersFromURL() called');
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'listeners';
    $('#page-title').text('Listeners');

    $('.nav-link').removeClass('active');
    $('[onclick="showListeners()"]').addClass('active');
    loadListenersView();
}

function showAIMock() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'ai-mock';
    $('#page-title').text('AI Mock API');
    if (!(window.dashboardApp.features && window.dashboardApp.features.aiMockVisible)) { console.warn('AI Mock hidden by feature flag'); showDashboardFromURL(); return; }

    $('.nav-link').removeClass('active');
    $('[onclick="showAIMock()"]').addClass('active');
    loadAIMockView();
}

function showAIMockFromURL() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'ai-mock';
    $('#page-title').text('AI Mock API');
    $('.nav-link').removeClass('active');
    if (!(window.dashboardApp.features && window.dashboardApp.features.aiMockVisible)) { console.warn('AI Mock hidden by feature flag'); showDashboardFromURL(); return; }

    $('[onclick="showAIMock()"]').addClass('active');
    loadAIMockView();
}

function showNewListeners() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearAIMockRefreshInterval();
    clearNewListenersRefreshInterval();
    window.dashboardApp.currentView = 'new-listeners';
    $('#page-title').text('Listeners');
    $('.nav-link').removeClass('active');
    $('[onclick="showNewListeners()"]').addClass('active');
    loadNewListenersView();
}

function showNewListenersFromURL() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearAIMockRefreshInterval();
    clearNewListenersRefreshInterval();
    window.dashboardApp.currentView = 'new-listeners';
    $('#page-title').text('Listeners');
    $('.nav-link').removeClass('active');
    $('[onclick="showNewListeners()"]').addClass('active');
    loadNewListenersView();
}

function showNewTunnels() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'new-tunnels';
    $('#page-title').text('Tunnels');
    $('.nav-link').removeClass('active');
    $('[onclick="showNewTunnels()"]').addClass('active');
    loadNewTunnelsView();
}

function showNewTunnelsFromURL() {
    clearLogsRefreshInterval();
    clearNewTunnelsRefreshInterval();
    clearNewListenersRefreshInterval();
    clearAIMockRefreshInterval();
    window.dashboardApp.currentView = 'new-tunnels';
    $('#page-title').text('Tunnels');
    $('.nav-link').removeClass('active');
    $('[onclick="showNewTunnels()"]').addClass('active');
    loadNewTunnelsView();
}

// Helper function to clear new tunnels refresh interval
function clearNewTunnelsRefreshInterval() {
    if (typeof stopNewTunnelsAutoRefresh === 'function') {
        stopNewTunnelsAutoRefresh();
    }
}

// Helper function to clear new listeners refresh interval
function clearNewListenersRefreshInterval() {
    if (typeof stopNewListenersAutoRefresh === 'function') {
        stopNewListenersAutoRefresh();
    }
}

// Helper function to clear AI Mock refresh interval
function clearAIMockRefreshInterval() {
    if (typeof stopAIMockAutoRefresh === 'function') {
        stopAIMockAutoRefresh();
    }
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
