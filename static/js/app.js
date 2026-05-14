/* APICalls — Main JavaScript */

/* ── Modal helpers ───────────────────────────── */
function openModal(id) {
  document.getElementById(id).classList.add('open');
}
function closeModal(id) {
  document.getElementById(id).classList.remove('open');
}
// Close on overlay click
document.addEventListener('click', function(e) {
  if (e.target.classList.contains('modal-overlay')) {
    e.target.classList.remove('open');
  }
});

/* ── Populate edit modal ─────────────────────── */
function populateModal(modalId, data) {
  const modal = document.getElementById(modalId);
  if (!modal) return;
  Object.entries(data).forEach(([k, v]) => {
    const el = modal.querySelector('[name="' + k + '"]');
    if (!el) return;
    if (el.type === 'checkbox') { el.checked = !!v; }
    else { el.value = v; }
  });
}

/* ── KV Editor ───────────────────────────────── */
function kvEditorInit(containerId, hiddenId, data) {
  const container = document.getElementById(containerId);
  const hidden    = document.getElementById(hiddenId);
  let rows = Array.isArray(data) ? data : [];

  function render() {
    container.innerHTML = '';
    rows.forEach((row, i) => {
      const div = document.createElement('div');
      div.className = 'kv-row';
      div.innerHTML = `
        <input type="text" placeholder="Key"   value="${esc(row.key)}"   data-i="${i}" data-f="key">
        <input type="text" placeholder="Value" value="${esc(row.value)}" data-i="${i}" data-f="value">
        <button type="button" class="kv-del" data-cid="${containerId}" data-hid="${hiddenId}" data-idx="${i}">✕</button>`;
      container.appendChild(div);
    });
    container.querySelectorAll('input').forEach(inp => {
      inp.addEventListener('input', () => {
        rows[+inp.dataset.i][inp.dataset.f] = inp.value;
        sync();
      });
    });
    container.querySelectorAll('.kv-del').forEach(btn => {
      btn.addEventListener('click', () => {
        rows.splice(+btn.dataset.idx, 1);
        render();
      });
    });
    sync();
  }

  function sync() { hidden.value = JSON.stringify(rows); }

  // Store add-row function keyed by containerId
  window._kvAdd = window._kvAdd || {};
  window._kvAdd[containerId] = function() { rows.push({key:'',value:''}); render(); };
  render();
}

// Called from HTML: kvAddRow('some-id')
function kvAddRow(containerId) {
  if (window._kvAdd && window._kvAdd[containerId]) {
    window._kvAdd[containerId]();
  }
}

function esc(s) {
  return String(s || '').replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

/* ── Variable highlighting ───────────────────── */
function highlightVars(text) {
  return text.replace(/\{\{([^}]+)\}\}/g,
    '<span class="var-pill">{{$1}}</span>');
}

/* ── Technology auto-fill ────────────────────── */
function loadTechnology(selectEl, targetFormId) {
  const techId = selectEl.value;
  if (!techId) return;
  fetch('/api/technologies/' + techId)
    .then(r => r.json())
    .then(t => {
      const form = document.getElementById(targetFormId);
      if (!form) return;
      ['method','url','body'].forEach(f => {
        const el = form.querySelector('[name="' + f + '"]');
        if (el) el.value = t[f] || '';
      });
      // Determine KV editor IDs from what exists in the form
      const isEdit = targetFormId.includes('edit');
      const hEditorId  = isEdit ? 'edit-headers-editor'  : 'headers-editor';
      const hHiddenId  = isEdit ? 'edit-headers_json'    : 'headers_json';
      const cvEditorId = isEdit ? 'edit-cv-editor'       : 'custom-values-editor';
      const cvHiddenId = isEdit ? 'edit-custom_values_json' : 'custom_values_json';
      if (document.getElementById(hEditorId))
        kvEditorInit(hEditorId, hHiddenId, t.headers       || []);
      if (document.getElementById(cvEditorId))
        kvEditorInit(cvEditorId, cvHiddenId, t.custom_values || []);
    });
}

/* ── Unmask password ─────────────────────────── */
function toggleMask(btn, cellId) {
  const cell = document.getElementById(cellId);
  if (cell.dataset.masked === '1') {
    cell.textContent = cell.dataset.real;
    cell.dataset.masked = '0';
    btn.textContent = 'Hide';
  } else {
    cell.textContent = '••••••••';
    cell.dataset.masked = '1';
    btn.textContent = 'Show';
  }
}

/* ── Confirm delete ───────────────────────────── */
function confirmDelete(formId) {
  if (confirm('Are you sure you want to delete this item?')) {
    document.getElementById(formId).submit();
  }
}

/* ── Flash auto-dismiss ──────────────────────── */
document.addEventListener('DOMContentLoaded', function() {
  const flash = document.querySelector('.flash');
  if (flash) setTimeout(() => flash.style.opacity = '0', 5000);
});
