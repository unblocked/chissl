// Listeners View Module
// Handles HTTP listener management functionality

console.log('🚨🚨🚨 LISTENERS VIEW MODULE LOADED - VERSION 2024-09-07-01:00 - OUTLINE BUTTONS 🚨🚨🚨');

// Show AI listener modal directly (redirect to new unified modal)
function showAddAIListenerModal() {
    console.log('🤖 Opening AI Listener Modal - using unified modal');
    showAIResponsePreview(null, null, 'create', null);
}

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
        '<th>URL</th>' +
        '<th>User</th>' +
        '<th>Port</th>' +
        '<th>Mode</th>' +
        '<th>Target/Response</th>' +
        '<th>Status</th>' +
        '<th>Created</th>' +
        '<th>Actions</th>' +
        '</tr>' +
        '</thead>' +
        '<tbody id="listeners-tbody">' +
        '<tr><td colspan="9" class="text-center">Loading listeners...</td></tr>' +
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
            console.log('🔍 Raw API data received:', data);
            var tbody = '';
            if (data && data.length > 0) {
                data.forEach(function(listener) {
                    var modeStr = String(listener.mode || '').toLowerCase().trim();
                    var isAI = (modeStr.indexOf('ai') !== -1) || (listener.ai_provider_id) || (listener.ai_provider_name) || (listener.ai_generation_status);
                    if (isAI) {
                        console.log('🚫 Skipping AI listener:', listener.id, 'mode:', listener.mode);
                        return; // Skip AI listeners entirely
                    }
                    if (!(modeStr === 'proxy' || modeStr === 'static')) {
                        console.log('🚫 Skipping non-regular listener:', listener.id, 'mode:', listener.mode);
                        return; // Skip any unknown modes
                    }

                    console.log('✅ Processing regular listener:', listener.id, 'mode:', listener.mode);

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
                    } else if (listener.mode === 'ai-mock') {
                        var aiStatus = listener.ai_generation_status || 'pending';
                        var statusClass = aiStatus === 'success' ? 'text-success' :
                                         aiStatus === 'failed' ? 'text-danger' : 'text-warning';
                        targetDisplay = '<small class="' + statusClass + '">AI: ' + aiStatus + '</small>';
                        if (listener.ai_provider_name) {
                            targetDisplay += '<br><small class="text-muted">Provider: ' + escapeHtml(listener.ai_provider_name) + '</small>';
                        }
                    }
                    
                    var protocol = listener.use_tls ? 'https' : 'http';
                    var url = protocol + '://localhost:' + listener.port;
                    var createdDate = listener.created_at ? new Date(listener.created_at).toLocaleString() : 'Unknown';

                    tbody += '<tr data-mode="' + escapeHtml(listener.mode || '') + '">' +
                        '<td><strong>' + escapeHtml(listener.name || 'Unnamed') + '</strong><br>' +
                        '<small class="text-muted">' + listener.id.substring(0, 8) + '...</small></td>' +
                        '<td><code>' + url + '</code></td>' +
                        '<td>' + escapeHtml(listener.username || 'Unknown') + '</td>' +
                        '<td><code>' + listener.port + '</code></td>' +
                        '<td><span class="badge badge-info">' + listener.mode + '</span></td>' +
                        '<td>' + targetDisplay + '</td>' +
                        '<td><span class="badge ' + statusClass + '">' + statusText + '</span></td>' +
                        '<td>' + createdDate + '</td>' +
                        '<td>' +
                        '<div class="btn-group btn-group-sm" role="group">' +
                        '<button class="btn btn-outline-info" onclick="showTrafficPayloads(\'' + listener.id + '\', \'listener\')" title="Inspect Traffic">' +
                        '<i class="fas fa-eye"></i></button>' +
                        '<button class="btn btn-outline-warning" onclick="editListener(\'' + listener.id + '\')" title="Edit Listener">' +
                        '<i class="fas fa-edit"></i></button>' +
                        '<button class="btn btn-outline-danger" onclick="deleteListener(\'' + listener.id + '\')" title="Delete">' +
                        '<i class="fas fa-trash"></i></button>' +
                        '</div>' +
                        '</td>' +
                        '</tr>';
                });
            } else {
                tbody = '<tr><td colspan="9" class="text-center text-muted">No listeners found</td></tr>';
            }
            $('#listeners-tbody').html(tbody);
            // Post-render hard guard: remove any AI rows that slipped through
            try {
                var removed = 0;
                $('#listeners-tbody tr').each(function() {
                    var modeAttr = ($(this).attr('data-mode') || '').toLowerCase();
                    var modeText = $(this).find('td').eq(4).text().toLowerCase();
                    if (modeAttr.indexOf('ai') !== -1 || modeText.indexOf('ai') !== -1) {
                        $(this).remove();
                        removed++;
                    }
                });
                if (removed > 0) {
                    console.log('🧹 Removed AI rows from listeners table:', removed);
                }
            } catch(e) { console.warn('AI row cleanup failed:', e); }
        })
        .fail(function() {
            $('#listeners-tbody').html('<tr><td colspan="10" class="text-center text-danger">Failed to load listeners</td></tr>');
        });
}

