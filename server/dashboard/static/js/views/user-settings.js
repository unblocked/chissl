// User Settings View Module
// Handles user profile and API token management

console.log('User Settings view module loaded');

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
        '<input type="text" class="form-control" id="displayName" placeholder="Your display name">' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="email">Email</label>' +
        '<input type="email" class="form-control" id="email" placeholder="your.email@example.com">' +
        '</div>' +
        '<button type="submit" class="btn btn-primary">Update Profile</button>' +
        '</form>' +
        '</div>' +
        '</div>' +
        '</div>' +
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
        '<div class="text-center text-muted">Loading tokens...</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="row mt-3">' +
        '<div class="col-12">' +
        '<div id="user-port-info"></div>' +
        '</div>' +
        '</div>';

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

// User profile management functions
function loadUserProfile() {
    if (window.dashboardApp.currentUser) {
        $('#displayName').val(window.dashboardApp.currentUser.display_name || '');
        $('#email').val(window.dashboardApp.currentUser.email || '');

        // Disable profile editing for CLI admin users
        if (window.dashboardApp.currentUser.username === 'admin' && !window.dashboardApp.currentUser.email) {
            $('#displayName, #email').prop('disabled', true);
            $('#profileForm button[type="submit"]').prop('disabled', true).text('Profile editing disabled for CLI admin');
        }

        // Disable email editing for SSO users
        if (window.dashboardApp.currentUser.sso_enabled || window.dashboardApp.currentUser.auth_method === 'auth0' || window.dashboardApp.currentUser.auth_method === 'sso') {
            $('#email').prop('disabled', true);
            $('#email').after('<small class="form-text text-muted">Email cannot be changed for SSO users</small>');
        }
    } else {
        // Load user info if not available
        loadUserInfo().then(function() {
            loadUserProfile();
        });
    }
}

