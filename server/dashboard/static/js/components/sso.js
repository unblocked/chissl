// SSO Components
// Handles SSO banners, configuration, and authentication

console.log('SSO components module loaded');

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
                var bannerHtml = '<div class="alert alert-warning alert-dismissible">' +
                    '<button type="button" class="close" data-dismiss="alert" aria-hidden="true">&times;</button>' +
                    '<h6><i class="fas fa-key"></i> Token Required for Tunnel Client</h6>' +
                    '<div>Since you\'re using SSO authentication, you need to generate an API token to use with the tunnel client.</div>' +
                    '<div class="mt-2">' +
                    '<button class="btn btn-sm btn-primary" onclick="showUserSettings()">' +
                    '<i class="fas fa-plus"></i> Generate Token</button>' +
                    '<small class="ml-2 text-muted">Go to User Settings → API Tokens</small>' +
                    '</div>' +
                    '</div>';
                $('#sso-banner').html(bannerHtml);
            }
        })
        .fail(function() {
            // Don't show error for SSO banner as it's optional
        });
}

// SSO Configuration Functions
function loadSSOConfigs() {
    $.get('/api/sso/configs')
        .done(function(configs) {
            var configsHtml = '';
            if (configs && configs.length > 0) {
                configs.forEach(function(config) {
                    var statusBadge = config.enabled ? 
                        '<span class="badge badge-success">Enabled</span>' : 
                        '<span class="badge badge-secondary">Disabled</span>';
                    
                    configsHtml += '<div class="card mb-3">' +
                        '<div class="card-body">' +
                        '<div class="d-flex justify-content-between align-items-center">' +
                        '<div>' +
                        '<h5 class="card-title mb-1">' +
                        '<i class="' + getProviderIcon(config.provider) + ' mr-2"></i>' +
                        config.provider.toUpperCase() + ' SSO</h5>' +
                        '<p class="card-text text-muted mb-0">Single Sign-On configuration</p>' +
                        '</div>' +
                        '<div>' +
                        statusBadge +
                        '<div class="btn-group ml-2" role="group">' +
                        '<button class="btn btn-sm btn-outline-primary" onclick="editSSOConfig(\'' + config.provider + '\')">' +
                        '<i class="fas fa-edit"></i> Edit</button>' +
                        '<button class="btn btn-sm btn-outline-success" onclick="testSSOConfig(\'' + config.provider + '\')">' +
                        '<i class="fas fa-test-tube"></i> Test</button>' +
                        '<button class="btn btn-sm btn-outline-danger" onclick="deleteSSOConfig(\'' + config.provider + '\')">' +
                        '<i class="fas fa-trash"></i> Delete</button>' +
                        '</div>' +
                        '</div>' +
                        '</div>' +
                        '</div>' +
                        '</div>';
                });
            } else {
                configsHtml = '<div class="text-center text-muted py-4">' +
                    '<i class="fas fa-shield-alt fa-3x mb-3"></i>' +
                    '<h5>No SSO Configurations</h5>' +
                    '<p>Add an SSO provider to enable single sign-on authentication.</p>' +
                    '</div>';
            }
            $('#sso-configs-list').html(configsHtml);
        })
        .fail(function() {
            $('#sso-configs-list').html('<div class="alert alert-danger">Failed to load SSO configurations</div>');
        });
}

function getProviderIcon(provider) {
    switch(provider) {
        case 'okta': return 'fab fa-okta';
        case 'google': return 'fab fa-google';
        case 'microsoft': return 'fab fa-microsoft';
        case 'github': return 'fab fa-github';
        default: return 'fas fa-shield-alt';
    }
}

function loadSSOLoginOptions() {
    $.get('/api/sso/enabled')
        .done(function(configs) {
            var ssoHtml = '';
            if (configs && configs.length > 0) {
                ssoHtml += '<div class="text-center mb-3"><small class="text-muted">Or sign in with:</small></div>';
                configs.forEach(function(config) {
                    ssoHtml += '<a href="' + getSSOLoginUrl(config.provider) + '" class="btn btn-outline-primary btn-block mb-2">' +
                        '<i class="' + getProviderIcon(config.provider) + ' mr-2"></i>' +
                        'Sign in with ' + config.provider.charAt(0).toUpperCase() + config.provider.slice(1) +
                        '</a>';
                });
            }
            $('#sso-login-options').html(ssoHtml);
        })
        .fail(function() {
            // Silently fail - SSO options are optional
        });
}

function getSSOLoginUrl(provider) {
    switch(provider) {
        case 'okta':
        case 'google':
        case 'microsoft':
        case 'github':
            return '/auth/' + provider;
        default:
            return '/auth/sso/' + provider;
    }
}