// Listener management functions
function showAddListenerModal() {
    console.log('Opening Add Listener Modal - AI Mock API option should be visible');
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
        '<option value="ai-mock" style="background-color: #e3f2fd; font-weight: bold;">🤖 AI Mock API</option>' +
        '</select>' +
        '<small class="form-text text-muted">Select "AI Mock API" to create intelligent mock APIs from OpenAPI specifications</small>' +
        '</div>' +
        '<div class="form-group" id="targetUrlGroup">' +
        '<label for="newTargetUrl">Target URL</label>' +
        '<input type="url" class="form-control" id="newTargetUrl" placeholder="http://localhost:3000">' +
        '</div>' +
        '<div class="form-group" id="responseGroup" style="display: none;">' +
        '<label for="newResponse">Static Response</label>' +
        '<textarea class="form-control" id="newResponse" rows="4" placeholder="Hello World!"></textarea>' +
        '</div>' +
        '<div class="form-group" id="aiGroup" style="display: none; background-color: #f8f9fa; padding: 15px; border-radius: 5px; border: 2px solid #007bff;">' +
        '<h5 style="color: #007bff; margin-bottom: 15px;">🤖 AI Mock API Configuration</h5>' +
        '<div class="row">' +
        '<div class="col-md-6">' +
        '<label for="newAIProvider">AI Provider *</label>' +
        '<select class="form-control" id="newAIProvider">' +
        '<option value="">Select AI Provider...</option>' +
        '</select>' +
        '<small class="form-text text-muted">Configure AI providers in Server Settings first</small>' +
        '</div>' +
        '<div class="col-md-6">' +
        '<label for="openApiFile">OpenAPI Specification *</label>' +
        '<input type="file" class="form-control-file" id="openApiFile" accept=".json,.yaml,.yml">' +
        '<small class="form-text text-muted">Upload OpenAPI/Swagger spec file</small>' +
        '</div>' +
        '</div>' +
        '<div class="form-group mt-3">' +
        '<label for="systemPrompt">Additional Instructions (Optional)</label>' +
        '<textarea class="form-control" id="systemPrompt" rows="4" placeholder="Optional: Provide additional instructions to customize the AI-generated responses...&#10;&#10;Examples:&#10;- Use realistic company names and data&#10;- Include specific error scenarios&#10;- Generate responses for a healthcare API&#10;- Use European date formats"></textarea>' +
        '<small class="form-text text-muted">These instructions supplement the built-in system prompt for API mock generation.</small>' +
        '</div>' +
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

    // Debug: Check if AI Mock API option exists
    setTimeout(function() {
        var aiOption = $('#newMode option[value="ai-mock"]');
        console.log('AI Mock API option found:', aiOption.length > 0);
        if (aiOption.length > 0) {
            console.log('AI Mock API option text:', aiOption.text());
            console.log('🤖 AI MOCK API OPTION IS VISIBLE! Look for the robot emoji in the dropdown!');
        } else {
            console.error('❌ AI Mock API option NOT FOUND in dropdown!');
            alert('ERROR: AI Mock API option not found! Check console for details.');
        }

        // Force show all options for debugging
        $('#newMode option').each(function(i, option) {
            console.log('Dropdown option ' + i + ':', $(option).val(), '=', $(option).text());
        });
    }, 100);

    $('#addListenerModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });
}

// Toggle fields based on mode selection
function toggleModeFields() {
    var mode = $('#newMode').val();
    console.log('Mode selected:', mode);

    // Hide all mode-specific groups first
    $('#targetUrlGroup').hide();
    $('#responseGroup').hide();
    $('#aiGroup').hide();

    if (mode === 'proxy') {
        $('#targetUrlGroup').show();
    } else if (mode === 'static') {
        $('#responseGroup').show();
    } else if (mode === 'ai-mock') {
        console.log('AI Mock mode selected - showing AI fields');
        $('#aiGroup').show();
        loadAIProvidersForSelect();
    }
}

function createListener() {
    var mode = $('#newMode').val();
    var listenerData = {
        name: $('#newName').val().trim(),
        port: parseInt($('#newPort').val()),
        mode: mode,
        target_url: $('#newTargetUrl').val(),
        response: $('#newResponse').val(),
        use_tls: $('#newUseTLS').is(':checked')
    };

    // Handle AI-specific data
    if (mode === 'ai-mock') {
        listenerData.ai_provider_id = $('#newAIProvider').val();
        listenerData.system_prompt = $('#systemPrompt').val().trim();

        // Validation for AI mode
        if (!listenerData.ai_provider_id) {
            alert('Please select an AI provider');
            return;
        }

        var fileInput = $('#openApiFile')[0];
        if (!fileInput.files || fileInput.files.length === 0) {
            alert('Please upload an OpenAPI specification file');
            return;
        }

        // Read the OpenAPI file
        var file = fileInput.files[0];
        var reader = new FileReader();
        reader.onload = function(e) {
            try {
                // Try to parse as JSON first
                var spec = JSON.parse(e.target.result);
                listenerData.openapi_spec = JSON.stringify(spec);
                createAIListener(listenerData);
            } catch (jsonError) {
                // If JSON parsing fails, assume it's YAML and send as-is
                listenerData.openapi_spec = e.target.result;
                createAIListener(listenerData);
            }
        };
        reader.readAsText(file);
    } else {
        // Regular listener creation
        $.post('/api/listeners', listenerData)
        .done(function(data) {
            $('#addListenerModal').modal('hide');
            loadListenersData();
            if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
            if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
            setTimeout(function(){
                try { loadListenersData(); } catch(e){}
                if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
                if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
            }, 1000);
            alert('Listener created successfully');
        })
        .fail(function(xhr) {
            alert('Failed to create listener: ' + (xhr.responseText || 'Unknown error'));
        });
    }
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
            if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
            if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
            setTimeout(function(){
                try { loadListenersData(); } catch(e){}
                if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
                if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
            }, 1000);
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
            if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
            if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
            alert('Listener deleted successfully');
        })
        .fail(function(xhr) {
            alert('Failed to delete listener: ' + (xhr.responseText || 'Unknown error'));
        });
    }
}

// Load AI providers for the select dropdown
function loadAIProvidersForSelect() {
    console.log('Loading AI providers for select dropdown');
    $.get('/api/ai-providers')
        .done(function(providers) {
            console.log('AI providers loaded:', providers);
            var options = '<option value="">Select AI Provider...</option>';
            if (providers && providers.length > 0) {
                providers.forEach(function(provider) {
                    if (provider.enabled) {
                        var statusText = provider.test_status === 'success' ? ' ✓' :
                                       provider.test_status === 'failed' ? ' ✗' : '';
                        options += '<option value="' + provider.id + '">' +
                                  escapeHtml(provider.name) + ' (' + provider.provider_type.toUpperCase() + ')' +
                                  statusText + '</option>';
                    }
                });
            } else {
                console.log('No AI providers found or none enabled');
            }
            $('#newAIProvider').html(options);
        })
        .fail(function(xhr) {
            console.error('Failed to load AI providers:', xhr);
            $('#newAIProvider').html('<option value="">Failed to load AI providers</option>');
        });
}

