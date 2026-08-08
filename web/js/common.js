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
      <a class="logo" href="index.html"><span class="logo-mark"><svg width="22" height="22" viewBox="0 0 24 24" fill="none"><rect x="2.5" y="2.5" width="19" height="19" rx="5" stroke="currentColor" stroke-width="2"/><circle cx="12" cy="12" r="3.5" fill="#E53012"/></svg></span>BLOG<sup>®</sup></a>
      <nav class="nav" id="nav-menu">
        <a href="index.html" data-nav="home"><i>01</i> 首页</a>
        <a href="articles.html" data-nav="archive"><i>02</i> 文章</a>
        <a href="tags.html" data-nav="tags"><i>03</i> 标签</a>
        ${adminArea}
        ${userArea}
        <button class="search-trigger" id="search-trigger" title="搜索 (快捷键 /)" aria-label="搜索">⌕</button>
        <button class="theme-btn" title="切换夜间模式">☾</button>
      </nav>
      <button class="nav-toggle" id="nav-toggle" aria-label="菜单" title="展开导航">
        <span></span><span></span><span></span>
      </button>
    </div>
  `;
  document.querySelector('.theme-btn').addEventListener('click', () => {
    applyTheme(!document.body.classList.contains('dark'));
  });
  // 全局搜索弹层：点击图标或按 / 键唤起
  initSearchOverlay();
  // 移动端汉堡菜单：点击展开/收起导航
  const toggle = document.getElementById('nav-toggle');
  const menu = document.getElementById('nav-menu');
  if (toggle && menu) {
    toggle.addEventListener('click', () => {
      const open = menu.classList.toggle('open');
      toggle.classList.toggle('active', open);
      toggle.setAttribute('aria-expanded', open);
    });
    // 点击导航项后自动收起（移动端体验）
    menu.querySelectorAll('a').forEach(a => {
      a.addEventListener('click', () => {
        menu.classList.remove('open');
        toggle.classList.remove('active');
      });
    });
  }
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
        <a href="/api/rss.xml" target="_blank">RSS</a>
        <a href="register.html">注册</a>
        <a href="admin/login.html">后台</a>
      </div>
    </div>
  `;
}

// 渲染浮动导航组：返回按钮（文章页）+ 返回顶部（全局）
function renderBackToTop() {
  if (document.getElementById('float-nav')) return;
  const nav = document.createElement('div');
  nav.id = 'float-nav';
  nav.className = 'float-nav';
  // 返回按钮：只有文章详情页显示
  const isArticle = location.pathname.includes('/article.html') || !!document.getElementById('article-content');
  nav.innerHTML = ''
    + (isArticle ? '<button class="fn-back" title="返回文章列表" aria-label="返回"><span>←</span></button>' : '')
    + '<button class="fn-top" title="返回顶部" aria-label="返回顶部">↑</button>';
  document.body.appendChild(nav);

  const backBtn = nav.querySelector('.fn-back');
  const topBtn = nav.querySelector('.fn-top');

  const onScroll = () => {
    const show = window.scrollY > 400;
    nav.classList.toggle('show', show);
  };
  window.addEventListener('scroll', onScroll, { passive: true });

  if (topBtn) {
    topBtn.addEventListener('click', () => {
      window.scrollTo({ top: 0, behavior: 'smooth' });
    });
  }
  if (backBtn) {
    backBtn.addEventListener('click', () => {
      if (document.referrer && document.referrer.includes(location.host)) {
        history.back();
      } else {
        location.href = 'articles.html';
      }
    });
  }
  onScroll();
}

// 数字滚动动画：从当前值平滑增长到目标值
// el 目标元素，target 目标值，duration 动画时长(ms)，formatter 可选格式化函数
function animateNumber(el, target, duration = 1200, formatter) {
  if (!el) return;
  const start = 0;
  const startTime = performance.now();
  const fmt = formatter || ((n) => String(Math.round(n)));
  const step = (now) => {
    const t = Math.min(1, (now - startTime) / duration);
    // easeOutCubic：先快后慢，数字"刹住"的观感更自然
    const eased = 1 - Math.pow(1 - t, 3);
    el.textContent = fmt(start + (target - start) * eased);
    if (t < 1) requestAnimationFrame(step);
    else el.textContent = fmt(target);
  };
  requestAnimationFrame(step);
}

