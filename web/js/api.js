/* ============================================================
   API 封装 — 统一请求 / 鉴权 / 错误处理
   基于接口文档契约:{ code, message, data }
   ============================================================ */

// 统一请求
async function request(method, url, data) {
  const opts = { method, headers: {} };
  const token = localStorage.getItem('token');
  if (token) opts.headers['Authorization'] = 'Bearer ' + token;

  if (data !== undefined) {
    if (data instanceof FormData) {
      opts.body = data; // 不设 Content-Type,浏览器自动带 boundary
    } else {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(data);
    }
  }

  let res;
  try {
    res = await fetch(url, opts);
  } catch (e) {
    throw new Error('网络请求失败,请检查后端是否启动');
  }

  let json;
  try {
    json = await res.json();
  } catch (e) {
    throw new Error('服务返回异常(' + res.status + ')');
  }

  // 业务 code 判断
  if (json.code === 401) {
    localStorage.removeItem('token');
    localStorage.removeItem('admin');
    if (location.pathname.includes('admin')) location.href = 'login.html';
  }
  if (json.code !== 200) {
    throw new Error(json.message || '请求失败');
  }
  return json.data;
}

// 快捷方法
const api = {
  get: (url) => request('GET', url),
  post: (url, data) => request('POST', url, data),
  put: (url, data) => request('PUT', url, data),
  del: (url) => request('DELETE', url),
};

/* ---------- 格式化工具 ---------- */
function fmtDate(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}.${p(d.getMonth() + 1)}.${p(d.getDate())}`;
}

function fmtDateTime(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}.${p(d.getMonth() + 1)}.${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

function fmtNum(n) {
  if (n >= 10000) return (n / 10000).toFixed(1) + 'w';
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
  return String(Math.round(n));
}

function fmtMinutes(content) {
  if (!content) return 1;
  const words = content.replace(/<[^>]*>/g, ' ').replace(/\s+/g, '').length;
  return Math.max(1, Math.ceil(words / 300));
}

function esc(str) {
  return String(str ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/* ---------- Toast 提示 ---------- */
function toast(msg, isErr) {
  let el = document.querySelector('.toast');
  if (!el) {
    el = document.createElement('div');
    el.className = 'toast';
    document.body.appendChild(el);
  }
  el.textContent = msg;
  el.className = 'toast show' + (isErr ? ' err' : '');
  clearTimeout(el._t);
  el._t = setTimeout(() => el.classList.remove('show'), 2600);
}

/* ---------- 通用查询参数 ---------- */
function getParam(name) {
  return new URLSearchParams(location.search).get(name);
}

/* ---------- 主题切换 ---------- */
function applyTheme(dark) {
  document.body.classList.toggle('dark', dark);
  localStorage.setItem('theme', dark ? 'dark' : 'light');
  const btn = document.querySelector('.theme-btn');
  if (btn) btn.textContent = dark ? '☀' : '☾';
}
function initTheme() {
  const saved = localStorage.getItem('theme');
  applyTheme(saved === 'dark');
}
