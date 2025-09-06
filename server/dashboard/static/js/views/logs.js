// Logs View Module
// Handles server logs functionality

console.log('Logs view module loaded');

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
        '<div id="logs-container" style="max-height: 600px; overflow-y: auto; background: #f8f9fa; padding: 15px; border-radius: 5px;">' +
        '<div class="text-center text-muted">Loading logs...</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';
    $('#main-content').html(content);
    loadLogsData();

    // Auto-refresh logs every 10 seconds (less aggressive)
    if (window.dashboardApp.logsRefreshInterval) {
        clearInterval(window.dashboardApp.logsRefreshInterval);
    }
    window.dashboardApp.logsRefreshInterval = setInterval(function() {
        loadLogsDataSilently();
    }, 10000);
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
                response.logs.forEach(function(log) {
                    var levelClass = '';
                    var levelIcon = '';
                    switch(log.level.toLowerCase()) {
                        case 'error':
                            levelClass = 'text-danger';
                            levelIcon = 'fas fa-exclamation-circle';
                            break;
                        case 'warning':
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
                            levelClass = 'text-secondary';
                            levelIcon = 'fas fa-circle';
                    }
                    
                    var timestamp = new Date(log.timestamp).toLocaleString();
                    logsHtml += '<div class="log-entry mb-2 p-2 border-left border-' + 
                        (log.level.toLowerCase() === 'error' ? 'danger' : 
                         log.level.toLowerCase() === 'warning' ? 'warning' : 'info') + '">' +
                        '<div class="d-flex justify-content-between align-items-start">' +
                        '<div class="flex-grow-1">' +
                        '<span class="' + levelClass + ' font-weight-bold">' +
                        '<i class="' + levelIcon + ' mr-1"></i>' +
                        log.level.toUpperCase() + '</span> ' +
                        '<small class="text-muted">' + timestamp + '</small>' +
                        '<div class="mt-1">' + escapeHtml(log.message) + '</div>' +
                        '</div>' +
                        '</div>' +
                        '</div>';
                });
            } else {
                logsHtml = '<div class="text-center text-muted">No logs found</div>';
            }
            $('#logs-container').html(logsHtml);
            
            // Auto-scroll to bottom for new logs
            var container = $('#logs-container')[0];
            container.scrollTop = container.scrollHeight;
        })
        .fail(function() {
            $('#logs-container').html('<div class="text-center text-danger">Failed to load logs</div>');
        });
}

function loadLogsDataSilently() {
    // Silent refresh that only updates if there are changes
    var limit = $('#logLimit').val() || 100;
    var level = $('#logLevel').val() || '';

    var url = '/api/logs?limit=' + limit;
    if (level) {
        url += '&level=' + level;
    }

    $.get(url)
        .done(function(response) {
            if (response && response.logs && response.logs.length > 0) {
                // Check if we have new logs by comparing with current content
                var currentLogCount = $('#logs-container .log-entry').length;
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
                '<h4 class="modal-title">Log Files</h4>' +
                '<button type="button" class="close" data-dismiss="modal">&times;</button>' +
                '</div>' +
                '<div class="modal-body">' +
                '<div class="table-responsive">' +
                '<table class="table table-striped">' +
                '<thead><tr><th>File Name</th><th>Size</th><th>Modified</th><th>Actions</th></tr></thead>' +
                '<tbody>';

            if (response && response.files && response.files.length > 0) {
                response.files.forEach(function(file) {
                    modalHtml += '<tr>' +
                        '<td><code>' + file.name + '</code></td>' +
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

            modalHtml += '</tbody></table></div></div>' +
                '<div class="modal-footer">' +
                '<button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>' +
                '</div>' +
                '</div></div></div>';

            $('body').append(modalHtml);
            $('#logFilesModal').modal('show');

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
