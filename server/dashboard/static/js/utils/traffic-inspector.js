// Traffic Inspector Utility
// Handles traffic inspection modal for tunnels and listeners

console.log('Traffic inspector utility loaded');

// Traffic inspection function with Live SSE, Recent, filters, and pretty/raw toggle
// Works for both tunnels and listeners
function showTrafficPayloads(entityId, entityType) {
    entityType = entityType || 'tunnel'; // default to tunnel for backward compatibility
    var modalHtml = '<div class="modal fade" id="payloadsModal" tabindex="-1" style="z-index: 1060;">' +
        '<div class="modal-dialog modal-xl" role="document">' +
        '<div class="modal-content">' +
        '<div class="modal-header">' +
        '<h4 class="modal-title">Traffic Inspector - ' + entityType.charAt(0).toUpperCase() + entityType.slice(1) + ' ' + entityId.substring(0, 8) + '...</h4>' +
        '<button type="button" class="close" data-dismiss="modal">&times;</button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<div class="row mb-3">' +
        '<div class="col-md-3">' +
        '<div class="nav flex-column nav-pills" role="tablist">' +
        '<a class="nav-link active" data-toggle="pill" href="#live-tab" role="tab">Live Traffic</a>' +
        '<a class="nav-link" data-toggle="pill" href="#recent-tab" role="tab">Recent (Last 50)</a>' +
        '</div>' +
        '</div>' +
        '<div class="col-md-9">' +
        '<div class="tab-content">' +
        '<div class="tab-pane fade show active" id="live-tab" role="tabpanel">' +
        '<div class="d-flex justify-content-between align-items-center mb-2">' +
        '<div>' +
        '<button id="startLiveBtn" class="btn btn-success btn-sm" onclick="startLiveTraffic(\'' + entityId + '\', \'' + entityType + '\')">' +
        '<i class="fas fa-play"></i> Start Live</button>' +
        '<button id="stopLiveBtn" class="btn btn-danger btn-sm ml-2" onclick="stopLiveTraffic()" style="display: none;">' +
        '<i class="fas fa-stop"></i> Stop Live</button>' +
        '<span id="liveStatus" class="ml-3 text-muted">Click Start Live to begin</span>' +
        '</div>' +
        '<div>' +
        '<button class="btn btn-outline-secondary btn-sm" onclick="clearLiveTraffic()">' +
        '<i class="fas fa-trash"></i> Clear</button>' +
        '</div>' +
        '</div>' +
        '<div id="live-traffic" style="height: 400px; overflow-y: auto; border: 1px solid #ddd; padding: 10px; background: #f8f9fa; font-family: monospace; font-size: 0.85em;">' +
        '<div class="text-muted">Live traffic will appear here...</div>' +
        '</div>' +
        '</div>' +
        '<div class="tab-pane fade" id="recent-tab" role="tabpanel">' +
        '<div class="d-flex justify-content-between align-items-center mb-2">' +
        '<div>' +
        '<label for="filterType" class="mr-2">Filter:</label>' +
        '<select id="filterType" class="form-control form-control-sm d-inline-block" style="width: auto;" onchange="loadRecentTraffic(\'' + entityId + '\', \'' + entityType + '\')">' +
        '<option value="">All</option>' +
        '<option value="request">Requests</option>' +
        '<option value="response">Responses</option>' +
        '</select>' +
        '</div>' +
        '<div>' +
        '<button class="btn btn-outline-primary btn-sm mr-2" onclick="loadRecentTraffic(\'' + entityId + '\', \'' + entityType + '\')">' +
        '<i class="fas fa-sync"></i> Refresh</button>' +
        '<div class="btn-group btn-group-sm" role="group">' +
        '<button id="prettyBtn" class="btn btn-outline-secondary active" onclick="toggleView(\'pretty\')">' +
        '<i class="fas fa-code"></i> Pretty</button>' +
        '<button id="rawBtn" class="btn btn-outline-secondary" onclick="toggleView(\'raw\')">' +
        '<i class="fas fa-file-alt"></i> Raw</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div id="recent-traffic" style="height: 400px; overflow-y: auto; border: 1px solid #ddd; padding: 10px; background: #f8f9fa;">' +
        '<div class="text-muted">Loading recent traffic...</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('body').append(modalHtml);
    $('#payloadsModal').modal('show');
    $('#payloadsModal').data('entityId', entityId).data('entityType', entityType);

    // Load recent traffic by default
    loadRecentTraffic(entityId, entityType);

    // Clean up when modal is closed
    $('#payloadsModal').on('hidden.bs.modal', function() {
        stopLiveTraffic();
        $(this).remove();
    });
}