// 图片懒加载：给所有 data-src 的 img 元素延迟加载
// 用 IntersectionObserver（比 scroll 监听性能好），不支持时回退立即加载
function initLazyImages() {
  const imgs = document.querySelectorAll('img[data-src]');
  if (!imgs.length) return;
  if (!('IntersectionObserver' in window)) {
    imgs.forEach(img => { if (img.dataset.src) img.src = img.dataset.src; });
    return;
  }
  const io = new IntersectionObserver((entries, obs) => {
    entries.forEach(e => {
      if (e.isIntersecting) {
        const img = e.target;
        if (img.dataset.src) {
          img.src = img.dataset.src;
          img.removeAttribute('data-src');
        }
        obs.unobserve(img);
      }
    });
  }, { rootMargin: '50px' });
  imgs.forEach(img => io.observe(img));
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

// ============================================================
// 全局搜索弹层：点击搜索图标或按 / 键打开
// 支持：实时搜索建议、搜索历史(localStorage)、热门标签快捷入口
// ============================================================
function initSearchOverlay() {
  const trigger = document.getElementById('search-trigger');
  if (!trigger) return;

  // 构建 DOM（只构建一次，复用）
  let overlay = document.getElementById('search-overlay');
  if (!overlay) {
    overlay = document.createElement('div');
    overlay.id = 'search-overlay';
    overlay.className = 'search-overlay';
    overlay.innerHTML = `
      <div class="search-modal">
        <div class="search-modal-bar">
          <span class="search-modal-ico">⌕</span>
          <input type="text" id="search-modal-input" placeholder="搜索文章标题 / 正文…" autocomplete="off">
          <kbd>Esc</kbd>
        </div>
        <div class="search-modal-body" id="search-modal-body">
          <div class="search-hint">
            <div class="search-hint-section" id="search-history-section">
              <div class="search-hint-title">最近搜索 <button class="search-hint-clear" onclick="clearSearchHistory()">清空</button></div>
              <div class="search-tags" id="search-history-list"></div>
            </div>
            <div class="search-hint-section">
              <div class="search-hint-title">热门标签</div>
              <div class="search-tags" id="search-hot-tags"></div>
            </div>
          </div>
          <div class="search-results" id="search-results" style="display:none;"></div>
        </div>
      </div>`;
    document.body.appendChild(overlay);

    const input = overlay.querySelector('#search-modal-input');

    // 输入实时搜索（防抖 250ms：避免每按一个键都打一次接口）
    let debounceTimer = null;
    input.addEventListener('input', () => {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => doSearchSuggest(input.value.trim()), 250);
    });

    // 回车 → 跳完整搜索页
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const q = input.value.trim();
        if (q) {
          saveSearchHistory(q);
          location.href = 'search.html?q=' + encodeURIComponent(q);
          closeSearchOverlay();
        }
      }
    });

    // 点击遮罩关闭
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) closeSearchOverlay();
    });

    // Esc 关闭（input 内监听即可，因为打开后焦点在 input）
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') closeSearchOverlay();
    });
  }

  // 打开
  trigger.addEventListener('click', openSearchOverlay);

  // 全局快捷键 / ：非输入框内按 / 唤起搜索
  document.addEventListener('keydown', (e) => {
    if (e.key !== '/') return;
    const tag = (e.target.tagName || '').toLowerCase();
    if (tag === 'input' || tag === 'textarea' || e.target.isContentEditable) return;
    e.preventDefault();
    openSearchOverlay();
  });
}

function openSearchOverlay() {
  const overlay = document.getElementById('search-overlay');
  if (!overlay) return;
  overlay.classList.add('show');
  document.body.style.overflow = 'hidden'; // 锁背景滚动
  const input = overlay.querySelector('#search-modal-input');
  // 渲染历史 + 热门标签
  renderSearchHistory();
  loadHotTagsForSearch();
  // 重置结果区
  overlay.querySelector('#search-results').style.display = 'none';
  overlay.querySelector('.search-hint').style.display = '';
  input.value = '';
  setTimeout(() => input.focus(), 50);
}

