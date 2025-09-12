// Server Settings View Module
// Handles server configuration including AI providers

console.log('Server Settings view module loaded');

function loadServerSettingsView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title"><i class="fas fa-cogs"></i> Server Settings</h3>' +
        '</div>' +
        '<div class="card-body">' +
        '<div class="row">' +
        '<div class="col-md-12">' +
        '<ul class="nav nav-tabs" id="settingsTabs" role="tablist">' +
        '<li class="nav-item">' +
        '<a class="nav-link active" id="general-tab" data-toggle="tab" href="#general" role="tab">General</a>' +
        '</li>' +
        '<li class="nav-item">' +
        '<a class="nav-link" id="security-tab" data-toggle="tab" href="#security" role="tab">Security</a>' +
        '</li>' +
        '<li class="nav-item">' +
        '<a class="nav-link" id="ai-providers-tab" data-toggle="tab" href="#ai-providers" role="tab">AI Providers</a>' +
        '</li>' +
        '</ul>' +
        '<div class="tab-content mt-3" id="settingsTabContent">' +
        '<div class="tab-pane fade show active" id="general" role="tabpanel">' +
        '<div id="general-settings-content">' +
        '<div class="text-center text-muted">Loading general settings...</div>' +
        '</div>' +
        '</div>' +
        '<div class="tab-pane fade" id="security" role="tabpanel">' +
        '<div id="security-settings-content">' +
        '<div class="text-center text-muted">Loading security settings...</div>' +
        '</div>' +
        '</div>' +

        '<div class="tab-pane fade" id="ai-providers" role="tabpanel">' +
        '<div class="d-flex justify-content-between align-items-center mb-3">' +
        '<h5>AI Provider Configurations</h5>' +
        '<button class="btn btn-primary btn-sm" onclick="showAddAIProviderModal()">' +
        '<i class="fas fa-plus"></i> Add AI Provider</button>' +
        '</div>' +
        '<div id="ai-providers-list">' +
        '<div class="text-center text-muted">Loading AI providers...</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('#main-content').html(content);

    // Load general settings content first (since it's the active tab)
    loadGeneralSettings();

    // Set up tab change handlers
    $('#settingsTabs a[data-toggle="tab"]').on('shown.bs.tab', function (e) {
        var target = $(e.target).attr("href");
        if (target === '#ai-providers') {
            loadAIProviders();
        } else if (target === '#security') {
            loadSecuritySettings();
        } else if (target === '#general') {
            loadGeneralSettings();
        }
    });
}

// Load general settings content (original settings view)
function loadGeneralSettings() {
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
        '<button class="btn btn-primary" onclick="updateReservedPortsThreshold()">' +
        '<i class="fas fa-save"></i> Save</button>' +
        '</div>' +
        '</div>' +
        '<small class="form-text text-muted">Ports 0 to this value are reserved for admin assignment</small>' +
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
        '</div>';

    $('#general-settings-content').html(content);
    // Append Experimental Features card (admin-only UI control)
    var expHtml = '\n<div class="row mt-3">\n  <div class="col-md-6">\n    <div class="card">\n      <div class="card-header"><h3 class="card-title">Experimental Features</h3></div>\n      <div class="card-body">\n        <div class="form-group">\n          <label>AI Mock API (experimental)</label>\n          <div class="custom-control custom-radio">\n            <input type="radio" id="aimock-visible-on" name="aimock-visible" class="custom-control-input">\n            <label class="custom-control-label" for="aimock-visible-on">Visible</label>\n          </div>\n          <div class="custom-control custom-radio">\n            <input type="radio" id="aimock-visible-off" name="aimock-visible" class="custom-control-input">\n            <label class="custom-control-label" for="aimock-visible-off">Hidden (default)</label>\n          </div>\n          <button class="btn btn-primary mt-2" onclick="updateAIMockVisibility()"><i class="fas fa-save"></i> Save</button>\n          <small class="form-text text-muted">This only controls UI visibility of the AI Mock page. Data is not deleted.</small>\n        </div>\n      </div>\n    </div>\n  </div>\n</div>\n';
    $('#general-settings-content').append(expHtml);

    // Load experimental features current values
    if (window.dashboardApp.currentUser && window.dashboardApp.currentUser.is_admin) {
        loadExperimentalFeatures();
    }


    // Load the data for general settings
    loadSettingsSystemInfo();
    loadReservedPortsThreshold();
    loadSSOConfigs();
}