function loadUserTokens() {
    $.get('/api/user/tokens')
    .done(function(data) {
        var tokensList = '';
        if (data && data.length > 0) {
            data.forEach(function(token) {
                var createdDate = new Date(token.created_at).toLocaleDateString();
                var lastUsed = token.last_used_at ? new Date(token.last_used_at).toLocaleDateString() : 'Never';
                
                tokensList += '<div class="card mb-2">' +
                    '<div class="card-body py-2">' +
                    '<div class="d-flex justify-content-between align-items-center">' +
                    '<div>' +
                    '<strong>' + escapeHtml(token.name) + '</strong><br>' +
                    '<small class="text-muted">Created: ' + createdDate + ' | Last used: ' + lastUsed + '</small>' +
                    '</div>' +
                    '<button class="btn btn-sm btn-danger" onclick="revokeToken(\'' + token.id + '\')">' +
                    '<i class="fas fa-trash"></i> Revoke' +
                    '</button>' +
                    '</div>' +
                    '</div>' +
                    '</div>';
            });
        } else {
            tokensList = '<div class="text-center text-muted">No API tokens generated</div>';
        }
        $('#tokens-list').html(tokensList);
    })
    .fail(function() {
        $('#tokens-list').html('<div class="text-center text-danger">Failed to load tokens</div>');
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
        '<label for="tokenExpiry">Expiry (days)</label>' +
        '<input type="number" class="form-control" id="tokenExpiry" placeholder="Leave empty for no expiry" min="1" max="365">' +
        '<small class="form-text text-muted">Optional: Token will expire after this many days</small>' +
        '</div>' +
        '<div class="alert alert-info">' +
        '<i class="fas fa-info-circle"></i> ' +
        'Use this token as the password when connecting with chiSSL client. Keep it secure!' +
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

    $.post('/api/user/tokens', tokenData)
    .done(function(data) {
        $('#generateTokenModal').modal('hide');

        // Show the generated token
        var tokenModalHtml = '<div class="modal fade" id="showTokenModal" tabindex="-1" role="dialog">' +
            '<div class="modal-dialog" role="document">' +
            '<div class="modal-content">' +
            '<div class="modal-header bg-success text-white">' +
            '<h5 class="modal-title">API Token Generated</h5>' +
            '<button type="button" class="close text-white" data-dismiss="modal">&times;</button>' +
            '</div>' +
            '<div class="modal-body">' +
            '<div class="alert alert-warning">' +
            '<i class="fas fa-exclamation-triangle"></i> ' +
            '<strong>Important:</strong> This token will only be shown once. Copy it now!' +
            '</div>' +
            '<div class="form-group">' +
            '<label>Your new API token:</label>' +
            '<div class="input-group">' +
            '<input type="text" class="form-control" id="newTokenValue" value="' + data.token + '" readonly>' +
            '<div class="input-group-append">' +
            '<button class="btn btn-outline-secondary" type="button" onclick="copyToClipboard(\'' + data.token + '\')">' +
            '<i class="fas fa-copy"></i> Copy' +
            '</button>' +
            '</div>' +
            '</div>' +
            '</div>' +
            '<div class="alert alert-info">' +
            '<i class="fas fa-info-circle"></i> ' +
            'Use this token as the password when connecting with chiSSL client.' +
            '</div>' +
            '</div>' +
            '<div class="modal-footer">' +
            '<button type="button" class="btn btn-primary" data-dismiss="modal">Done</button>' +
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

    $.ajax({
        url: '/api/user/profile',
        method: 'PUT',
        data: JSON.stringify(profileData),
        contentType: 'application/json'
    })
    .done(function() {
        alert('Profile updated successfully');
        // Refresh user info
        loadUserInfo();
    })
    .fail(function(xhr) {
        var errorMsg = 'Failed to update profile';
        if (xhr.responseJSON && xhr.responseJSON.error) {
            errorMsg = xhr.responseJSON.error;
        }
        alert('Error: ' + errorMsg);
    });
}

function loadUserPortInfo() {
    // Load user's port reservations and general port info
    $.get('/api/user/port-reservations')
        .done(function(data) {
            var portInfoHtml = '';
            if (data && data.length > 0) {
                portInfoHtml = '<div class="card">' +
                    '<div class="card-header">' +
                    '<h5 class="card-title"><i class="fas fa-network-wired"></i> Your Port Reservations</h5>' +
                    '</div>' +
                    '<div class="card-body">' +
                    '<div class="row">';
                
                data.forEach(function(reservation, index) {
                    if (index > 0 && index % 2 === 0) {
                        portInfoHtml += '</div><div class="row">';
                    }
                    portInfoHtml += '<div class="col-md-6 mb-3">' +
                        '<div class="border p-3 rounded">' +
                        '<h6 class="text-primary">Ports ' + reservation.start_port + '-' + reservation.end_port + '</h6>';
                    if (reservation.description) {
                        portInfoHtml += '<p class="text-muted mb-0">' + escapeHtml(reservation.description) + '</p>';
                    }
                    portInfoHtml += '</div></div>';
                });
                
                portInfoHtml += '</div>' +
                    '<div class="alert alert-success mt-3">' +
                    '<i class="fas fa-info-circle"></i> ' +
                    'You can create tunnels and listeners on these reserved ports in addition to unreserved ports.' +
                    '</div>' +
                    '</div></div>';
            } else {
                // Show general port info for users without reservations
                $.get('/api/user/reserved-ports-threshold')
                    .done(function(thresholdData) {
                        var threshold = thresholdData.threshold || 10000;
                        portInfoHtml = '<div class="card">' +
                            '<div class="card-header">' +
                            '<h5 class="card-title"><i class="fas fa-network-wired"></i> Available Ports</h5>' +
                            '</div>' +
                            '<div class="card-body">' +
                            '<div class="alert alert-info">' +
                            '<h6><i class="fas fa-info-circle"></i> Port Access Information</h6>' +
                            '<p><strong>You can use ports ' + threshold + ' and above</strong></p>' +
                            '<small class="text-muted">' +
                            'Ports 0-' + (threshold-1) + ' are reserved for admin assignment. ' +
                            'Contact your administrator if you need access to reserved ports.' +
                            '</small>' +
                            '</div>' +
                            '</div></div>';
                        $('#user-port-info').html(portInfoHtml);
                    })
                    .fail(function() {
                        portInfoHtml = '<div class="card">' +
                            '<div class="card-header">' +
                            '<h5 class="card-title"><i class="fas fa-network-wired"></i> Available Ports</h5>' +
                            '</div>' +
                            '<div class="card-body">' +
                            '<div class="alert alert-info">' +
                            '<h6><i class="fas fa-info-circle"></i> Port Access Information</h6>' +
                            '<p><strong>You can use ports 10000 and above</strong></p>' +
                            '<small class="text-muted">' +
                            'Ports 0-9999 are reserved for admin assignment. ' +
                            'Contact your administrator if you need access to reserved ports.' +
                            '</small>' +
                            '</div>' +
                            '</div></div>';
                        $('#user-port-info').html(portInfoHtml);
                    });
                return;
            }
            $('#user-port-info').html(portInfoHtml);
        })
        .fail(function() {
            // Show fallback port info
            var portInfoHtml = '<div class="card">' +
                '<div class="card-header">' +
                '<h5 class="card-title"><i class="fas fa-network-wired"></i> Available Ports</h5>' +
                '</div>' +
                '<div class="card-body">' +
                '<div class="alert alert-info">' +
                '<h6><i class="fas fa-info-circle"></i> Port Access Information</h6>' +
                '<p><strong>You can use ports 10000 and above</strong></p>' +
                '<small class="text-muted">' +
                'Contact your administrator for specific port assignments.' +
                '</small>' +
                '</div>' +
                '</div></div>';
            $('#user-port-info').html(portInfoHtml);
        });
}