function closeSearchOverlay() {
  const overlay = document.getElementById('search-overlay');
  if (!overlay) return;
  overlay.classList.remove('show');
  document.body.style.overflow = '';
}

// 搜索建议：实时拉取前 5 条匹配结果
async function doSearchSuggest(q) {
  const body = document.getElementById('search-modal-body');
  const resultsEl = document.getElementById('search-results');
  const hintEl = body.querySelector('.search-hint');
  if (!q) {
    resultsEl.style.display = 'none';
    hintEl.style.display = '';
    return;
  }
  try {
    const data = await api.get('/api/articles?keyword=' + encodeURIComponent(q) + '&page=1&pageSize=5&sort=latest');
    hintEl.style.display = 'none';
    resultsEl.style.display = '';
    if (!data.list || !data.list.length) {
      resultsEl.innerHTML = '<div class="search-empty">没有匹配的文章，按回车查看全部结果 →</div>';
      return;
    }
    resultsEl.innerHTML = `
      <div class="search-results-count">约 ${data.total} 条结果</div>
      ${data.list.map(a => `
        <a class="search-result-item" href="article.html?id=${a.id}">
          <span class="sr-cat">${esc(a.category?.name || '未分类')}</span>
          <span class="sr-title">${highlightKeyword(esc(a.title), q)}</span>
          <span class="sr-date">${fmtDate(a.createdAt)}</span>
        </a>`).join('')}
      <div class="search-results-more" onclick="document.getElementById('search-modal-input').dispatchEvent(new KeyboardEvent('keydown',{key:'Enter'}))">查看全部结果 →</div>`;
  } catch (e) {
    resultsEl.innerHTML = '<div class="search-empty">搜索失败，请重试</div>';
  }
}

// 关键词高亮：在已转义的文本里给匹配段加 <mark>
function highlightKeyword(escapedText, keyword) {
  if (!keyword) return escapedText;
  // 转义关键词里的正则特殊字符，避免注入
  const safe = keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return escapedText.replace(new RegExp(safe, 'gi'), (m) => `<mark>${m}</mark>`);
}

// ============ 搜索历史（localStorage） ============
function getSearchHistory() {
  try { return JSON.parse(localStorage.getItem('search_history') || '[]'); }
  catch (e) { return []; }
}

function saveSearchHistory(q) {
  if (!q) return;
  let list = getSearchHistory();
  // 去重（大小写不敏感），放到最前
  list = list.filter(item => item.toLowerCase() !== q.toLowerCase());
  list.unshift(q);
  // 只保留最近 8 条
  if (list.length > 8) list = list.slice(0, 8);
  localStorage.setItem('search_history', JSON.stringify(list));
}

function clearSearchHistory() {
  localStorage.removeItem('search_history');
  renderSearchHistory();
}

function renderSearchHistory() {
  const wrap = document.getElementById('search-history-list');
  const section = document.getElementById('search-history-section');
  if (!wrap) return;
  const list = getSearchHistory();
  if (!list.length) {
    section.style.display = 'none';
    return;
  }
  section.style.display = '';
  wrap.innerHTML = list.map(q =>
    `<a class="tag-item" href="search.html?q=${encodeURIComponent(q)}"># ${esc(q)}</a>`
  ).join('');
}

// 热门标签：拉取标签列表，按文章数排序取前 10
async function loadHotTagsForSearch() {
  const wrap = document.getElementById('search-hot-tags');
  if (!wrap) return;
  if (wrap.dataset.loaded) return; // 只加载一次
  try {
    const tags = await api.get('/api/tags');
    const sorted = (tags || []).sort((a, b) => (b.articleCount || 0) - (a.articleCount || 0)).slice(0, 10);
    wrap.innerHTML = sorted.map(t =>
      `<a class="tag-item" href="articles.html?tag=${encodeURIComponent(t.name)}">${esc(t.name)} <span class="n">${t.articleCount || 0}</span></a>`
    ).join('') || '<span class="search-empty">暂无标签</span>';
    wrap.dataset.loaded = '1';
  } catch (e) {
    wrap.innerHTML = '<span class="search-empty">加载失败</span>';
  }
}