// Create AI-powered listener
function createAIListener(listenerData) {
    $.post('/api/ai-listeners', listenerData)
    .done(function(data) {
        $('#addListenerModal').modal('hide');
        loadListenersData();
            if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
            if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
            setTimeout(function(){
                try { loadListenersData(); } catch(e){}
                if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
                if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
            }, 1000);

        // Show success message with generation status
        var message = 'AI Listener created successfully!\n\n';
        message += 'The AI is now processing your OpenAPI specification to generate mock responses.\n';
        message += 'This may take a few moments. You can check the generation status in the listeners list.';
        alert(message);
    })
    .fail(function(xhr) {
        var errorMsg = 'Failed to create AI listener';
        if (xhr.responseJSON && xhr.responseJSON.error) {
            errorMsg = xhr.responseJSON.error;
        } else if (xhr.responseText) {
            errorMsg = xhr.responseText;
        }
        alert('Error: ' + errorMsg);
    });
}

// Show AI chat modal for AI-powered listeners
function showAIChat(listenerID) {
    // First, get the AI listener information
    $.get('/api/listeners')
        .done(function(listeners) {
            var listener = listeners.find(l => l.id === listenerID);
            if (!listener || listener.mode !== 'ai-mock') {
                alert('This is not an AI-powered listener');
                return;
            }

            var modalHtml = '<div class="modal fade" id="aiChatModal" tabindex="-1" role="dialog">' +
                '<div class="modal-dialog modal-lg" role="document">' +
                '<div class="modal-content">' +
                '<div class="modal-header">' +
                '<h4 class="modal-title">Chat with AI - ' + escapeHtml(listener.name) + '</h4>' +
                '<button type="button" class="close" data-dismiss="modal">&times;</button>' +
                '</div>' +
                '<div class="modal-body">' +
                '<div class="row">' +
                '<div class="col-md-12">' +
                '<div class="alert alert-info">' +
                '<i class="fas fa-robot"></i> ' +
                'Chat with the AI to refine and improve the generated mock responses. ' +
                'You can ask for changes, additional error cases, or different data formats.' +
                '</div>' +
                '<div id="chatMessages" style="height: 300px; overflow-y: auto; border: 1px solid #ddd; padding: 15px; background: #f8f9fa; margin-bottom: 15px;">' +
                '<div class="text-muted">Loading conversation...</div>' +
                '</div>' +
                '<div class="input-group">' +
                '<input type="text" class="form-control" id="chatInput" placeholder="Type your message..." onkeypress="handleChatKeyPress(event, \'' + listenerID + '\')">' +
                '<div class="input-group-append">' +
                '<button class="btn btn-primary" onclick="sendChatMessage(\'' + listenerID + '\')">' +
                '<i class="fas fa-paper-plane"></i> Send</button>' +
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
            $('#aiChatModal').modal('show');

            $('#aiChatModal').on('hidden.bs.modal', function() {
                $(this).remove();
            });

            // Load existing conversation
            loadAIConversation(listenerID);
        })
        .fail(function() {
            alert('Failed to load listener information');
        });
}

// Load existing AI conversation
function loadAIConversation(listenerID) {
    // For now, show a welcome message
    // TODO: Load actual conversation history from the AI listener
    var welcomeMessage = '<div class="mb-3">' +
        '<div class="d-flex">' +
        '<div class="mr-2"><i class="fas fa-robot text-primary"></i></div>' +
        '<div class="flex-grow-1">' +
        '<div class="bg-light p-2 rounded">' +
        '<strong>AI Assistant:</strong> Hello! I\'ve generated mock responses for your API based on the OpenAPI specification. ' +
        'How can I help you improve them? You can ask me to modify the data, add error cases, or adjust the response format.' +
        '</div>' +
        '<small class="text-muted">' + new Date().toLocaleTimeString() + '</small>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('#chatMessages').html(welcomeMessage);
}

// Handle Enter key press in chat input
function handleChatKeyPress(event, listenerID) {
    if (event.key === 'Enter') {
        sendChatMessage(listenerID);
    }
}

// Send chat message to AI
function sendChatMessage(listenerID) {
    var message = $('#chatInput').val().trim();
    if (!message) {
        return;
    }

    // Clear input
    $('#chatInput').val('');

    // Add user message to chat
    var userMessageHtml = '<div class="mb-3">' +
        '<div class="d-flex justify-content-end">' +
        '<div class="ml-2">' +
        '<div class="bg-primary text-white p-2 rounded">' +
        '<strong>You:</strong> ' + escapeHtml(message) +
        '</div>' +
        '<small class="text-muted float-right">' + new Date().toLocaleTimeString() + '</small>' +
        '</div>' +
        '<div class="ml-2"><i class="fas fa-user text-primary"></i></div>' +
        '</div>' +
        '</div>';

    $('#chatMessages').append(userMessageHtml);

    // Show typing indicator
    var typingHtml = '<div id="typing-indicator" class="mb-3">' +
        '<div class="d-flex">' +
        '<div class="mr-2"><i class="fas fa-robot text-primary"></i></div>' +
        '<div class="flex-grow-1">' +
        '<div class="bg-light p-2 rounded">' +
        '<i class="fas fa-spinner fa-spin"></i> AI is thinking...' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('#chatMessages').append(typingHtml);

    // Scroll to bottom
    $('#chatMessages').scrollTop($('#chatMessages')[0].scrollHeight);

    // Send message to AI
    $.post('/api/ai-listeners/' + listenerID + '/chat', {
        message: message
    })
    .done(function(response) {
        // Remove typing indicator
        $('#typing-indicator').remove();

        // Add AI response
        var aiMessageHtml = '<div class="mb-3">' +
            '<div class="d-flex">' +
            '<div class="mr-2"><i class="fas fa-robot text-primary"></i></div>' +
            '<div class="flex-grow-1">' +
            '<div class="bg-light p-2 rounded">' +
            '<strong>AI Assistant:</strong> ' + escapeHtml(response.response) +
            '</div>' +
            '<small class="text-muted">' + new Date().toLocaleTimeString() + '</small>' +
            '</div>' +
            '</div>' +
            '</div>';

        $('#chatMessages').append(aiMessageHtml);

        // Scroll to bottom
        $('#chatMessages').scrollTop($('#chatMessages')[0].scrollHeight);

        // Refresh listeners data to show any updates
        loadListenersData();
            if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
            if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
    })
    .fail(function(xhr) {
        // Remove typing indicator
        $('#typing-indicator').remove();

        var errorMsg = 'Failed to send message';
        if (xhr.responseJSON && xhr.responseJSON.error) {
            errorMsg = xhr.responseJSON.error;
        }

        var errorHtml = '<div class="mb-3">' +
            '<div class="d-flex">' +
            '<div class="mr-2"><i class="fas fa-exclamation-triangle text-danger"></i></div>' +
            '<div class="flex-grow-1">' +
            '<div class="bg-danger text-white p-2 rounded">' +
            '<strong>Error:</strong> ' + errorMsg +
            '</div>' +
            '</div>' +
            '</div>' +
            '</div>';

        $('#chatMessages').append(errorHtml);
        $('#chatMessages').scrollTop($('#chatMessages')[0].scrollHeight);
    });
}