// AI Provider Management Functions
function loadAIProviders() {
    $.get('/api/ai-providers')
        .done(function(providers) {
            var providersHtml = '';
            if (providers && providers.length > 0) {
                providers.forEach(function(provider) {
                    var statusBadge = '';
                    switch(provider.test_status) {
                        case 'success':
                            statusBadge = '<span class="badge badge-success">Connected</span>';
                            break;
                        case 'failed':
                            statusBadge = '<span class="badge badge-danger">Failed</span>';
                            break;
                        default:
                            statusBadge = '<span class="badge badge-secondary">Untested</span>';
                    }

                    var enabledBadge = provider.enabled ?
                        '<span class="badge badge-success">Enabled</span>' :
                        '<span class="badge badge-secondary">Disabled</span>';

                    var providerIcon = getAIProviderIcon(provider.provider_type);

                    providersHtml += '<div class="card mb-3">' +
                        '<div class="card-body">' +
                        '<div class="d-flex justify-content-between align-items-start">' +
                        '<div class="flex-grow-1">' +
                        '<h5 class="card-title mb-1">' +
                        '<i class="' + providerIcon + ' mr-2"></i>' +
                        escapeHtml(provider.name) +
                        '</h5>' +
                        '<p class="card-text text-muted mb-2">' +
                        provider.provider_type.toUpperCase() + ' • Model: ' + provider.model +
                        '</p>' +
                        '<div class="mb-2">' +
                        statusBadge + ' ' + enabledBadge +
                        '</div>' +
                        (provider.test_message ?
                            '<small class="text-muted">' + escapeHtml(provider.test_message) + '</small>' : '') +
                        '</div>' +
                        '<div class="btn-group" role="group">' +
                        '<button class="btn btn-sm btn-outline-success" onclick="testAIProvider(\'' + provider.id + '\')" title="Test Connection">' +
                        '<i class="fas fa-plug"></i> Test</button>' +
                        '<button class="btn btn-sm btn-outline-primary" onclick="editAIProvider(\'' + provider.id + '\')" title="Edit">' +
                        '<i class="fas fa-edit"></i> Edit</button>' +
                        '<button class="btn btn-sm btn-outline-danger" onclick="deleteAIProvider(\'' + provider.id + '\')" title="Delete">' +
                        '<i class="fas fa-trash"></i> Delete</button>' +
                        '</div>' +
                        '</div>' +
                        '</div>' +
                        '</div>';
                });
            } else {
                providersHtml = '<div class="text-center text-muted py-4">' +
                    '<i class="fas fa-robot fa-3x mb-3"></i>' +
                    '<h5>No AI Providers Configured</h5>' +
                    '<p>Add an AI provider to enable AI-powered mock API generation.</p>' +
                    '</div>';
            }
            $('#ai-providers-list').html(providersHtml);
        })
        .fail(function() {
            $('#ai-providers-list').html('<div class="alert alert-danger">Failed to load AI providers</div>');
        });
}

function getAIProviderIcon(providerType) {
    switch(providerType) {
        case 'openai': return 'fas fa-brain';
        case 'claude': return 'fas fa-robot';
        default: return 'fas fa-microchip';
    }
}

