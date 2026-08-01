/* ============================================================
   公共布局渲染 — 顶栏 / 页脚 / 主题切换
   ============================================================ */

// 当前导航高亮
function navActive(activeName) {
  document.querySelectorAll('.nav a').forEach(a => {
    const name = a.getAttribute('data-nav');
    a.classList.toggle('active', name === activeName);
  });
}

// 渲染顶部导航
function renderTopbar(activeName) {
  const topbar = document.getElementById('topbar');
  if (!topbar) return;
  const token = localStorage.getItem('token');
  const admin = getStoredAdmin();
  // 用户区:未登录 → 登录/注册;已登录 → 昵称 + 个人中心/退出
  const userArea = token
    ? `<a href="me.html" data-nav="me"><i>00</i> ${esc(admin?.nickname || '我的')}</a>
       <a href="javascript:;" onclick="doLogout()" data-nav="logout">退出</a>`
    : `<a href="login.html" data-nav="login"><i>00</i> 登录</a>
       <a href="register.html" data-nav="register"><i>00</i> 注册</a>`;
  const adminArea = (admin && admin.role >= 2)
    ? `<a href="admin/dashboard.html" data-nav="admin"><i>◎</i> 管理</a>` : '';
  topbar.innerHTML = `
    <div class="grid">
      <a class="logo" href="index.html"><span class="tag">◎</span>BLOG<sup>®</sup></a>
      <nav class="nav">
        <a href="index.html" data-nav="home"><i>01</i> 首页</a>
        <a href="articles.html" data-nav="archive"><i>02</i> 文章</a>
        <a href="tags.html" data-nav="tags"><i>03</i> 标签</a>
        ${adminArea}
        ${userArea}
        <button class="theme-btn" title="切换夜间模式">☾</button>
      </nav>
    </div>
  `;
  document.querySelector('.theme-btn').addEventListener('click', () => {
    applyTheme(!document.body.classList.contains('dark'));
  });
  navActive(activeName);
}

// 读取登录用户(本地缓存)
function getStoredAdmin() {
  try { return JSON.parse(localStorage.getItem('admin') || 'null'); }
  catch (e) { return null; }
}

// 退出登录
function doLogout() {
  localStorage.removeItem('token');
  localStorage.removeItem('admin');
  toast('已退出登录');
  setTimeout(() => location.reload(), 500);
}

// 渲染页脚
function renderFooter() {
  const footer = document.getElementById('footer');
  if (!footer) return;
  footer.innerHTML = `
    <div class="grid">
      <div class="copy">© 2026 <b>BLOG-SYSTEM</b> — Built with Go + Gin + Swiss</div>
      <div class="links">
        <a href="about.html">关于</a>
        <a href="/rss.xml" target="_blank">RSS</a>
        <a href="register.html">注册</a>
        <a href="admin/login.html">后台</a>
      </div>
    </div>
  `;
}

// 渲染每日一言(页面有 .quote 时)
async function renderQuote() {
  const quoteEl = document.querySelector('.quote blockquote');
  if (!quoteEl) return;
  try {
    const data = await api.get('/api/quote/random');
    quoteEl.innerHTML = `<p>${esc(data.content)}</p><cite>—— ${esc(data.author)}</cite>`;
  } catch (e) {
    quoteEl.innerHTML = `<p>把每一个今天都当作新的开始。</p><cite>—— 佚名</cite>`;
  }
}

// 站点信息填充页脚社交链接
async function renderSiteInfo() {
  try {
    const site = await api.get('/api/site');
    if (site.github) { const el = document.getElementById('ft-github'); if (el) el.href = site.github; }
    if (site.email) { const el = document.getElementById('ft-email'); if (el) el.href = 'mailto:' + site.email; }
    const titleEl = document.querySelector('.brand-name-inline');
    if (titleEl && site.title) titleEl.textContent = site.title;
  } catch (e) { /* 静默,用默认值 */ }
}

// 初始化页面公共部分
async function initPage(activeName) {
  initTheme();
  renderTopbar(activeName);
  renderFooter();
  renderQuote();
  renderSiteInfo();
}
