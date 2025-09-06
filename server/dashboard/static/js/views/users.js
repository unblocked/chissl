// Users View Module
// Handles user management functionality

console.log('Users view module loaded');

function loadUsersView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title">Users</h3>' +
        '<div class="card-tools">' +
        '<button class="btn btn-primary btn-sm" onclick="showAddUserModal()" id="add-user-btn">' +
        '<i class="fas fa-plus"></i> Add User' +
        '</button>' +
        '<button class="btn btn-secondary btn-sm ml-2" onclick="loadUsersData()">' +
        '<i class="fas fa-sync"></i> Refresh' +
        '</button>' +
        '</div>' +
        '</div>' +
        '<div class="card-body">' +
        '<div class="table-responsive">' +
        '<table class="table table-striped">' +
        '<thead><tr><th>Username</th><th>Email</th><th>Display Name</th><th>Admin</th><th>Port Reservations</th><th>Actions</th></tr></thead>' +
        '<tbody id="users-tbody"><tr><td colspan="6" class="text-center">Loading users...</td></tr></tbody>' +
        '</table>' +
        '</div>' +
        '</div></div></div></div>';

    $('#main-content').html(content);
    loadUsersData();
}

function loadUsersData() {
    // Only load if user is admin
    if (!window.dashboardApp.isAdmin) {
        $('#users-tbody').html('<tr><td colspan="6" class="text-center text-warning">Admin access required</td></tr>');
        return;
    }

    $.get('/users')
        .done(function(data) {
            var usersHtml = '';
            if (data && data.length > 0) {
                data.forEach(function(user) {
                    var adminBadge = user.is_admin ? 
                        '<span class="badge badge-danger">Admin</span>' : 
                        '<span class="badge badge-secondary">User</span>';
                    
                    var portReservationsHtml = '';
                    if (user.port_reservations && user.port_reservations.length > 0) {
                        portReservationsHtml = '<div class="btn-group btn-group-sm">';
                        user.port_reservations.forEach(function(reservation) {
                            portReservationsHtml += '<span class="badge badge-info mr-1" title="' + 
                                (reservation.description || 'Port reservation') + '">' +
                                reservation.start_port + '-' + reservation.end_port + '</span>';
                        });
                        portReservationsHtml += '</div>';
                        
                        // Add management buttons
                        portReservationsHtml += '<div class="mt-1">' +
                            '<button class="btn btn-outline-primary btn-xs" onclick="showAddPortReservationModal(\'' + user.username + '\')" title="Add Port Reservation">' +
                            '<i class="fas fa-plus"></i></button>' +
                            '<button class="btn btn-outline-danger btn-xs ml-1" onclick="removeUserPortReservations(\'' + user.username + '\')" title="Remove All Reservations">' +
                            '<i class="fas fa-trash"></i></button>' +
                            '</div>';
                    } else {
                        portReservationsHtml = '<button class="btn btn-outline-primary btn-sm" onclick="showAddPortReservationModal(\'' + user.username + '\')" title="Add Port Reservation">' +
                            '<i class="fas fa-plus"></i> Add Ports</button>';
                    }
                    
                    usersHtml += '<tr>' +
                        '<td><strong>' + escapeHtml(user.username) + '</strong></td>' +
                        '<td>' + escapeHtml(user.email || '') + '</td>' +
                        '<td>' + escapeHtml(user.display_name || '') + '</td>' +
                        '<td>' + adminBadge + '</td>' +
                        '<td>' + portReservationsHtml + '</td>' +
                        '<td>' +
                        '<div class="btn-group btn-group-sm" role="group">' +
                        '<button class="btn btn-outline-primary edit-user-btn" data-username="' + user.username + '" title="Edit User">' +
                        '<i class="fas fa-edit"></i>' +
                        '</button>' +
                        '<button class="btn btn-outline-danger delete-user-btn" data-username="' + user.username + '" title="Delete User">' +
                        '<i class="fas fa-trash"></i>' +
                        '</button>' +
                        '</div>' +
                        '</td>' +
                        '</tr>';
                });
            } else {
                usersHtml = '<tr><td colspan="6" class="text-center text-muted">No users found</td></tr>';
            }
            $('#users-tbody').html(usersHtml);
        })
        .fail(function() {
            $('#users-tbody').html('<tr><td colspan="6" class="text-center text-danger">Failed to load users</td></tr>');
        });
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
        '<div class="form-check">' +
        '<input type="checkbox" class="form-check-input" id="newIsAdmin">' +
        '<label class="form-check-label" for="newIsAdmin">Admin User</label>' +
        '</div>' +
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

    $.post('/users', userData)
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
        '<div class="form-check">' +
        '<input type="checkbox" class="form-check-input" id="editIsAdmin">' +
        '<label class="form-check-label" for="editIsAdmin">Admin User</label>' +
        '</div>' +
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
            $('#editIsAdmin').prop('checked', user.is_admin);
        });
}

function updateUser(username) {
    var userData = {
        username: username,
        is_admin: $('#editIsAdmin').is(':checked')
    };

    var password = $('#editPassword').val();
    if (password) {
        userData.password = password;
    }

    $.ajax({
        url: '/user/' + username,
        method: 'PUT',
        data: JSON.stringify(userData),
        contentType: 'application/json'
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