var liveEventSource = null;
var currentView = 'pretty';

function startLiveTraffic(entityId, entityType) {
    if (liveEventSource) {
        stopLiveTraffic();
    }

    $('#startLiveBtn').hide();
    $('#stopLiveBtn').show();
    $('#liveStatus').html('<span class="text-success"><i class="fas fa-circle"></i> Live - Connected</span>');

    // Clear existing content
    $('#live-traffic').html('');

    // Start SSE connection (capture service)
    function normPlural(type){ if(type==='tunnel') return 'tunnels'; if(type==='listener') return 'listeners'; if((type||'').indexOf('multicast')===0) return 'multicast'; return type; }
    var plural = normPlural(entityType);
    liveEventSource = new EventSource('/api/capture/' + plural + '/' + encodeURIComponent(entityId) + '/stream');
    
    liveEventSource.onmessage = function(event) {
        try {
            var data = JSON.parse(event.data);
            appendLiveTraffic(data);
        } catch (e) {
            console.error('Failed to parse live traffic data:', e);
        }
    };

    liveEventSource.onerror = function(event) {
        $('#liveStatus').html('<span class="text-danger"><i class="fas fa-exclamation-circle"></i> Connection Error</span>');
        setTimeout(function() {
            if (liveEventSource && liveEventSource.readyState === EventSource.CLOSED) {
                stopLiveTraffic();
            }
        }, 3000);
    };
}

function stopLiveTraffic() {
    if (liveEventSource) {
        liveEventSource.close();
        liveEventSource = null;
    }
    $('#startLiveBtn').show();
    $('#stopLiveBtn').hide();
    $('#liveStatus').html('<span class="text-muted">Stopped</span>');
}

function clearLiveTraffic() {
    $('#live-traffic').html('<div class="text-muted">Live traffic will appear here...</div>');
}

function appendLiveTraffic(data) {
    var timestamp = new Date().toLocaleTimeString();
    var typeClass = data.type === 'request' ? 'text-primary' : 'text-success';
    var typeIcon = data.type === 'request' ? 'fas fa-arrow-right' : 'fas fa-arrow-left';
    
    var html = '<div class="border-bottom pb-2 mb-2">' +
        '<div class="d-flex justify-content-between align-items-center">' +
        '<span class="' + typeClass + ' font-weight-bold">' +
        '<i class="' + typeIcon + '"></i> ' + data.type.toUpperCase() + '</span>' +
        '<small class="text-muted">' + timestamp + '</small>' +
        '</div>';
    
    if (data.method && data.url) {
        html += '<div><strong>' + data.method + '</strong> ' + data.url + '</div>';
    }
    
    if (data.headers) {
        html += '<div class="mt-1"><small class="text-muted">Headers:</small><br>';
        Object.keys(data.headers).forEach(function(key) {
            html += '<small>' + key + ': ' + escapeHtml(data.headers[key]) + '</small><br>';
        });
        html += '</div>';
    }
    
    if (data.body) {
        html += '<div class="mt-1"><small class="text-muted">Body:</small><br>';
        html += '<pre class="traffic-payload">' + escapeHtml(data.body) + '</pre>';
        html += '</div>';
    }
    
    html += '</div>';
    
    $('#live-traffic').append(html);
    
    // Auto-scroll to bottom
    var container = $('#live-traffic')[0];
    container.scrollTop = container.scrollHeight;
}

