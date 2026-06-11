async function refreshOne(name) {
  const r = await fetch(`/admin/api/sources/${encodeURIComponent(name)}/refresh`, { method: 'POST' });
  if (!r.ok) { alert('refresh failed: ' + (await r.text())); return; }
  location.reload();
}
async function refreshAll() {
  const r = await fetch('/admin/api/refresh', { method: 'POST' });
  if (!r.ok) { alert('refresh all failed: ' + (await r.text())); return; }
  location.reload();
}
async function deleteOne(name) {
  if (!confirm(`Delete source ${name}?`)) return;
  const r = await fetch(`/admin/api/sources/${encodeURIComponent(name)}`, { method: 'DELETE' });
  if (!r.ok) { alert('delete failed: ' + (await r.text())); return; }
  location.reload();
}
async function submitNew(e) {
  e.preventDefault();
  const f = e.target;
  const body = {
    name: f.name.value,
    type: f.type.value,
    url: f.url.value,
    ref: f.ref.value,
    enabled: f.enabled.checked,
  };
  const r = await fetch('/admin/api/sources', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!r.ok) { alert('add failed: ' + (await r.text())); return false; }
  location.href = '/admin/';
  return false;
}
async function submitEdit(e, name) {
  e.preventDefault();
  const f = e.target;
  const body = {
    name: f.name.value,
    type: f.type.value,
    url: f.url.value,
    ref: f.ref.value,
    enabled: f.enabled.checked,
  };
  const r = await fetch(`/admin/api/sources/${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!r.ok) { alert('update failed: ' + (await r.text())); return false; }
  location.href = '/admin/';
  return false;
}
