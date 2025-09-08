// Component-based Auto-Refresh System
// Refreshes individual components without changing views

window.AutoRefresh = {
    intervals: {},
    
    // Start auto-refresh for a component
    start: function(componentId, refreshFunction, intervalMs) {
        // Stop existing interval if any
        this.stop(componentId);

        // Debug logging for listeners-table
        if (componentId === 'listeners-table') {
            console.log('🔥🔥🔥 AUTO-REFRESH: listeners-table auto-refresh started!');
            console.log('🔥🔥🔥 AUTO-REFRESH: Stack trace:', new Error().stack);
        }

        // Start new interval
        this.intervals[componentId] = setInterval(function() {
            try {
                if (componentId === 'listeners-table') {
                    console.log('🔥🔥🔥 AUTO-REFRESH: listeners-table interval executing');
                }
                refreshFunction();
            } catch (error) {
                console.error('Auto-refresh error for', componentId, ':', error);
            }
        }, intervalMs);

        console.log('Auto-refresh started for', componentId, 'every', intervalMs/1000, 'seconds');
    },
    
    // Stop auto-refresh for a component
    stop: function(componentId) {
        if (this.intervals[componentId]) {
            clearInterval(this.intervals[componentId]);
            delete this.intervals[componentId];
            console.log('Auto-refresh stopped for', componentId);
        }
    },
    
    // Stop all auto-refresh intervals
    stopAll: function() {
        for (var componentId in this.intervals) {
            this.stop(componentId);
        }
    },
    
    // Check if component is auto-refreshing
    isActive: function(componentId) {
        return !!this.intervals[componentId];
    }
};

// Dashboard Stats Cards Refresh Functions
window.DashboardStats = {
    refreshSystemInfo: function() {
        $.get('/api/system')
            .done(function(data) {
                $('#stats-version').text(data.version || 'Unknown');
                $('#stats-uptime').text(formatUptime(data.uptime || 0));
                $('#stats-fingerprint').text(data.fingerprint || 'Unknown');
            })
            .fail(function() {
                $('#stats-version').text('Error');
                $('#stats-uptime').text('Error');
                $('#stats-fingerprint').text('Error');
            });
    },
    
    refreshTunnelStats: function() {
        $.get('/api/stats')
            .done(function(data) {
                $('#stats-active-tunnels').text(data.active_tunnels || 0);
                $('#stats-total-tunnels').text(data.total_tunnels || 0);
                $('#stats-total-connections').text(data.total_connections || 0);
                $('#stats-data-transferred').text(formatBytes(data.data_transferred || 0));
            })
            .fail(function() {
                $('#stats-active-tunnels').text('Error');
                $('#stats-total-tunnels').text('Error');
                $('#stats-total-connections').text('Error');
                $('#stats-data-transferred').text('Error');
            });
    },
    
    refreshListenerStats: function() {
        $.get('/api/listeners')
            .done(function(data) {
                var activeListeners = 0;
                var totalListeners = data ? data.length : 0;
                
                if (data && data.length > 0) {
                    data.forEach(function(listener) {
                        if (listener.status === 'active') {
                            activeListeners++;
                        }
                    });
                }
                
                $('#stats-active-listeners').text(activeListeners);
                $('#stats-total-listeners').text(totalListeners);
            })
            .fail(function() {
                $('#stats-active-listeners').text('Error');
                $('#stats-total-listeners').text('Error');
            });
    }
};

// Table Refresh Functions
window.TableRefresh = {
    refreshTunnelsTable: function() {
        if (typeof loadTunnelsData === 'function') {
            loadTunnelsData();
        }
    },
    
    refreshListenersTable: function() {
        console.log('🔄🔄🔄 AUTO-REFRESH: refreshListenersTable called');
        if (typeof loadListenersData === 'function') {
            console.log('🔄🔄🔄 AUTO-REFRESH: calling loadListenersData');
            loadListenersData();
        } else {
            console.log('🔄🔄🔄 AUTO-REFRESH: loadListenersData function not found!');
        }
    }
};

// Utility functions
function formatUptime(seconds) {
    if (!seconds || seconds < 0) return '0s';
    
    var days = Math.floor(seconds / 86400);
    var hours = Math.floor((seconds % 86400) / 3600);
    var minutes = Math.floor((seconds % 3600) / 60);
    var secs = Math.floor(seconds % 60);
    
    if (days > 0) {
        return days + 'd ' + hours + 'h ' + minutes + 'm';
    } else if (hours > 0) {
        return hours + 'h ' + minutes + 'm ' + secs + 's';
    } else if (minutes > 0) {
        return minutes + 'm ' + secs + 's';
    } else {
        return secs + 's';
    }
}

function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    
    var k = 1024;
    var sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    var i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

console.log('Auto-refresh system loaded');