function showAddAIProviderModal() {
    var modalHtml = '<div class="modal fade" id="addAIProviderModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog modal-lg" role="document">' +
        '<div class="modal-content">' +
        '<div class="modal-header">' +
        '<h4 class="modal-title">Add AI Provider</h4>' +
        '<button type="button" class="close" data-dismiss="modal">&times;</button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<form id="addAIProviderForm">' +
        '<div class="form-group">' +
        '<label for="providerName">Name</label>' +
        '<input type="text" class="form-control" id="providerName" placeholder="e.g., Production OpenAI" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="providerType">Provider Type</label>' +
        '<select class="form-control" id="providerType" onchange="updateProviderDefaults()" required>' +
        '<option value="">Select provider...</option>' +
        '<option value="openai">OpenAI (GPT)</option>' +
        '<option value="claude">Anthropic Claude</option>' +
        '</select>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="apiKey">API Key</label>' +
        '<input type="password" class="form-control" id="apiKey" placeholder="Your API key" required>' +
        '<small class="form-text text-muted">API keys are encrypted and stored securely</small>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="apiEndpoint">API Endpoint</label>' +
        '<input type="url" class="form-control" id="apiEndpoint" placeholder="https://api.openai.com/v1">' +
        '<small class="form-text text-muted">Leave empty to use default endpoint</small>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="model">Model</label>' +
        '<input type="text" class="form-control" id="model" placeholder="gpt-3.5-turbo" required>' +
        '</div>' +
        '<div class="row">' +
        '<div class="col-md-6">' +
        '<div class="form-group">' +
        '<label for="maxTokens">Max Tokens</label>' +
        '<input type="number" class="form-control" id="maxTokens" value="4000" min="100" max="32000">' +
        '</div>' +
        '</div>' +
        '<div class="col-md-6">' +
        '<div class="form-group">' +
        '<label for="temperature">Temperature</label>' +
        '<input type="number" class="form-control" id="temperature" value="0.7" min="0" max="1" step="0.1">' +
        '<small class="form-text text-muted">0 = deterministic, 1 = creative</small>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="form-group">' +
        '<div class="form-check">' +
        '<input type="checkbox" class="form-check-input" id="enabled" checked>' +
        '<label class="form-check-label" for="enabled">Enable this provider</label>' +
        '</div>' +
        '</div>' +
        '</form>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        '<button type="button" class="btn btn-primary" onclick="createAIProvider()">Create Provider</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('body').append(modalHtml);
    $('#addAIProviderModal').modal('show');

    $('#addAIProviderModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });
}

function updateProviderDefaults() {
    var providerType = $('#providerType').val();

    switch(providerType) {
        case 'openai':
            $('#apiEndpoint').val('https://api.openai.com/v1');
            $('#model').val('gpt-3.5-turbo');
            break;
        case 'claude':
            $('#apiEndpoint').val('https://api.anthropic.com/v1');
            $('#model').val('claude-3-sonnet-20240229');
            break;
    }
}

function createAIProvider() {
    var providerData = {
        name: $('#providerName').val().trim(),
        provider_type: $('#providerType').val(),
        api_key: $('#apiKey').val(),
        api_endpoint: $('#apiEndpoint').val().trim(),
        model: $('#model').val().trim(),
        max_tokens: parseInt($('#maxTokens').val()),
        temperature: parseFloat($('#temperature').val()),
        enabled: $('#enabled').is(':checked')
    };

    // Validation
    if (!providerData.name || !providerData.provider_type || !providerData.api_key || !providerData.model) {
        alert('Please fill in all required fields');
        return;
    }

    $.ajax({
        url: '/api/ai-providers',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify(providerData)
    })
    .done(function() {
        $('#addAIProviderModal').modal('hide');
        loadAIProviders();
        alert('AI provider created successfully!');
    })
    .fail(function(xhr) {
        var errorMsg = 'Failed to create AI provider';
        if (xhr.responseJSON && xhr.responseJSON.error) {
            errorMsg = xhr.responseJSON.error;
        } else if (xhr.responseText) {
            errorMsg = xhr.responseText;
        }
        alert('Error: ' + errorMsg);
    });
}

function testAIProvider(providerId) {
    // Show loading state
    var testBtn = $('button[onclick="testAIProvider(\'' + providerId + '\')"]');
    var originalHtml = testBtn.html();
    testBtn.html('<i class="fas fa-spinner fa-spin"></i> Testing...').prop('disabled', true);

    $.post('/api/ai-providers/' + providerId + '/test')
        .done(function(result) {
            if (result.status === 'success') {
                alert('Connection test successful!\n\n' + result.message);
            } else {
                alert('Connection test failed:\n\n' + result.message);
            }
            loadAIProviders(); // Refresh to show updated test status
        })
        .fail(function(xhr) {
            alert('Failed to test AI provider: ' + (xhr.responseText || 'Unknown error'));
        })
        .always(function() {
            testBtn.html(originalHtml).prop('disabled', false);
        });
}

function editAIProvider(providerId) {
    // Get provider details first
    $.get('/api/ai-providers/' + providerId)
        .done(function(provider) {
            showEditAIProviderModal(provider);
        })
        .fail(function() {
            alert('Failed to load AI provider details');
        });
}

function showEditAIProviderModal(provider) {
    var modalHtml = '<div class="modal fade" id="editAIProviderModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog modal-lg" role="document">' +
        '<div class="modal-content">' +
        '<div class="modal-header">' +
        '<h4 class="modal-title">Edit AI Provider: ' + escapeHtml(provider.name) + '</h4>' +
        '<button type="button" class="close" data-dismiss="modal">&times;</button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<form id="editAIProviderForm">' +
        '<div class="form-group">' +
        '<label for="editProviderName">Name</label>' +
        '<input type="text" class="form-control" id="editProviderName" value="' + escapeHtml(provider.name) + '" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="editProviderType">Provider Type</label>' +
        '<input type="text" class="form-control" id="editProviderType" value="' + provider.provider_type + '" readonly>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="editApiKey">API Key</label>' +
        '<input type="password" class="form-control" id="editApiKey" placeholder="Leave empty to keep current key">' +
        '<small class="form-text text-muted">Only enter a new key if you want to change it</small>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="editApiEndpoint">API Endpoint</label>' +
        '<input type="url" class="form-control" id="editApiEndpoint" value="' + (provider.api_endpoint || '') + '">' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="editModel">Model</label>' +
        '<input type="text" class="form-control" id="editModel" value="' + provider.model + '" required>' +
        '</div>' +
        '<div class="row">' +
        '<div class="col-md-6">' +
        '<div class="form-group">' +
        '<label for="editMaxTokens">Max Tokens</label>' +
        '<input type="number" class="form-control" id="editMaxTokens" value="' + provider.max_tokens + '" min="100" max="32000">' +
        '</div>' +
        '</div>' +
        '<div class="col-md-6">' +
        '<div class="form-group">' +
        '<label for="editTemperature">Temperature</label>' +
        '<input type="number" class="form-control" id="editTemperature" value="' + provider.temperature + '" min="0" max="1" step="0.1">' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="form-group">' +
        '<div class="form-check">' +
        '<input type="checkbox" class="form-check-input" id="editEnabled"' + (provider.enabled ? ' checked' : '') + '>' +
        '<label class="form-check-label" for="editEnabled">Enable this provider</label>' +
        '</div>' +
        '</div>' +
        '</form>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        '<button type="button" class="btn btn-primary" onclick="updateAIProvider(\'' + provider.id + '\')">Update Provider</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('body').append(modalHtml);
    $('#editAIProviderModal').modal('show');

    $('#editAIProviderModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });
}

function updateAIProvider(providerId) {
    var providerData = {
        name: $('#editProviderName').val().trim(),
        api_endpoint: $('#editApiEndpoint').val().trim(),
        model: $('#editModel').val().trim(),
        max_tokens: parseInt($('#editMaxTokens').val()),
        temperature: parseFloat($('#editTemperature').val()),
        enabled: $('#editEnabled').is(':checked')
    };

    // Only include API key if it was changed
    var apiKey = $('#editApiKey').val();
    if (apiKey) {
        providerData.api_key = apiKey;
    }

    $.ajax({
        url: '/api/ai-providers/' + providerId,
        method: 'PUT',
        data: JSON.stringify(providerData),
        contentType: 'application/json'
    })
    .done(function() {
        $('#editAIProviderModal').modal('hide');
        loadAIProviders();
        alert('AI provider updated successfully!');
    })
    .fail(function(xhr) {
        var errorMsg = 'Failed to update AI provider';
        if (xhr.responseJSON && xhr.responseJSON.error) {
            errorMsg = xhr.responseJSON.error;
        } else if (xhr.responseText) {
            errorMsg = xhr.responseText;
        }
        alert('Error: ' + errorMsg);
    });
}

function deleteAIProvider(providerId) {
    if (confirm('Are you sure you want to delete this AI provider? This action cannot be undone.')) {
        $.ajax({
            url: '/api/ai-providers/' + providerId,
            method: 'DELETE'
        })
        .done(function() {
            loadAIProviders();
            alert('AI provider deleted successfully');
        })
        .fail(function(xhr) {
            alert('Failed to delete AI provider: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

// General Settings Functions (from original dashboard)
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
        })
        .fail(function() {
            $('#reservedPortsThreshold').val(10000);
        });
}

function updateReservedPortsThreshold() {
    var threshold = parseInt($('#reservedPortsThreshold').val());
    if (isNaN(threshold) || threshold < 1 || threshold > 65535) {
        alert('Please enter a valid port number between 1 and 65535');
        return;
    }

    $.post('/api/user/reserved-ports-threshold', {
        threshold: threshold
    })
    .done(function() {
        alert('Reserved ports threshold updated successfully');
    })
    .fail(function(xhr) {
        alert('Failed to update threshold: ' + (xhr.responseText || 'Unknown error'));
    });
}

function loadSSOConfigs() {
    // Use the original SSO loading function from the dashboard
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
                        '<h6 class="mb-1"><i class="' + providerIcon + '"></i> ' + providerName + '</h6>' +
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

function formatUptime(seconds) {
    var days = Math.floor(seconds / 86400);
    var hours = Math.floor((seconds % 86400) / 3600);
    var minutes = Math.floor((seconds % 3600) / 60);

    if (days > 0) {
        return days + 'd ' + hours + 'h ' + minutes + 'm';
    } else if (hours > 0) {
        return hours + 'h ' + minutes + 'm';
    } else {
        return minutes + 'm';
    }
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

// Experimental features: load current settings
function loadExperimentalFeatures() {
    $.get('/api/settings/feature/ai-mock-visible')
      .done(function(res) {
          var enabled = !!(res && res.enabled);
          if (enabled) {
              $('#aimock-visible-on').prop('checked', true);
          } else {
              $('#aimock-visible-off').prop('checked', true);
          }
      })
      .fail(function() {
          // Default hidden
          $('#aimock-visible-off').prop('checked', true);
      });
}

// Experimental features: update AI Mock visibility (admin only)
function updateAIMockVisibility() {
    var enabled = $('#aimock-visible-on').is(':checked');
    $.ajax({
        url: '/api/settings/feature/ai-mock-visible',
        method: 'PUT',
        contentType: 'application/json',
        data: JSON.stringify({ enabled: enabled })
    })
    .done(function() {
        alert('AI Mock visibility updated');
        // Update runtime flag so menu/state reflects without reload
        window.dashboardApp = window.dashboardApp || {};
        window.dashboardApp.features = window.dashboardApp.features || {};
        window.dashboardApp.features.aiMockVisible = enabled;


        // If disabling and currently on ai-mock, navigate away
        if (!enabled && window.dashboardApp.currentView === 'ai-mock') {
            showDashboard();
        }
        // Optionally reload nav to add/remove menu item on next page load
    })
    .fail(function(xhr) {
        alert('Failed to update setting: ' + (xhr.responseText || 'Unknown error'));
    });
}


// Security Settings Functions
function loadSecuritySettings() {
    $.when(
        $.get('/api/settings/login-backoff'),
        $.get('/api/settings/ip-rate'),
        $.get('/api/settings/session')
    ).done(function(lbRes, ipRes, sessRes) {
        var lb = (lbRes && lbRes[0]) || {};
        var ip = (ipRes && ipRes[0]) || {};
        var sess = (sessRes && sessRes[0]) || {};
        var html = '' +
        '<div class="row">' +
        '  <div class="col-md-8">' +
        '    <div class="card mb-3">' +
        '      <div class="card-header"><h3 class="card-title">Login Backoff & Lockout</h3></div>' +
        '      <div class="card-body">' +
        '        <div class="form-row">' +
        '          <div class="form-group col-md-4">' +
        '            <label>Base Delay (ms)</label>' +
        '            <input type="number" min="0" max="60000" class="form-control" id="lb-base" value="' + (lb.base_delay_ms || 250) + '">' +
        '          </div>' +
        '          <div class="form-group col-md-4">' +
        '            <label>Max Delay (ms)</label>' +
        '            <input type="number" min="0" max="600000" class="form-control" id="lb-max" value="' + (lb.max_delay_ms || 5000) + '">' +
        '          </div>' +
        '          <div class="form-group col-md-4">' +
        '            <label>Max Exponent</label>' +
        '            <input type="number" min="0" max="10" class="form-control" id="lb-exp" value="' + (lb.max_exponent || 4) + '">' +
        '          </div>' +
        '        </div>' +
        '        <div class="form-row">' +
        '          <div class="form-group col-md-6">' +
        '            <label>Hard Lock Failures</label>' +
        '            <input type="number" min="0" max="50" class="form-control" id="lb-fails" value="' + (lb.hard_lock_failures || 8) + '">' +
        '          </div>' +
        '          <div class="form-group col-md-6">' +
        '            <label>Hard Lock Duration (minutes)</label>' +
        '            <input type="number" min="0" max="1440" class="form-control" id="lb-mins" value="' + (lb.hard_lock_minutes || 10) + '">' +
        '          </div>' +
        '        </div>' +
        '        <div class="form-group form-check">' +
        '           <input type="checkbox" class="form-check-input" id="lb-perip" ' + ((lb.per_ip_enabled ? 'checked' : '')) + '>' +
        '           <label class="form-check-label" for="lb-perip">Enable per-IP buckets (may impact users behind NAT)</label>' +
        '        </div>' +
        '        <button class="btn btn-primary" onclick="updateLoginBackoffSettings()"><i class="fas fa-save"></i> Save</button>' +
        '      </div>' +
        '    </div>' +
        '    <div class="card mb-3">' +
        '      <div class="card-header"><h3 class="card-title">IP-only Rate Limiter</h3></div>' +
        '      <div class="card-body">' +
        '        <div class="form-row">' +
        '          <div class="form-group col-md-6">' +
        '            <label>Max Requests per Minute (per IP)</label>' +
        '            <input type="number" min="0" max="100000" class="form-control" id="ip-max-per-minute" value="' + (ip.max_per_minute || 120) + '">' +
        '            <small class="form-text text-muted">0 disables the IP-only limiter</small>' +
        '          </div>' +
        '          <div class="form-group col-md-6">' +
        '            <label>Ban Duration (minutes)</label>' +
        '            <input type="number" min="0" max="10080" class="form-control" id="ip-ban-minutes" value="' + (ip.ban_minutes || 10) + '">' +
        '          </div>' +
        '        </div>' +
        '        <button class="btn btn-primary" onclick="updateIPRateSettings()"><i class="fas fa-save"></i> Save</button>' +
        '      </div>' +
        '    </div>' +
        '    <div class="card mb-3">' +
        '      <div class="card-header"><h3 class="card-title">Dashboard Session</h3></div>' +
        '      <div class="card-body">' +
        '        <div class="form-row">' +
        '          <div class="form-group col-md-6">' +
        '            <label>Session TTL (minutes)</label>' +
        '            <input type="number" min="1" max="10080" class="form-control" id="session-ttl" value="' + (sess.session_ttl_minutes || 1440) + '">' +
        '            <small class="form-text text-muted">Default 1440 (24 hours)</small>' +
        '          </div>' +
        '        </div>' +
        '        <button class="btn btn-primary" onclick="updateSessionSettings()"><i class="fas fa-save"></i> Save</button>' +
        '      </div>' +
        '    </div>' +
        '    <div class="card mb-3">' +
        '      <div class="card-header d-flex justify-content-between align-items-center">' +
        '        <h3 class="card-title mb-0">Recent Security Events</h3>' +
        '        <button class="btn btn-sm btn-outline-secondary" onclick="loadSecurityEvents()"><i class="fas fa-sync"></i> Refresh</button>' +
        '      </div>' +
        '      <div class="card-body">' +
        '        <div id="security-events-list" class="small text-muted">Loading...</div>' +
        '      </div>' +
        '    </div>' +
        '    <div class="card">' +
        '      <div class="card-header"><h3 class="card-title">Security Event Webhooks</h3></div>' +
        '      <div class="card-body">' +
        '        <div class="form-row">' +
        '          <div class="form-group col-md-5">' +
        '            <label>Webhook URL</label>' +
        '            <input type="url" class="form-control" id="wh-url" placeholder="https://...">' +
        '          </div>' +
        '          <div class="form-group col-md-3">' +
        '            <label>Payload Type</label>' +
        '            <select class="form-control" id="wh-type">' +
        '              <option value="slack">Slack (text JSON)</option>' +
        '              <option value="json">Raw JSON</option>' +
        '            </select>' +
        '          </div>' +
        '          <div class="form-group col-md-2">' +
        '            <label>Enabled</label>' +
        '            <div><input type="checkbox" id="wh-enabled" checked></div>' +
        '          </div>' +
        '          <div class="form-group col-md-2 d-flex align-items-end">' +
        '            <button class="btn btn-primary btn-block" onclick="addSecurityWebhook()"><i class="fas fa-plus"></i> Add</button>' +
        '          </div>' +
        '        </div>' +
        '        <div class="form-group">' +
        '          <label>Description</label>' +
        '          <input type="text" id="wh-desc" class="form-control" placeholder="Optional note">' +
        '        </div>' +
        '        <div id="security-webhooks-list" class="mt-3">Loading...</div>' +
        '      </div>' +
        '    </div>' +
        '  </div>' +
        '</div>';
        $('#security-settings-content').html(html);
        loadSecurityEvents();
        loadSecurityWebhooks();
    }).fail(function() {
        $('#security-settings-content').html('<div class="alert alert-danger">Failed to load security settings</div>');
    });
}

function updateLoginBackoffSettings() {
  var payload = {
    base_delay_ms: parseInt($('#lb-base').val(), 10) || 0,
    max_delay_ms: parseInt($('#lb-max').val(), 10) || 0,
    max_exponent: parseInt($('#lb-exp').val(), 10) || 0,
    hard_lock_failures: parseInt($('#lb-fails').val(), 10) || 0,
    hard_lock_minutes: parseInt($('#lb-mins').val(), 10) || 0,
    per_ip_enabled: $('#lb-perip').is(':checked')
  };
  $.ajax({
    url: '/api/settings/login-backoff',
    method: 'PUT',
    contentType: 'application/json',
    data: JSON.stringify(payload)
  })
  .done(function(){ alert('Login backoff settings updated'); })
  .fail(function(xhr){ alert('Failed to update settings: ' + (xhr.responseText || 'Unknown error')); });
}

function loadSecurityEvents() {
  $.get('/api/security/events?limit=20')
    .done(function(items){
      if (!items || items.length === 0) {
        $('#security-events-list').html('<div class="text-muted">No recent events</div>');
        return;
      }
      var html = '<ul class="list-group list-group-flush">';
      items.forEach(function(ev){
        var when = new Date(ev.at).toLocaleString();
        var sev = (ev.severity || 'info').toUpperCase();
        html += '<li class="list-group-item p-2">' +
          '<span class="badge badge-' + (sev === 'WARN' ? 'warning' : 'secondary') + ' mr-2">' + sev + '</span>' +
          '<strong>' + ev.type + '</strong>: ' + escapeHtml(ev.message || '') +
          (ev.username ? ' <span class="text-muted">user=' + escapeHtml(ev.username) + '</span>' : '') +
          (ev.ip ? ' <span class="text-muted">ip=' + escapeHtml(ev.ip) + '</span>' : '') +
          ' <span class="text-muted float-right">' + when + '</span>' +
          '</li>';
      });
      html += '</ul>';
      $('#security-events-list').html(html);
    })
    .fail(function(){
      $('#security-events-list').html('<div class="text-danger">Failed to load events</div>');
    });
}

function loadSecurityWebhooks() {
  $.get('/api/security/webhooks')
    .done(function(rows){
      var html = '';
      if (!rows || rows.length === 0) {
        html = '<div class="text-muted">No webhooks configured</div>';
      } else {
        html = '<table class="table table-sm"><thead><tr>' +
          '<th>URL</th><th>Type</th><th>Enabled</th><th>Description</th><th></th></tr></thead><tbody>';
        rows.forEach(function(w){
          html += '<tr>' +
            '<td class="text-truncate" style="max-width:260px" title="' + escapeHtml(w.url) + '">' + escapeHtml(w.url) + '</td>' +
            '<td>' + (w.type || '') + '</td>' +
            '<td>' + (w.enabled ? '<span class="badge badge-success">Yes</span>' : '<span class="badge badge-secondary">No</span>') + '</td>' +
            '<td class="text-truncate" style="max-width:200px" title="' + escapeHtml(w.description || '') + '">' + escapeHtml(w.description || '') + '</td>' +
            '<td class="text-right">' +
              '<button class="btn btn-sm btn-outline-secondary mr-2" onclick="toggleSecurityWebhook(' + w.id + ',' + (!w.enabled) + ')">' + (w.enabled ? 'Disable' : 'Enable') + '</button>' +
              '<button class="btn btn-sm btn-outline-info mr-2" onclick="testSecurityWebhook(' + w.id + ')"><i class="fas fa-paper-plane"></i> Test</button>' +
              '<button class="btn btn-sm btn-outline-danger" onclick="deleteSecurityWebhook(' + w.id + ')"><i class="fas fa-trash"></i></button>' +
            '</td>' +
          '</tr>';
        });
        html += '</tbody></table>';
      }
      $('#security-webhooks-list').html(html);
    })
    .fail(function(){
      $('#security-webhooks-list').html('<div class="text-danger">Failed to load webhooks</div>');
    });
}

function addSecurityWebhook() {
  var data = {
    url: $('#wh-url').val().trim(),
    type: $('#wh-type').val(),
    enabled: $('#wh-enabled').is(':checked'),
    description: $('#wh-desc').val().trim()
  };
  if (!data.url || !data.type) { alert('URL and type are required'); return; }
  $.ajax({ url: '/api/security/webhooks', method: 'POST', contentType: 'application/json', data: JSON.stringify(data) })
    .done(function(){
      $('#wh-url').val(''); $('#wh-desc').val(''); $('#wh-enabled').prop('checked', true); $('#wh-type').val('slack');
      loadSecurityWebhooks();
    })
    .fail(function(xhr){ alert('Failed to add webhook: ' + (xhr.responseText || 'Unknown error')); });
}

function toggleSecurityWebhook(id, enabled) {
  // Minimal update: send enabled flag; server requires type/url fields, so we send placeholders which are ignored if empty
  $.ajax({ url: '/api/security/webhooks/' + id, method: 'PUT', contentType: 'application/json', data: JSON.stringify({ enabled: !!enabled, type: 'json', url: '', description: '' }) })
    .done(function(){ loadSecurityWebhooks(); })
    .fail(function(xhr){ alert('Failed to update webhook: ' + (xhr.responseText || 'Unknown error')); });
}

function deleteSecurityWebhook(id) {
  if (!confirm('Delete this webhook?')) return;
  $.ajax({ url: '/api/security/webhooks/' + id, method: 'DELETE' })
    .done(function(){ loadSecurityWebhooks(); })
    .fail(function(xhr){ alert('Failed to delete webhook: ' + (xhr.responseText || 'Unknown error')); });
}


function updateSessionSettings() {
  var ttl = parseInt($('#session-ttl').val(), 10) || 0;
  if (ttl < 1 || ttl > 10080) { alert('TTL must be between 1 and 10080 minutes'); return; }
  $.ajax({
    url: '/api/settings/session',
    method: 'PUT',
    contentType: 'application/json',
    data: JSON.stringify({ session_ttl_minutes: ttl })
  })
  .done(function(){ alert('Session TTL updated'); })
  .fail(function(xhr){ alert('Failed to update session TTL: ' + (xhr.responseText || 'Unknown error')); });
}

function updateIPRateSettings() {
  var payload = {
    max_per_minute: parseInt($('#ip-max-per-minute').val(), 10) || 0,
    ban_minutes: parseInt($('#ip-ban-minutes').val(), 10) || 0
  };
  $.ajax({
    url: '/api/settings/ip-rate',
    method: 'PUT',
    contentType: 'application/json',
    data: JSON.stringify(payload)
  })
  .done(function(){ alert('IP rate limiter settings updated'); })
  .fail(function(xhr){ alert('Failed to update IP rate settings: ' + (xhr.responseText || 'Unknown error')); });
}


function testSecurityWebhook(id) {
  $.ajax({ url: '/api/security/webhooks/' + id + '/test', method: 'POST' })
    .done(function(){ alert('Test message sent'); })
    .fail(function(xhr){ alert('Failed to send test: ' + (xhr.responseText || 'Unknown error')); });
}
