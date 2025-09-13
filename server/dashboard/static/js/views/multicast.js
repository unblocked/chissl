// Multi-cast Tunnels View (Phase 1 - Webhook/SSE)
(function(){
  console.log('🛰️ Multicast View loaded');

  window.showMulticast = function() {
    if (typeof stopLogsAutoRefresh === 'function') stopLogsAutoRefresh();
    if (typeof stopNewTunnelsAutoRefresh === 'function') stopNewTunnelsAutoRefresh();
    if (typeof stopNewListenersAutoRefresh === 'function') stopNewListenersAutoRefresh();
    window.dashboardApp.currentView = 'multicast';
    $('#page-title').text('Multi-cast Tunnels');
    $('.nav-link').removeClass('active');
    $('[onclick="showMulticast()"]').addClass('active');
    loadMulticastView();
  };
  window.showMulticastFromURL = window.showMulticast;

  function isAdmin(){
    try {
      if (window.isAdmin === true) return true;
      var u = window.currentUser || (window.dashboardApp && window.dashboardApp.currentUser) || null;
      if (!u) return false;
      return !!(u.admin || u.is_admin);
    } catch(e){ return false; }
  }

  window.loadMulticastView = function(){
    const $content = $('#main-content');
    $content.html('<div class="p-3"><div id="multicast-admin"></div><div id="multicast-list" class="mt-3"></div></div>');
    if (isAdmin()) {
      $('#multicast-admin').html(`
        <div class="card">
          <div class="card-body">
            <h5 class="card-title">Create Multi-cast Tunnel</h5>
            <div class="form-row">
              <div class="form-group col-md-4"><label>Name</label><input id="mc-name" class="form-control" placeholder="Webhook Fan-out"/></div>
              <div class="form-group col-md-2"><label>Port</label><input id="mc-port" type="number" class="form-control" placeholder="8443"/></div>
              <div class="form-group col-md-2"><label>Visible</label><br><input id="mc-visible" type="checkbox" checked/> Visible to all users</div>
              <div class="form-group col-md-2"><label>Enabled</label><br><input id="mc-enabled" type="checkbox"/> Start now</div>
              <div class="form-group col-md-2"><label>Bidirectional</label><br><input id="mc-bidir" type="checkbox"/> Forward and return first response</div>
            </div>
            <button id="mc-create" class="btn btn-outline-primary">Create</button>
          </div>
        </div>
      `);
      $('#mc-create').off('click').on('click', function(){
        const payload = {
          name: ($('#mc-name').val()||'').trim(),
          port: parseInt($('#mc-port').val(), 10),
          mode: $('#mc-bidir').is(':checked') ? 'bidirectional' : 'webhook',
          visible: $('#mc-visible').is(':checked'),
          enabled: $('#mc-enabled').is(':checked')
        };
        if (!payload.port || payload.port < 1 || payload.port > 65535) { alert('Please enter a valid port'); return; }
        $.ajax({ url: '/api/multicast-tunnels', method: 'POST', contentType: 'application/json', data: JSON.stringify(payload) })
          .done(function(){ loadMulticastView(); })
          .fail(function(xhr){ alert('Create failed: '+(xhr.responseText||xhr.statusText)); });
      });
    }

    const renderTable = function(items, admin){
      let html = '<div class="card"><div class="card-body">';
      html += '<div class="d-flex justify-content-between align-items-center mb-2">'
            + '<h5 class="card-title mb-0">'+(admin?'Manage':'Available')+' Multi-cast Tunnels</h5>'
            + '<button id="mc-refresh" class="btn btn-outline-secondary btn-sm">Refresh</button>'
            + '</div>';
      if (!items || items.length === 0) {
        html += '<div class="text-muted">No multi-cast tunnels yet.</div>';
      } else {
        html += '<div class="table-responsive"><table class="table table-sm table-hover"><thead><tr>'+
                '<th>Name</th><th>Port</th><th>Status</th>'+(admin?'<th>Visible</th><th>Enabled</th><th>Actions</th>':'<th>Connect</th>')+'</tr></thead><tbody>';
        items.forEach(function(it){
          const streamUrl = '/api/capture/multicast/' + it.id + '/stream';
          html += '<tr>'+
            '<td>'+escapeHtml(it.name||'')+'</td>'+
            '<td>'+it.port+'</td>'+
            '<td>'+(it.status||'')+'</td>'+
            (admin
              ? ('<td>'+(it.visible?'<span class="text-success">visible</span>':'<span class="text-muted">hidden</span>')+'</td>'+
                 '<td>'+(it.enabled?'<span class="text-success">enabled</span>':'<span class="text-muted">disabled</span>')+'</td>'+
                 '<td>'+
                   '<div class="btn-group btn-group-sm" role="group">'+
                     '<button class="btn btn-outline-info" data-action="events" data-id="'+it.id+'">Events</button>'+
                     '<button class="btn btn-outline-primary" data-action="edit" data-id="'+it.id+'">Edit</button>'+
                     '<button class="btn btn-outline-secondary" data-action="toggle-visible" data-id="'+it.id+'">'+(it.visible?'Hide':'Make Visible')+'</button>'+
                     '<button class="btn btn-outline-secondary" data-action="toggle-enabled" data-id="'+it.id+'">'+(it.enabled?'Disable':'Enable')+'</button>'+
                     '<button class="btn btn-outline-danger" data-action="delete" data-id="'+it.id+'">Delete</button>'+
                   '</div>'+
                 '</td>')
              : ('<td><button class="btn btn-outline-info btn-sm" data-action="events" data-id="'+it.id+'">Events</button></td>')
            )+
          '</tr>';
        });
        html += '</tbody></table></div>';
      }
      html += '</div></div>';
      $('#multicast-list').html(html);
      // keep items for edit modal lookup
      $('#multicast-list').data('items', items);
      $('#mc-refresh').on('click', loadMulticastView);

      // Wire row actions
      $('#multicast-list [data-action]').off('click').on('click', function(){
        const id = $(this).data('id');
        const action = $(this).data('action');
        if (action === 'events') { showTrafficPayloads(id, 'multicast'); return; }
        if (action === 'edit') {
          const list = $('#multicast-list').data('items') || [];
          const it = list.find(x=>x.id===id);
          if (it) openEditModal(it);
          return;
        }
        if (action === 'toggle-visible') { updateMulticast(id, { visible: true }, { visible: false }); return; }
        if (action === 'toggle-enabled') { updateMulticast(id, { enabled: true }, { enabled: false }); return; }
        if (action === 'delete') {
          if (!confirm('Delete this multicast?')) return;
          $.ajax({ url: '/api/multicast-tunnels/'+encodeURIComponent(id), method: 'DELETE' })
            .done(loadMulticastView)
            .fail(function(xhr){ alert('Delete failed: '+(xhr.responseText||xhr.statusText)); });
        }
      });
    };

    // Admin sees full list with actions; others see the visible-to-all (still authenticated) list
    if (isAdmin()) {
      $.get('/api/multicast-tunnels')
        .done(function(items){ renderTable(items, true); })
        .fail(function(xhr){
          if (xhr && xhr.status === 401) {
            $('#multicast-list').html('<div class="alert alert-warning">Authentication required. Please sign in again.</div>');
          } else {
            $('#multicast-list').html('<div class="alert alert-danger">Failed to load.</div>');
          }
        });
    } else {
      $.get('/api/multicast-tunnels/public')
        .done(function(items){ renderTable(items, false); })
        .fail(function(xhr){
          if (xhr && xhr.status === 401) {
            $('#multicast-list').html('<div class="alert alert-warning">Authentication required. Please sign in again.</div>');
          } else {
            $('#multicast-list').html('<div class="alert alert-danger">Failed to load.</div>');
          }
        });
    }
  };

  function updateMulticast(id, ifFalse, ifTrue){
    // Fetch current row to know current states
    $.get('/api/multicast-tunnels')
      .done(function(items){
        const it = (items||[]).find(x=>x.id===id);
        if (!it) { loadMulticastView(); return; }
        const desired = {};
        if (ifFalse.hasOwnProperty('visible')) desired.visible = !it.visible;
        if (ifFalse.hasOwnProperty('enabled')) desired.enabled = !it.enabled;
        $.ajax({ url: '/api/multicast-tunnels/'+encodeURIComponent(id), method: 'PUT', contentType: 'application/json', data: JSON.stringify(desired) })
          .done(loadMulticastView)
          .fail(function(xhr){ alert('Update failed: '+(xhr.responseText||xhr.statusText)); });
      })
      .fail(function(){ alert('Failed to load current state'); });
  }

  function loadRecent(id){
    $('#multicast-recent').html('<div class="card"><div class="card-body"><div class="d-flex justify-content-between align-items-center mb-2"><h5 class="card-title mb-0">Recent Events</h5><button class="btn btn-outline-secondary btn-sm" id="mc-recent-refresh">Refresh</button></div><div id="mc-recent-out" class="mc-recent" style="max-height:50vh;overflow:auto;"></div></div></div>');
    // inject minimal styles once
    if (!document.getElementById('mc-recent-style')) {
      var st = document.createElement('style');
      st.id = 'mc-recent-style';
      st.textContent = '.mc-recent .evt{border-bottom:1px solid #eee;padding:6px 0}'+
                       '.mc-recent .hdr{font-weight:600}'+
                       '.mc-recent .badge{display:inline-block;padding:2px 6px;border-radius:4px;font-size:12px;margin-left:6px;border:1px solid #ccc;color:#333}'+
                       '.mc-recent pre{background:#f8f9fa;border:1px solid #e9ecef;border-radius:4px;padding:6px;margin:6px 0;white-space:pre-wrap;word-break:break-word;}';
      document.head.appendChild(st);
    }
    function b64ToStr(b){ try { return atob(b||''); } catch(e){ return b||''; } }
    function pretty(v){ try{ return JSON.stringify(JSON.parse(v), null, 2); } catch(e){ return v; } }
    function fmtEvent(e){
      var type = (e && e.type) || '';
      var time = (e && e.time) || '';
      var cid = (e && (e.conn_id || (e.meta && e.meta.conn_id))) || '';
      var header = '<div class="hdr">'+escapeHtml(type)+'<span class="badge">'+escapeHtml(cid)+'</span><span class="text-muted" style="margin-left:6px">'+escapeHtml(time)+'</span></div>';
      var body = '';
      if (e && e.data) {
        var s = b64ToStr(e.data);
        body = pretty(s);
      }
      if (!body && e && e.meta && Object.keys(e.meta).length) {
        body = JSON.stringify(e.meta, null, 2);
      }
      return '<div class="evt">'+ header + (body ? '<pre>'+escapeHtml(body)+'</pre>' : '') + '</div>';
    }
    const doLoad = function(){
      $.get('/api/capture/multicast/'+encodeURIComponent(id)+'/recent')
        .done(function(events){
          const out = document.getElementById('mc-recent-out');
          out.innerHTML = (events||[]).map(fmtEvent).join('');
          out.scrollTop = out.scrollHeight;
        })
        .fail(function(){ $('#mc-recent-out').text('Failed to load recent events'); });
    };
    $('#mc-recent-refresh').off('click').on('click', doLoad);
    doLoad();
  }


  function openStreamModal(id, url){
    const modalId = 'mc-stream-modal';
    const html = `
      <div class="modal" tabindex="-1" role="dialog" id="${modalId}">
        <div class="modal-dialog modal-lg" role="document">
          <div class="modal-content">
            <div class="modal-header"><h5 class="modal-title">Live Stream: ${id}</h5>
              <button type="button" class="close" data-dismiss="modal" aria-label="Close"><span aria-hidden="true">&times;</span></button>
            </div>
            <div class="modal-body"><pre id="mc-stream-output" style="max-height:50vh;overflow:auto;background:#111;color:#0f0;padding:10px;"></pre></div>
            <div class="modal-footer"><button type="button" class="btn btn-secondary" data-dismiss="modal">Close</button></div>
          </div>
        </div>
      </div>`;
    // ensure single instance
    $('#'+modalId).remove();
    $('body').append(html);
    $('#'+modalId).modal('show');


    const out = document.getElementById('mc-stream-output');
    try {
      const es = new EventSource(url);
      es.onmessage = function(ev){ try{ const obj = JSON.parse(ev.data); out.textContent += (JSON.stringify(obj)+"\n"); out.scrollTop = out.scrollHeight; } catch(e){ /* ignore */ } };
      $('#'+modalId).on('hidden.bs.modal', function(){ es.close(); $(this).remove(); });
    } catch(e) {
      out.textContent = 'Failed to connect to stream: '+e;
    }
  }


  function openEditModal(it){
    const mid = 'mc-edit-modal';
    const html = `
      <div class="modal" tabindex="-1" role="dialog" id="${mid}">
        <div class="modal-dialog" role="document">
          <div class="modal-content">
            <div class="modal-header"><h5 class="modal-title">Edit Multi-cast</h5>
              <button type="button" class="close" data-dismiss="modal" aria-label="Close"><span aria-hidden="true">&times;</span></button>
            </div>
            <div class="modal-body">
              <div class="form-group"><label>Name</label><input id="mc-edit-name" class="form-control" value="${escapeHtml(it.name||'')}"/></div>
              <div class="form-group"><label>Port</label><input id="mc-edit-port" type="number" class="form-control" value="${it.port}"/></div>
              <div class="form-row">
                <div class="form-group col-md-4"><label>Visible</label><br><input id="mc-edit-visible" type="checkbox" ${it.visible?'checked':''}/> Visible to all users</div>
                <div class="form-group col-md-4"><label>Enabled</label><br><input id="mc-edit-enabled" type="checkbox" ${it.enabled?'checked':''}/></div>
                <div class="form-group col-md-4"><label>Bidirectional</label><br><input id="mc-edit-bidir" type="checkbox" ${it.mode==='bidirectional'?'checked':''}/></div>
              </div>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-secondary" data-dismiss="modal">Cancel</button>
              <button type="button" class="btn btn-primary" id="mc-edit-save">Save</button>
            </div>
          </div>
        </div>
      </div>`;
    $('#'+mid).remove();
    $('body').append(html);
    $('#'+mid).modal('show');
    $('#mc-edit-save').off('click').on('click', function(){
      const payload = {
        name: ($('#mc-edit-name').val()||'').trim(),
        port: parseInt($('#mc-edit-port').val(), 10),
        mode: $('#mc-edit-bidir').is(':checked') ? 'bidirectional' : 'webhook',
        visible: $('#mc-edit-visible').is(':checked'),
        enabled: $('#mc-edit-enabled').is(':checked')
      };
      if (!payload.port || payload.port<1 || payload.port>65535) { alert('Invalid port'); return; }
      $.ajax({ url: '/api/multicast-tunnels/'+encodeURIComponent(it.id), method: 'PUT', contentType: 'application/json', data: JSON.stringify(payload) })
        .done(function(){ $('#'+mid).modal('hide'); loadMulticastView(); })
        .fail(function(xhr){ alert('Update failed: '+(xhr.responseText||xhr.statusText)); });
    });
    $('#'+mid).on('hidden.bs.modal', function(){ $(this).remove(); });
  }

  function escapeHtml(str){ return (''+str).replace(/[&<>"']/g, function(c){return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;','\'':'&#39;'}[c]);}); }
})();

