// Port Reservations Component
// Handles port reservation management modals and functionality

console.log('Port reservations component loaded');

function showAddPortReservationModal(preselectedUser) {
    var modalHtml = '<div class="modal fade" id="addPortReservationModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog" role="document">' +
        '<div class="modal-content">' +
        '<div class="modal-header">' +
        '<h4 class="modal-title">Add Port Reservation</h4>' +
        '<button type="button" class="close" data-dismiss="modal" aria-label="Close">' +
        '<span aria-hidden="true">&times;</span></button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<form id="addPortReservationForm">' +
        '<div class="form-group">' +
        '<label for="reservationUser">User</label>';
    
    if (preselectedUser) {
        modalHtml += '<input type="text" class="form-control" id="reservationUser" value="' + preselectedUser + '" readonly>';
    } else {
        modalHtml += '<select class="form-control" id="reservationUser" required>' +
            '<option value="">Select a user...</option>' +
            '</select>';
    }
    
    modalHtml += '</div>' +
        '<div class="form-group">' +
        '<label for="startPort">Start Port</label>' +
        '<input type="number" class="form-control" id="startPort" min="1" max="65535" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="endPort">End Port</label>' +
        '<input type="number" class="form-control" id="endPort" min="1" max="65535" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="reservationDescription">Description (optional)</label>' +
        '<input type="text" class="form-control" id="reservationDescription" placeholder="e.g., Development servers">' +
        '</div>' +
        '</form>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        '<button type="button" class="btn btn-primary" onclick="createPortReservation()">Create Reservation</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    // Remove any existing modal first
    $('#addPortReservationModal').remove();
    $('body').append(modalHtml);

    // Load users for dropdown if not preselected
    if (!preselectedUser) {
        $.get('/users')
            .done(function(users) {
                var options = '<option value="">Select a user...</option>';
                users.forEach(function(user) {
                    options += '<option value="' + user.username + '">' + user.username + '</option>';
                });
                $('#reservationUser').html(options);
            })
            .fail(function() {
                $('#reservationUser').html('<option value="">Failed to load users</option>');
            });
    }

    // Initialize and show modal
    $('#addPortReservationModal').modal({
        backdrop: true,
        keyboard: true,
        show: true
    });

    // Clean up when modal is hidden
    $('#addPortReservationModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });

    // Validate port range
    $('#startPort, #endPort').on('input', function() {
        var startPort = parseInt($('#startPort').val());
        var endPort = parseInt($('#endPort').val());
        
        if (startPort && endPort && startPort > endPort) {
            $('#endPort').val(startPort);
        }
        
        // Warn about high port numbers
        if (startPort > 50000 || endPort > 50000) {
            if (!$('#port-warning').length) {
                $('#addPortReservationForm').append(
                    '<div id="port-warning" class="alert alert-warning mt-2">' +
                    '<i class="fas fa-exclamation-triangle"></i> ' +
                    'High port numbers (>50000) may not be suitable for all applications.' +
                    '</div>'
                );
            }
        } else {
            $('#port-warning').remove();
        }
    });
}

function createPortReservation() {
    // Only allow if user is admin
    if (!window.dashboardApp.isAdmin) {
        alert('Admin access required');
        return;
    }

    var reservationData = {
        username: $('#reservationUser').val(),
        start_port: parseInt($('#startPort').val()),
        end_port: parseInt($('#endPort').val()),
        description: $('#reservationDescription').val().trim()
    };

    // Validation
    if (!reservationData.username) {
        alert('Please select a user');
        return;
    }
    
    if (!reservationData.start_port || !reservationData.end_port) {
        alert('Please enter valid port numbers');
        return;
    }
    
    if (reservationData.start_port > reservationData.end_port) {
        alert('Start port must be less than or equal to end port');
        return;
    }
    
    if (reservationData.start_port < 1 || reservationData.end_port > 65535) {
        alert('Port numbers must be between 1 and 65535');
        return;
    }

    $.post('/api/port-reservations', reservationData)
    .done(function() {
        $('#addPortReservationModal').modal('hide');
        
        // Refresh users data to show updated port reservations
        if (window.dashboardApp.currentView === 'users') {
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
    if (!window.dashboardApp.isAdmin) {
        alert('Admin access required');
        return;
    }

    if (confirm('Are you sure you want to delete this port reservation?')) {
        $.ajax({
            url: '/api/port-reservations/' + reservationId,
            method: 'DELETE'
        })
        .done(function() {
            // Refresh users data to show updated port reservations
            if (window.dashboardApp.currentView === 'users') {
                loadUsersData();
            }
            alert('Port reservation deleted successfully');
        })
        .fail(function(xhr) {
            alert('Failed to delete port reservation: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

function removeUserPortReservations(username) {
    // Only allow if user is admin
    if (!window.dashboardApp.isAdmin) {
        alert('Admin access required');
        return;
    }

    if (confirm('Are you sure you want to remove ALL port reservations for user "' + username + '"? This action cannot be undone.')) {
        // Get all reservations for this user first
        $.get('/users')
            .done(function(users) {
                var user = users.find(u => u.username === username);
                if (user && user.port_reservations && user.port_reservations.length > 0) {
                    // Delete each reservation
                    var deletePromises = user.port_reservations.map(function(reservation) {
                        return $.ajax({
                            url: '/api/port-reservations/' + reservation.id,
                            method: 'DELETE'
                        });
                    });
                    
                    Promise.all(deletePromises)
                    .then(function() {
                        // Refresh users data to show updated port reservations
                        if (window.dashboardApp.currentView === 'users') {
                            loadUsersData();
                        }
                        alert('All port reservations removed successfully for user "' + username + '"');
                    })
                    .catch(function(xhr) {
                        alert('Failed to remove some port reservations: ' + (xhr.responseText || 'Unknown error'));
                    });
                } else {
                    alert('No port reservations found for user "' + username + '"');
                }
            })
            .fail(function() {
                alert('Failed to load user data');
            });
    }
}
