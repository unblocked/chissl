// Listeners View Module
// Handles HTTP listener management functionality

console.log('Listeners view module loaded');

function loadListenersView() {
    var content = '<div class="row">' +
        '<div class="col-12">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h3 class="card-title">HTTP Listeners</h3>' +
        '<div class="card-tools">' +
        '<button class="btn btn-primary btn-sm" onclick="showAddListenerModal()">' +
        '<i class="fas fa-plus"></i> Add Listener</button>' +
        '<button class="btn btn-secondary btn-sm ml-2" onclick="loadListenersData()">' +
        '<i class="fas fa-sync"></i> Refresh</button>' +
        '</div>' +
        '</div>' +
        '<div class="card-body">' +
        '<table class="table table-bordered table-striped" id="listeners-table">' +
        '<thead>' +
        '<tr>' +
        '<th>Name</th>' +
        '<th>Port</th>' +
        '<th>Mode</th>' +
        '<th>Target/Response</th>' +
        '<th>TLS</th>' +
        '<th>Status</th>' +
        '<th>Connections</th>' +
        '<th>Bytes Sent</th>' +
        '<th>Bytes Received</th>' +
        '<th>Actions</th>' +
        '</tr>' +
        '</thead>' +
        '<tbody id="listeners-tbody">' +
        '<tr><td colspan="10" class="text-center">Loading listeners...</td></tr>' +
        '</tbody>' +
        '</table>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';
    $('#main-content').html(content);
    loadListenersData();
}

// Data loading functions for listeners
function loadListenersData() {
    $.get('/api/listeners')
        .done(function(data) {
            var tbody = '';
            if (data && data.length > 0) {
                data.forEach(function(listener) {
                    var statusClass = listener.active ? 'badge-success' : 'badge-danger';
                    var statusText = listener.active ? 'Active' : 'Inactive';
                    var tlsBadge = listener.use_tls ? 
                        '<span class="badge badge-success">TLS</span>' : 
                        '<span class="badge badge-secondary">Plain</span>';
                    
                    var targetDisplay = '';
                    if (listener.mode === 'proxy') {
                        targetDisplay = '<small class="text-muted">→ ' + (listener.target_url || 'N/A') + '</small>';
                    } else if (listener.mode === 'static') {
                        var responsePreview = listener.response ? 
                            (listener.response.length > 50 ? 
                                listener.response.substring(0, 50) + '...' : 
                                listener.response) : 'Empty';
                        targetDisplay = '<small class="text-muted">' + escapeHtml(responsePreview) + '</small>';
                    }
                    
                    tbody += '<tr>' +
                        '<td><strong>' + escapeHtml(listener.name || 'Unnamed') + '</strong><br>' +
                        '<small class="text-muted">' + listener.id.substring(0, 8) + '...</small></td>' +
                        '<td><code>' + listener.port + '</code></td>' +
                        '<td><span class="badge badge-info">' + listener.mode + '</span></td>' +
                        '<td>' + targetDisplay + '</td>' +
                        '<td>' + tlsBadge + '</td>' +
                        '<td><span class="badge ' + statusClass + '">' + statusText + '</span></td>' +
                        '<td>' + (listener.connections || 0) + '</td>' +
                        '<td>' + formatBytes(listener.bytes_sent || 0) + '</td>' +
                        '<td>' + formatBytes(listener.bytes_recv || 0) + '</td>' +
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
                tbody = '<tr><td colspan="10" class="text-center text-muted">No listeners found</td></tr>';
            }
            $('#listeners-tbody').html(tbody);
        })
        .fail(function() {
            $('#listeners-tbody').html('<tr><td colspan="10" class="text-center text-danger">Failed to load listeners</td></tr>');
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
        '<label for="newName">Name</label>' +
        '<input type="text" class="form-control" id="newName" placeholder="My Listener">' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="newPort">Port</label>' +
        '<input type="number" class="form-control" id="newPort" placeholder="8080" min="1" max="65535" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="newMode">Mode</label>' +
        '<select class="form-control" id="newMode" onchange="toggleModeFields()">' +
        '<option value="proxy">Proxy</option>' +
        '<option value="static">Static Response</option>' +
        '</select>' +
        '</div>' +
        '<div class="form-group" id="targetUrlGroup">' +
        '<label for="newTargetUrl">Target URL</label>' +
        '<input type="url" class="form-control" id="newTargetUrl" placeholder="http://localhost:3000">' +
        '</div>' +
        '<div class="form-group" id="responseGroup" style="display: none;">' +
        '<label for="newResponse">Static Response</label>' +
        '<textarea class="form-control" id="newResponse" rows="4" placeholder="Hello World!"></textarea>' +
        '</div>' +
        '<div class="form-group">' +
        '<div class="form-check">' +
        '<input type="checkbox" class="form-check-input" id="newUseTLS">' +
        '<label class="form-check-label" for="newUseTLS">Use TLS (HTTPS)</label>' +
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

// Toggle fields based on mode selection
function toggleModeFields() {
    var mode = $('#newMode').val();
    if (mode === 'proxy') {
        $('#targetUrlGroup').show();
        $('#responseGroup').hide();
    } else {
        $('#targetUrlGroup').hide();
        $('#responseGroup').show();
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

    $.post('/api/listeners', listenerData)
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
                '<label for="editPort">Port (read-only)</label>' +
                '<input type="number" class="form-control" id="editPort" value="' + listener.port + '" readonly>' +
                '</div>' +
                '<div class="form-group">' +
                '<label for="editMode">Mode (read-only)</label>' +
                '<input type="text" class="form-control" id="editMode" value="' + listener.mode + '" readonly>' +
                '</div>' +
                (listener.mode === 'proxy' ? 
                    '<div class="form-group">' +
                    '<label for="editTargetUrl">Target URL</label>' +
                    '<input type="url" class="form-control" id="editTargetUrl" value="' + (listener.target_url || '') + '">' +
                    '</div>' : 
                    '<div class="form-group">' +
                    '<label for="editResponse">Static Response</label>' +
                    '<textarea class="form-control" id="editResponse" rows="4">' + (listener.response || '') + '</textarea>' +
                    '</div>') +
                '<div class="form-group">' +
                '<div class="form-check">' +
                '<input type="checkbox" class="form-check-input" id="editUseTLS"' + (listener.use_tls ? ' checked' : '') + '>' +
                '<label class="form-check-label" for="editUseTLS">Use TLS (HTTPS)</label>' +
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
        data: JSON.stringify(updateData),
        contentType: 'application/json'
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