function loadRecentTraffic(entityId, entityType) {
    var filter = $('#filterType').val();
    function normPlural(type){ if(type==='tunnel') return 'tunnels'; if(type==='listener') return 'listeners'; if((type||'').indexOf('multicast')===0) return 'multicast'; return type; }
    var plural = normPlural(entityType);
    var url = '/api/capture/' + plural + '/' + encodeURIComponent(entityId) + '/recent';
    if (filter) {
        url += '?type=' + filter;
    }

    $.get(url)
        .done(function(data) {
            displayTrafficData(data);
        })
        .fail(function() {
            $('#recent-traffic').html('<div class="text-danger">Failed to load traffic data</div>');
        });
}

function displayTrafficData(data) {
    var html = '';
    if (data && data.length > 0) {
        data.forEach(function(item) {
            var timestamp = new Date(item.timestamp).toLocaleString();
            var typeClass = item.type === 'request' ? 'text-primary' : 'text-success';
            var typeIcon = item.type === 'request' ? 'fas fa-arrow-right' : 'fas fa-arrow-left';
            
            html += '<div class="border-bottom pb-2 mb-2">' +
                '<div class="d-flex justify-content-between align-items-center">' +
                '<span class="' + typeClass + ' font-weight-bold">' +
                '<i class="' + typeIcon + '"></i> ' + item.type.toUpperCase() + '</span>' +
                '<small class="text-muted">' + timestamp + '</small>' +
                '</div>';
            
            if (item.method && item.url) {
                html += '<div><strong>' + item.method + '</strong> ' + item.url + '</div>';
            }
            
            if (currentView === 'pretty') {
                html += formatPrettyTraffic(item);
            } else {
                html += formatRawTraffic(item);
            }
            
            html += '</div>';
        });
    } else {
        html = '<div class="text-muted">No traffic data available</div>';
    }
    $('#recent-traffic').html(html);
}

function formatPrettyTraffic(item) {
    var html = '';
    
    if (item.headers) {
        html += '<div class="mt-1"><small class="text-muted">Headers:</small><br>';
        Object.keys(item.headers).forEach(function(key) {
            html += '<small>' + key + ': ' + escapeHtml(item.headers[key]) + '</small><br>';
        });
        html += '</div>';
    }
    
    if (item.body) {
        html += '<div class="mt-1"><small class="text-muted">Body:</small><br>';
        try {
            var parsed = JSON.parse(item.body);
            html += '<pre class="traffic-payload">' + JSON.stringify(parsed, null, 2) + '</pre>';
        } catch (e) {
            html += '<pre class="traffic-payload">' + escapeHtml(item.body) + '</pre>';
        }
        html += '</div>';
    }
    
    return html;
}

function formatRawTraffic(item) {
    var html = '<div class="mt-1"><pre class="traffic-payload">';
    
    if (item.method && item.url) {
        html += item.method + ' ' + item.url + '\n';
    }
    
    if (item.headers) {
        Object.keys(item.headers).forEach(function(key) {
            html += key + ': ' + item.headers[key] + '\n';
        });
        html += '\n';
    }
    
    if (item.body) {
        html += item.body;
    }
    
    html += '</pre></div>';
    return html;
}

function toggleView(view) {
    currentView = view;
    $('#prettyBtn, #rawBtn').removeClass('active');
    $('#' + view + 'Btn').addClass('active');
    
    // Reload current data with new view
    var activeTab = $('.tab-pane.active').attr('id');
    if (activeTab === 'recent-tab') {
        var entityId = $('#payloadsModal').data('entityId');
        var entityType = $('#payloadsModal').data('entityType');
        if (entityId && entityType) {
            loadRecentTraffic(entityId, entityType);
        }
    }
}