// Utility function to escape HTML
function escapeHtml(text) {
    if (!text) return '';
    var map = {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#039;'
    };
    return text.replace(/[&<>"']/g, function(m) { return map[m]; });
}

// Show AI response preview modal (unified for create/edit/preview)
function showAIResponsePreview(listenerData, previewData, mode, editData) {
    // mode can be 'create', 'edit', or 'preview' (default)
    mode = mode || 'preview';
    var isConfigMode = mode === 'create' || mode === 'edit';
    var isEditMode = mode === 'edit';
    var modalTitle = isConfigMode ?
        (isEditMode ? 'Edit AI Endpoint' : 'Create AI Endpoint') :
        'AI Generated Mock Responses Preview';
    var headerClass = isConfigMode ? 'bg-primary' : 'bg-info';
    var headerIcon = isConfigMode ? 'fa-robot' : 'fa-eye';

    var modalHtml = '<div class="modal fade" id="aiPreviewModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog modal-xl" role="document">' +
        '<div class="modal-content">' +
        '<div class="modal-header ' + headerClass + ' text-white">' +
        '<h4 class="modal-title"><i class="fas ' + headerIcon + '"></i> ' + modalTitle + '</h4>' +
        '<button type="button" class="close text-white" data-dismiss="modal">&times;</button>' +
        '</div>' +
        '<div class="modal-body" style="max-height: 70vh; overflow-y: auto;">' +
        (isConfigMode ? '' : '<div class="alert alert-info">' +
        '<i class="fas fa-info-circle"></i> <strong>Preview:</strong> ' +
        'Review the AI-generated mock responses below. You can refine them by providing additional instructions.' +
        '</div>') +
        '<div class="row">' +
        (isConfigMode ? '<div class="col-md-4">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h5><i class="fas fa-cog"></i> Configuration</h5>' +
        '</div>' +
        '<div class="card-body">' +
        '<form id="aiEndpointConfigForm">' +
        '<div class="form-group">' +
        '<label for="aiConfigName">Name *</label>' +
        '<input type="text" class="form-control" id="aiConfigName" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="aiConfigPort">Port *</label>' +
        '<input type="number" class="form-control" id="aiConfigPort" min="1024" max="65535" required>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="aiConfigProvider">AI Provider *</label>' +
        '<select class="form-control" id="aiConfigProvider" required>' +
        '<option value="">Loading providers...</option>' +
        '</select>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="aiConfigFile">OpenAPI Specification *</label>' +
        '<input type="file" class="form-control-file" id="aiConfigFile" accept=".json,.yaml,.yml">' +
        '<small class="form-text text-muted">Upload your OpenAPI/Swagger specification file</small>' +
        '</div>' +
        '<div class="form-group">' +
        '<label for="aiConfigInstructions">Additional Instructions (Optional)</label>' +
        '<textarea class="form-control" id="aiConfigInstructions" rows="4" ' +
        'placeholder="Optional: Provide additional instructions to customize the AI-generated responses...&#10;&#10;Examples:&#10;- Use realistic company names and data&#10;- Include specific error scenarios&#10;- Generate responses for a healthcare API&#10;- Use European date formats"></textarea>' +
        '<small class="form-text text-muted">These instructions supplement the built-in system prompt for API mock generation.</small>' +
        '</div>' +
        '<div class="form-group">' +
        '<div class="form-check">' +
        '<input type="checkbox" class="form-check-input" id="aiConfigTLS">' +
        '<label class="form-check-label" for="aiConfigTLS">Enable TLS/HTTPS</label>' +
        '</div>' +
        '</div>' +
        '</form>' +
        '</div>' +
        '</div>' +
        '</div>' : '') +
        '<div class="' + (isConfigMode ? 'col-md-8' : 'col-md-8') + '">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h5><i class="fas fa-code"></i> Generated API Paths</h5>' +
        '</div>' +
        '<div class="card-body" style="max-height: 400px; overflow-y: auto;">' +
        '<div id="apiEndpointsPreview"></div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="col-md-4">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h5><i class="fas fa-edit"></i> ' + (isConfigMode ? 'Generate & Refine' : 'Refinement Instructions') + '</h5>' +
        '</div>' +
        '<div class="card-body">' +
        '<textarea class="form-control" id="refinementInstructions" rows="8" ' +
        'placeholder="Optional: Provide specific instructions to refine the generated responses...&#10;&#10;Examples:&#10;- Make user names more diverse&#10;- Add more product categories&#10;- Include error responses for validation&#10;- Use realistic email domains"></textarea>' +
        '<small class="form-text text-muted">Leave empty to accept the current responses as-is.</small>' +
        '</div>' +
        '</div>' +
        '<div class="card mt-3">' +
        '<div class="card-header">' +
        '<h5><i class="fas fa-info"></i> Summary</h5>' +
        '</div>' +
        '<div class="card-body">' +
        '<div id="previewSummary"></div>' +
        '</div>' +
        '</div>' +
        '<div class="card mt-3">' +
        '<div class="card-header">' +
        '<h5><i class="fas fa-bug"></i> Debug Info</h5>' +
        '</div>' +
        '<div class="card-body">' +
        '<div id="debugInfo"></div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>' +
        (isConfigMode ?
            '<button type="button" class="btn btn-primary" id="generatePreviewBtn" onclick="generateAIPreviewFromConfig()">' +
            '<i class="fas fa-magic"></i> Generate Preview</button>' +
            '<button type="button" class="btn btn-warning" onclick="refineAIResponses()" style="display: none;">' +
            '<i class="fas fa-magic"></i> Refine Responses</button>' +
            '<button type="button" class="btn btn-success" onclick="acceptAIResponses()" style="display: none;">' +
            '<i class="fas fa-check"></i> ' + (isEditMode ? 'Update Endpoint' : 'Create Endpoint') + '</button>' :
            '<button type="button" class="btn btn-warning" onclick="refineAIResponses()">' +
            '<i class="fas fa-magic"></i> Refine Responses</button>' +
            '<button type="button" class="btn btn-success" onclick="acceptAIResponses()">' +
            '<i class="fas fa-check"></i> Accept & Create Listener</button>') +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('body').append(modalHtml);
    $('#aiPreviewModal').modal('show');

    $('#aiPreviewModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });

    // Store data for later use
    window.currentAIListenerData = listenerData;
    window.currentAIPreviewData = previewData;
    window.currentAIModalMode = mode;
    window.currentAIEditData = editData;

    if (isConfigMode) {
        // Load AI providers for configuration
        loadAIProvidersForConfig();

        // If edit mode, populate form
        if (isEditMode && editData) {
            populateAIConfigForm(editData);
        }

        // Show initial state
        $('#apiEndpointsPreview').html('<div class="text-center text-muted py-5">' +
            '<i class="fas fa-robot fa-3x mb-3"></i>' +
            '<h5>Ready to Generate AI Responses</h5>' +
            '<p>Fill in the configuration and click "Generate Preview" to see AI-generated mock API paths and responses.</p>' +
            '</div>');
    } else {
        // Populate the preview (existing behavior)
        populateAIPreview(previewData);
    }
}

// Populate the AI preview with generated responses
function populateAIPreview(previewData) {
    try {
        var responses = JSON.parse(previewData.generated_responses);
        var pathsHtml = '';
        var pathCount = 0;
        var methodCount = 0;

        if (responses.paths) {
            for (var path in responses.paths) {
                pathCount++;
                pathsHtml += '<div class="path-group mb-4">';
                pathsHtml += '<h6 class="text-primary"><i class="fas fa-route"></i> ' + escapeHtml(path) + '</h6>';

                for (var method in responses.paths[path]) {
                    methodCount++;
                    var methodData = responses.paths[path][method];
                    var methodClass = getMethodClass(method);

                    pathsHtml += '<div class="method-group ml-3 mb-3">';
                    pathsHtml += '<div class="d-flex align-items-center mb-2">';
                    pathsHtml += '<span class="badge badge-' + methodClass + ' mr-2">' + method.toUpperCase() + '</span>';
                    pathsHtml += '<strong>' + escapeHtml(path) + '</strong>';
                    pathsHtml += '</div>';

                    if (methodData.responses) {
                        for (var statusCode in methodData.responses) {
                            var response = methodData.responses[statusCode];
                            var statusClass = getStatusClass(statusCode);

                            pathsHtml += '<div class="response-group ml-4 mb-2">';
                            pathsHtml += '<div class="d-flex align-items-center mb-1">';
                            pathsHtml += '<span class="badge badge-' + statusClass + ' mr-2">' + statusCode + '</span>';
                            pathsHtml += '<small class="text-muted">Response</small>';
                            pathsHtml += '</div>';

                            if (response.content && response.content['application/json'] && response.content['application/json'].example) {
                                pathsHtml += '<pre class="bg-light p-2 rounded" style="font-size: 12px; max-height: 200px; overflow-y: auto;">';
                                pathsHtml += escapeHtml(JSON.stringify(response.content['application/json'].example, null, 2));
                                pathsHtml += '</pre>';
                            }
                            pathsHtml += '</div>';
                        }
                    }
                    pathsHtml += '</div>';
                }
                pathsHtml += '</div>';
            }
        }

        $('#apiEndpointsPreview').html(pathsHtml);

        // Update summary
        var summaryHtml = '<div class="text-center">';
        summaryHtml += '<div class="row">';
        summaryHtml += '<div class="col-6">';
        summaryHtml += '<h4 class="text-primary">' + pathCount + '</h4>';
        summaryHtml += '<small>API Paths</small>';
        summaryHtml += '</div>';
        summaryHtml += '<div class="col-6">';
        summaryHtml += '<h4 class="text-success">' + methodCount + '</h4>';
        summaryHtml += '<small>HTTP Methods</small>';
        summaryHtml += '</div>';
        summaryHtml += '</div>';
        summaryHtml += '</div>';

        $('#previewSummary').html(summaryHtml);

        // Show debug information
        var debugHtml = '';
        if (previewData.debug_info) {
            var debug = previewData.debug_info;
            debugHtml += '<div class="row">';
            debugHtml += '<div class="col-6"><strong>Provider:</strong><br>' + debug.provider_name + ' (' + debug.provider_type + ')</div>';
            debugHtml += '<div class="col-6"><strong>Model:</strong><br>' + debug.model + '</div>';
            debugHtml += '</div>';
            debugHtml += '<div class="row mt-2">';
            debugHtml += '<div class="col-6"><strong>OpenAPI Size:</strong><br>' + debug.openapi_spec_size + ' chars</div>';
            debugHtml += '<div class="col-6"><strong>Response Size:</strong><br>' + debug.response_size + ' chars</div>';
            debugHtml += '</div>';
            if (debug.system_prompt) {
                debugHtml += '<div class="mt-2"><strong>System Prompt:</strong><br>';
                debugHtml += '<pre class="bg-light p-2 rounded" style="font-size: 11px; max-height: 100px; overflow-y: auto;">';
                debugHtml += escapeHtml(debug.system_prompt);
                debugHtml += '</pre></div>';
            }
        } else {
            debugHtml = '<div class="text-muted">No debug information available</div>';
        }
        $('#debugInfo').html(debugHtml);

    } catch (error) {
        $('#apiEndpointsPreview').html('<div class="alert alert-danger">Error parsing generated responses: ' + error.message + '</div>');
        $('#previewSummary').html('<div class="text-danger">Parse Error</div>');
        $('#debugInfo').html('<div class="text-danger">Debug info unavailable due to parse error</div>');
    }
}

// Get CSS class for HTTP method
function getMethodClass(method) {
    switch (method.toUpperCase()) {
        case 'GET': return 'success';
        case 'POST': return 'primary';
        case 'PUT': return 'warning';
        case 'DELETE': return 'danger';
        case 'PATCH': return 'info';
        default: return 'secondary';
    }
}

// Get CSS class for HTTP status code
function getStatusClass(statusCode) {
    var code = parseInt(statusCode);
    if (code >= 200 && code < 300) return 'success';
    if (code >= 300 && code < 400) return 'info';
    if (code >= 400 && code < 500) return 'warning';
    if (code >= 500) return 'danger';
    return 'secondary';
}

// Refine AI responses with user instructions
function refineAIResponses() {
    var instructions = $('#refinementInstructions').val().trim();
    if (!instructions) {
        alert('Please provide refinement instructions or click "Accept & Create Listener" to proceed with current responses.');
        return;
    }

    // Show loading state
    $('.modal-footer .btn-warning').prop('disabled', true).html('<i class="fas fa-spinner fa-spin"></i> Refining...');
    $('.modal-footer .btn-success').prop('disabled', true);

    // Add user instructions to the listener data
    var refinementData = Object.assign({}, window.currentAIListenerData);
    refinementData.user_instructions = instructions;
    refinementData.previous_responses = window.currentAIPreviewData.generated_responses;

    $.ajax({
        url: '/api/ai-listeners/refine',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify(refinementData)
    })
    .done(function(refinedData) {
        // Update the preview with refined responses
        window.currentAIPreviewData = refinedData;
        populateAIPreview(refinedData);

        // Clear instructions and show success
        $('#refinementInstructions').val('');

        // Re-enable buttons
        $('.modal-footer .btn-warning').prop('disabled', false).html('<i class="fas fa-magic"></i> Refine Responses');
        $('.modal-footer .btn-success').prop('disabled', false);

        // Show success message
        var successHtml = '<div class="alert alert-success alert-dismissible fade show mt-2">' +
            '<i class="fas fa-check-circle"></i> Responses refined successfully! Review the updated endpoints below.' +
            '<button type="button" class="close" data-dismiss="alert">&times;</button>' +
            '</div>';
        $('.modal-body').prepend(successHtml);

        // Auto-dismiss success message
        setTimeout(function() {
            $('.alert-success').alert('close');
        }, 5000);
    })
    .fail(function(xhr) {
        var errorMsg = 'Failed to refine responses';
        if (xhr.responseJSON && xhr.responseJSON.error) {
            errorMsg = xhr.responseJSON.error;
        }
        alert('Error: ' + errorMsg);

        // Re-enable buttons
        $('.modal-footer .btn-warning').prop('disabled', false).html('<i class="fas fa-magic"></i> Refine Responses');
        $('.modal-footer .btn-success').prop('disabled', false);
    });
}

// Accept AI responses and create the listener
function acceptAIResponses() {
    console.log('🚀 acceptAIResponses called');
    console.log('📊 currentAIListenerData:', window.currentAIListenerData);
    console.log('📊 currentAIPreviewData:', window.currentAIPreviewData);

    // Show loading state
    $('.modal-footer .btn-success').prop('disabled', true).html('<i class="fas fa-spinner fa-spin"></i> Creating Listener...');
    $('.modal-footer .btn-warning').prop('disabled', true);

    // Prepare final listener data - only send what the server expects
    var finalData = {
        name: window.currentAIListenerData.name,
        port: window.currentAIListenerData.port,
        ai_provider_id: window.currentAIListenerData.ai_provider_id,
        openapi_spec: window.currentAIListenerData.openapi_spec,
        system_prompt: window.currentAIListenerData.system_prompt || '',
        use_tls: window.currentAIListenerData.use_tls || false
    };

    console.log('📤 Sending finalData:', finalData);

    $.ajax({
        url: '/api/ai-listeners',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify(finalData)
    })
    .done(function(data) {
        $('#aiPreviewModal').modal('hide');
        loadListenersData();
        if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
        if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
        setTimeout(function(){
            try { loadListenersData(); } catch(e){}
            if (typeof loadNewListenersData === 'function') { try { loadNewListenersData(); } catch(e){} }
            if (typeof loadAIMockData === 'function') { try { loadAIMockData(); } catch(e){} }
        }, 1000);

        var message = '🎉 AI Listener created successfully!\n\n';
        message += 'Your AI-powered mock API is now active and ready to serve requests.\n';
        message += 'The listener is running on port ' + finalData.port + '.';
        alert(message);
    })
    .fail(function(xhr) {
        var errorMsg = 'Failed to create AI listener';
        if (xhr.responseJSON && xhr.responseJSON.error) {
            errorMsg = xhr.responseJSON.error;
        } else if (xhr.responseText) {
            errorMsg = xhr.responseText;
        }
        alert('Error: ' + errorMsg);

        // Re-enable buttons
        $('.modal-footer .btn-success').prop('disabled', false).html('<i class="fas fa-check"></i> Accept & Create Listener');
        $('.modal-footer .btn-warning').prop('disabled', false);
    });
}

// Edit AI listener - show the same modal as creation for consistency
function editAIListener(listenerID) {
    console.log('🤖 editAIListener called with ID:', listenerID);

    // Get basic listener details
    $.get('/api/listener/' + listenerID)
        .done(function(listener) {
            console.log('📝 Loaded listener for editing:', listener);

            // Show the same unified modal as creation, but in edit mode
            showAIResponsePreview(null, null, 'edit', {
                listener_id: listenerID,
                listener: listener
            });
        })
        .fail(function(xhr) {
            console.error('❌ Failed to load listener:', xhr);
            alert('Failed to load AI listener details: ' + (xhr.responseText || 'Unknown error'));
        });
}

// Show AI listener edit modal with version management
function showAIListenerEditModal(listenerID, versionData) {
    var modalHtml = '<div class="modal fade" id="editAIListenerModal" tabindex="-1" role="dialog">' +
        '<div class="modal-dialog modal-xl" role="document">' +
        '<div class="modal-content">' +
        '<div class="modal-header bg-primary text-white">' +
        '<h4 class="modal-title"><i class="fas fa-robot"></i> Edit AI Listener - Version Management</h4>' +
        '<button type="button" class="close text-white" data-dismiss="modal">&times;</button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<div class="row">' +
        '<div class="col-md-4">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h5><i class="fas fa-history"></i> Response Versions</h5>' +
        '</div>' +
        '<div class="card-body" style="max-height: 400px; overflow-y: auto;">' +
        '<div id="versionsList"></div>' +
        '</div>' +
        '</div>' +
        '<div class="card mt-3">' +
        '<div class="card-header">' +
        '<h5><i class="fas fa-edit"></i> Refine Current Version</h5>' +
        '</div>' +
        '<div class="card-body">' +
        '<textarea class="form-control" id="editRefinementInstructions" rows="6" ' +
        'placeholder="Provide instructions to refine the current AI responses...&#10;&#10;Examples:&#10;- Add more realistic data&#10;- Include additional error cases&#10;- Change response format"></textarea>' +
        '<button class="btn btn-success btn-sm mt-2" onclick="refineCurrentVersion()">' +
        '<i class="fas fa-magic"></i> Create New Version</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="col-md-8">' +
        '<div class="card">' +
        '<div class="card-header">' +
        '<h5><i class="fas fa-code"></i> Current Active Responses</h5>' +
        '</div>' +
        '<div class="card-body" style="max-height: 500px; overflow-y: auto;">' +
        '<div id="currentResponsesPreview"></div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button>' +
        '<button type="button" class="btn btn-info" onclick="downloadAIResponses()">' +
        '<i class="fas fa-download"></i> Download Responses</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';

    $('body').append(modalHtml);
    $('#editAIListenerModal').modal('show');

    $('#editAIListenerModal').on('hidden.bs.modal', function() {
        $(this).remove();
    });

    // Store data for later use
    window.currentEditingListenerID = listenerID;
    window.currentVersionData = versionData;

    // Populate the interface
    populateVersionsList(versionData.versions);
    populateCurrentResponses(versionData.active_version);
}

// Populate the versions list
function populateVersionsList(versions) {
    var versionsHtml = '';

    if (versions && versions.length > 0) {
        versions.forEach(function(version) {
            var isActive = version.is_active;
            var cardClass = isActive ? 'border-primary' : 'border-light';
            var badgeClass = isActive ? 'badge-primary' : 'badge-secondary';
            var statusIcon = version.generation_status === 'success' ? 'fa-check text-success' :
                           version.generation_status === 'failed' ? 'fa-times text-danger' :
                           'fa-spinner fa-spin text-warning';

            versionsHtml += '<div class="card mb-2 ' + cardClass + '">';
            versionsHtml += '<div class="card-body p-2">';
            versionsHtml += '<div class="d-flex justify-content-between align-items-center">';
            versionsHtml += '<div>';
            versionsHtml += '<span class="badge ' + badgeClass + '">v' + version.version_number + '</span>';
            if (isActive) versionsHtml += ' <span class="badge badge-success">Active</span>';
            versionsHtml += '<br><small class="text-muted">' + formatDate(version.created_at) + '</small>';
            if (version.user_instructions) {
                versionsHtml += '<br><small class="text-info"><i class="fas fa-edit"></i> Refined</small>';
            }
            versionsHtml += '</div>';
            versionsHtml += '<div>';
            versionsHtml += '<i class="fas ' + statusIcon + '"></i>';
            versionsHtml += '</div>';
            versionsHtml += '</div>';

            if (!isActive) {
                versionsHtml += '<div class="mt-2">';
                versionsHtml += '<button class="btn btn-sm btn-outline-primary" onclick="activateVersion(\'' + version.id + '\')">';
                versionsHtml += '<i class="fas fa-check"></i> Activate</button>';
                versionsHtml += '</div>';
            }

            versionsHtml += '</div>';
            versionsHtml += '</div>';
        });
    } else {
        versionsHtml = '<div class="text-center text-muted">No versions found</div>';
    }

    $('#versionsList').html(versionsHtml);
}

// Populate current responses preview
function populateCurrentResponses(activeVersion) {
    if (!activeVersion || !activeVersion.generated_responses) {
        $('#currentResponsesPreview').html('<div class="text-center text-muted">No active responses</div>');
        return;
    }

    try {
        var responses = JSON.parse(activeVersion.generated_responses);
        var responsesHtml = '';

        if (responses.paths) {
            for (var path in responses.paths) {
                responsesHtml += '<div class="endpoint-group mb-3">';
                responsesHtml += '<h6 class="text-primary"><i class="fas fa-link"></i> ' + escapeHtml(path) + '</h6>';

                for (var method in responses.paths[path]) {
                    var methodData = responses.paths[path][method];
                    var methodClass = getMethodClass(method);

                    responsesHtml += '<div class="method-group ml-3 mb-2">';
                    responsesHtml += '<span class="badge badge-' + methodClass + ' mr-2">' + method.toUpperCase() + '</span>';

                    if (methodData.responses) {
                        for (var statusCode in methodData.responses) {
                            var response = methodData.responses[statusCode];
                            if (response.content && response.content['application/json'] && response.content['application/json'].example) {
                                responsesHtml += '<pre class="bg-light p-2 rounded mt-1" style="font-size: 11px; max-height: 150px; overflow-y: auto;">';
                                responsesHtml += escapeHtml(JSON.stringify(response.content['application/json'].example, null, 2));
                                responsesHtml += '</pre>';
                            }
                        }
                    }
                    responsesHtml += '</div>';
                }
                responsesHtml += '</div>';
            }
        }

        $('#currentResponsesPreview').html(responsesHtml);
    } catch (error) {
        $('#currentResponsesPreview').html('<div class="alert alert-danger">Error parsing responses: ' + error.message + '</div>');
    }
}

// Refine current version - create a new version with user instructions
function refineCurrentVersion() {
    var instructions = $('#editRefinementInstructions').val().trim();
    if (!instructions) {
        alert('Please provide refinement instructions.');
        return;
    }

    var activeVersion = window.currentVersionData.active_version;
    if (!activeVersion) {
        alert('No active version found to refine.');
        return;
    }

    // Show loading state
    $('.btn-success').prop('disabled', true).html('<i class="fas fa-spinner fa-spin"></i> Creating...');

    var refinementData = {
        ai_listener_id: activeVersion.ai_listener_id,
        openapi_spec: activeVersion.openapi_spec,
        system_prompt: activeVersion.system_prompt,
        user_instructions: instructions,
        previous_responses: activeVersion.generated_responses
    };

    $.ajax({
        url: '/api/ai-listeners/' + window.currentEditingListenerID + '/refine',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify(refinementData)
    })
    .done(function(newVersion) {
        // Refresh the version data
        $.get('/api/ai-listeners/' + window.currentEditingListenerID + '/versions')
            .done(function(updatedData) {
                window.currentVersionData = updatedData;
                populateVersionsList(updatedData.versions);
                populateCurrentResponses(updatedData.active_version);

                // Clear instructions
                $('#editRefinementInstructions').val('');

                // Show success message
                alert('New version created successfully! The refined responses are now active.');
            });
    })
    .fail(function(xhr) {
        var errorMsg = 'Failed to create new version';
        if (xhr.responseJSON && xhr.responseJSON.error) {
            errorMsg = xhr.responseJSON.error;
        }
        alert('Error: ' + errorMsg);
    })
    .always(function() {
        $('.btn-success').prop('disabled', false).html('<i class="fas fa-magic"></i> Create New Version');
    });
}

// Activate a specific version
function activateVersion(versionID) {
    if (!confirm('Are you sure you want to activate this version? It will replace the current active responses.')) {
        return;
    }

    $.ajax({
        url: '/api/ai-listeners/' + window.currentEditingListenerID + '/activate/' + versionID,
        method: 'POST'
    })
    .done(function() {
        // Refresh the version data
        $.get('/api/ai-listeners/' + window.currentEditingListenerID + '/versions')
            .done(function(updatedData) {
                window.currentVersionData = updatedData;
                populateVersionsList(updatedData.versions);
                populateCurrentResponses(updatedData.active_version);

                alert('Version activated successfully!');
            });
    })
    .fail(function(xhr) {
        alert('Failed to activate version: ' + (xhr.responseText || 'Unknown error'));
    });
}

// Download AI responses as JSON
function downloadAIResponses() {
    var activeVersion = window.currentVersionData.active_version;
    if (!activeVersion || !activeVersion.generated_responses) {
        alert('No active responses to download.');
        return;
    }

    try {
        var responses = JSON.parse(activeVersion.generated_responses);
        var dataStr = JSON.stringify(responses, null, 2);
        var dataBlob = new Blob([dataStr], {type: 'application/json'});

        var link = document.createElement('a');
        link.href = URL.createObjectURL(dataBlob);
        link.download = 'ai-responses-v' + activeVersion.version_number + '.json';
        link.click();
    } catch (error) {
        alert('Error preparing download: ' + error.message);
    }
}

// Format date for display
function formatDate(dateString) {
    var date = new Date(dateString);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
}

// Load AI providers for configuration form
function loadAIProvidersForConfig() {
    $.get('/api/ai-providers')
        .done(function(providers) {
            var options = '<option value="">Select AI Provider...</option>';
            if (providers && providers.length > 0) {
                providers.forEach(function(provider) {
                    if (provider.enabled) {
                        var statusText = provider.test_status === 'success' ? ' ✓' :
                                       provider.test_status === 'failed' ? ' ✗' : '';
                        options += '<option value="' + provider.id + '">' +
                                  escapeHtml(provider.name) + ' (' + provider.provider_type + ')' + statusText + '</option>';
                    }
                });
            }
            $('#aiConfigProvider').html(options);
        })
        .fail(function() {
            $('#aiConfigProvider').html('<option value="">Failed to load providers</option>');
        });
}

// Populate AI config form for edit mode
function populateAIConfigForm(editData) {
    if (editData.listener) {
        $('#aiConfigName').val(editData.listener.name);
        $('#aiConfigPort').val(editData.listener.port);
        $('#aiConfigTLS').prop('checked', editData.listener.use_tls);
    }

    if (editData.ai_listener) {
        $('#aiConfigProvider').val(editData.ai_listener.ai_provider_id);
        $('#aiConfigInstructions').val(editData.ai_listener.system_prompt);
    }
}

// Generate AI preview from configuration form
function generateAIPreviewFromConfig() {
    // Validate form
    var name = $('#aiConfigName').val().trim();
    var port = parseInt($('#aiConfigPort').val());
    var providerId = $('#aiConfigProvider').val();
    var fileInput = $('#aiConfigFile')[0];

    if (!name || !port || !providerId) {
        alert('Please fill in all required fields (Name, Port, AI Provider).');
        return;
    }

    if (!fileInput.files || fileInput.files.length === 0) {
        if (window.currentAIModalMode !== 'edit') {
            alert('Please select an OpenAPI specification file.');
            return;
        }
    }

    // Show loading state
    $('#generatePreviewBtn').prop('disabled', true).html('<i class="fas fa-spinner fa-spin"></i> Generating...');

    // Read file content
    if (fileInput.files && fileInput.files.length > 0) {
        var reader = new FileReader();
        reader.onload = function(e) {
            var openApiContent = e.target.result;
            generatePreviewWithData(name, port, providerId, openApiContent);
        };
        reader.readAsText(fileInput.files[0]);
    } else {
        // Edit mode - use existing OpenAPI spec
        var openApiContent = window.currentAIEditData.ai_listener.openapi_spec;
        generatePreviewWithData(name, port, providerId, openApiContent);
    }
}

// Generate preview with collected data
function generatePreviewWithData(name, port, providerId, openApiContent) {
    var instructions = $('#aiConfigInstructions').val().trim();
    var useTLS = $('#aiConfigTLS').is(':checked');

    var requestData = {
        name: name,
        port: port,
        ai_provider_id: providerId,
        openapi_spec: openApiContent,
        system_prompt: instructions,
        use_tls: useTLS,
        mode: 'ai-mock'
    };

    $.ajax({
        url: '/api/ai-listeners/preview',
        method: 'POST',
        contentType: 'application/json',
        data: JSON.stringify(requestData)
    })
    .done(function(previewData) {
        // Store the data
        window.currentAIListenerData = requestData;
        window.currentAIPreviewData = previewData;

        // Show the preview
        populateAIPreview(previewData);

        // Show refinement and create buttons
        $('.btn-warning, .btn-success').show();

        // Update button text
        $('#generatePreviewBtn').html('<i class="fas fa-sync"></i> Regenerate');
    })
    .fail(function(xhr) {
        var errorMsg = 'Failed to generate AI preview';
        var debugInfo = '';

        if (xhr.responseJSON) {
            if (xhr.responseJSON.error) {
                errorMsg = xhr.responseJSON.error;
            }
            if (xhr.responseJSON.details) {
                debugInfo = '\n\nDetails: ' + xhr.responseJSON.details;
            }
        }

        alert('Error: ' + errorMsg + debugInfo);
    })
    .always(function() {
        $('#generatePreviewBtn').prop('disabled', false);
    });
}





// Show unified AI endpoint management modal
function showAIEndpointModal(mode, listenerData) {
    // mode can be 'create' or 'edit'
    // listenerData is provided for edit mode

    var isEditMode = mode === 'edit';
    var modalTitle = isEditMode ? 'Edit AI Endpoint' : 'Create AI Endpoint';
    var submitButtonText = isEditMode ? 'Update & Preview' : 'Generate Preview';

    showAIResponsePreview(null, null, mode, listenerData);
}






