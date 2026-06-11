// 简易 toast 通知,替代浏览器原生 alert/confirm,体验更友好
function toast(msg, isError) {
  const el = document.createElement('div');
  el.className = 'toast' + (isError ? ' toast-error' : '');
  el.textContent = msg;
  document.body.appendChild(el);
  // 触发动画
  requestAnimationFrame(() => el.classList.add('toast-show'));
  setTimeout(() => {
    el.classList.remove('toast-show');
    setTimeout(() => el.remove(), 300);
  }, 3500);
}

function confirmDialog(msg) {
  // 使用原生 confirm 保持同步语义;生产可替换为自定义 modal
  return new Promise(resolve => resolve(window.confirm(msg)));
}

async function readError(r, fallback) {
  try {
    const t = await r.text();
    return t || fallback;
  } catch (_) {
    return fallback;
  }
}

async function refreshOne(name) {
  const r = await fetch(`/admin/api/sources/${encodeURIComponent(name)}/refresh`, { method: 'POST' });
  if (!r.ok) {
    toast('刷新失败:' + (await readError(r, '未知错误')), true);
    return;
  }
  location.reload();
}

async function refreshAll() {
  const r = await fetch('/admin/api/refresh', { method: 'POST' });
  if (!r.ok) {
    toast('全部刷新失败:' + (await readError(r, '未知错误')), true);
    return;
  }
  location.reload();
}

async function deleteOne(name) {
  const ok = await confirmDialog(`确定要删除源「${name}」吗?\n此操作会同时删除该源的本地缓存,无法恢复。`);
  if (!ok) return;
  const r = await fetch(`/admin/api/sources/${encodeURIComponent(name)}`, { method: 'DELETE' });
  if (!r.ok) {
    toast('删除失败:' + (await readError(r, '未知错误')), true);
    return;
  }
  location.reload();
}

function buildBody(f) {
  return {
    name: f.name.value,
    type: f.type.value,
    url: f.url.value,
    ref: f.ref.value,
    enabled: f.enabled.checked,
  };
}

async function submitNew(e) {
  e.preventDefault();
  const f = e.target;
  const r = await fetch('/admin/api/sources', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(buildBody(f)),
  });
  if (!r.ok) {
    toast('添加失败:' + (await readError(r, '未知错误')), true);
    return false;
  }
  toast('添加成功,正在拉取...');
  location.href = '/admin/';
  return false;
}

async function submitEdit(e, name) {
  e.preventDefault();
  const f = e.target;
  const r = await fetch(`/admin/api/sources/${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(buildBody(f)),
  });
  if (!r.ok) {
    toast('保存失败:' + (await readError(r, '未知错误')), true);
    return false;
  }
  toast('保存成功');
  location.href = '/admin/';
  return false;
}

// 当类型切换时,自动更新 URL 输入框的占位符
function updateUrlHint() {
  const sel = document.getElementById('type-select');
  if (!sel) return;
  const urlInput = document.querySelector('input[name="url"]');
  if (!urlInput) return;
  if (sel.value === 'git') {
    urlInput.placeholder = 'https://github.com/owner/repo.git';
  } else {
    urlInput.placeholder = 'https://internal.example.com/marketplace.json';
  }
}