// ============================================================
// 图片灯箱：点击文章内图片放大查看
// 在 article.js 的 renderArticle 后调用 initLightbox()
// ============================================================
function initLightbox() {
  const content = document.getElementById('article-content');
  if (!content) return;
  const imgs = content.querySelectorAll('img');
  if (!imgs.length) return;

  // 构建灯箱 DOM（全局只一个）
  let lb = document.getElementById('lightbox');
  if (!lb) {
    lb = document.createElement('div');
    lb.id = 'lightbox';
    lb.className = 'lightbox';
    lb.innerHTML = `
      <button class="lightbox-close" aria-label="关闭">×</button>
      <button class="lightbox-prev" aria-label="上一张">‹</button>
      <img class="lightbox-img" alt="预览大图">
      <button class="lightbox-next" aria-label="下一张">›</button>
      <div class="lightbox-caption"></div>`;
    document.body.appendChild(lb);

    lb.addEventListener('click', (e) => {
      if (e.target === lb || e.target.classList.contains('lightbox-close')) closeLightbox();
    });
    lb.querySelector('.lightbox-prev').addEventListener('click', (e) => { e.stopPropagation(); lightboxNav(-1); });
    lb.querySelector('.lightbox-next').addEventListener('click', (e) => { e.stopPropagation(); lightboxNav(1); });
    document.addEventListener('keydown', (e) => {
      if (!lb.classList.contains('show')) return;
      if (e.key === 'Escape') closeLightbox();
      else if (e.key === 'ArrowLeft') lightboxNav(-1);
      else if (e.key === 'ArrowRight') lightboxNav(1);
    });
  }

  // 收集所有图片到数组，方便上一张/下一张切换
  const list = Array.from(imgs).map(img => ({
    src: img.src,
    alt: img.alt || img.title || '',
  }));

  imgs.forEach((img, i) => {
    img.style.cursor = 'zoom-in';
    img.addEventListener('click', () => openLightbox(i, list));
  });
}

let lightboxIndex = 0;
let lightboxList = [];

function openLightbox(index, list) {
  lightboxIndex = index;
  lightboxList = list;
  const lb = document.getElementById('lightbox');
  updateLightbox();
  lb.classList.add('show');
  document.body.style.overflow = 'hidden';
}

function updateLightbox() {
  const lb = document.getElementById('lightbox');
  const img = lb.querySelector('.lightbox-img');
  const caption = lb.querySelector('.lightbox-caption');
  const item = lightboxList[lightboxIndex];
  if (!item) return;
  img.src = item.src;
  img.alt = item.alt;
  caption.textContent = item.alt ? `${item.alt}（${lightboxIndex + 1}/${lightboxList.length}）` : `${lightboxIndex + 1}/${lightboxList.length}`;
  // 只有一张时隐藏左右按钮
  lb.querySelector('.lightbox-prev').style.display = lightboxList.length > 1 ? '' : 'none';
  lb.querySelector('.lightbox-next').style.display = lightboxList.length > 1 ? '' : 'none';
}

function lightboxNav(dir) {
  lightboxIndex = (lightboxIndex + dir + lightboxList.length) % lightboxList.length;
  updateLightbox();
}

function closeLightbox() {
  const lb = document.getElementById('lightbox');
  if (lb) lb.classList.remove('show');
  document.body.style.overflow = '';
}

// 初始化页面公共部分
async function initPage(activeName) {
  initTheme();
  renderTopbar(activeName);
  renderFooter();
  renderQuote();
  renderSiteInfo();
  renderBackToTop();
  // 后台页面是独立布局，不在 topbar 体系内，跳过懒加载
  if (!location.pathname.includes('/admin/')) {
    initLazyImages();
    loadBgFlow(); // 前台页面才启用流动线条背景
  }
}

// 动态加载背景流动线条脚本（前台所有页面共享，避免逐页改 <script>）
function loadBgFlow() {
  if (document.getElementById('bg-flow-script')) return;
  const s = document.createElement('script');
  s.id = 'bg-flow-script';
  s.src = 'js/bg-flow.js?v=5';
  document.body.appendChild(s);
}
