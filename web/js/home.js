/* ============================================================
   首页逻辑 — 头条精选 + 分类索引 + 文章目录 + 站点统计
   ============================================================ */

initPage('home').then(loadHome);

async function loadHome() {
  try {
    // 站点统计
    const site = await api.get('/api/site');
    if (site.stats) {
      // 数字滚动动画：比直接显示更生动（animateNumber 在 common.js）
      const ac = site.stats.articleCount ?? 0;
      const vc = site.stats.viewCount ?? 0;
      // 文章数小，整数显示；浏览量可能很大，用 fmtNum 格式化（带 k/w）
      animateNumber(document.getElementById('stat-articles'), ac, 900);
      animateNumber(document.getElementById('stat-views'), vc, 1400, fmtNum);
    }
    if (site.title) document.getElementById('hero-brand').textContent = '● ' + site.title + ' · EST. ' + (site.stats?.foundYear ?? 2019);

    // 分类索引
    const cats = await api.get('/api/categories');
    renderCats(cats);

    // 最新文章(取前 6 篇)
    const data = await api.get('/api/articles?page=1&pageSize=6&sort=latest');
    renderFeature(data.list);
    renderDir(data.list, data.total);
  } catch (e) {
    toast(e.message, true);
    document.getElementById('dir-list').innerHTML = '<div class="empty">文章加载失败: ' + esc(e.message) + '</div>';
  }
}

function renderCats(cats) {
  const wrap = document.getElementById('cat-list');
  document.getElementById('cats-count').textContent = '01—' + String(cats.length).padStart(2, '0');
  wrap.innerHTML = cats.map((c, i) => `
    <a class="cat" href="articles.html?categoryId=${c.id}">
      <span class="idx">${String(i + 1).padStart(2, '0')}</span>
      <span class="name">${esc(c.name)}</span>
      <span class="cnt">${c.articleCount ?? 0} posts</span>
      <span class="bar"></span>
    </a>
  `).join('');
  if (!cats.length) wrap.innerHTML = '<div class="empty" style="grid-column:1/13">暂无分类</div>';
}

function renderFeature(list) {
  const sec = document.getElementById('feature-sec');
  if (!list || !list.length) {
    sec.innerHTML = '<div class="empty" style="grid-column:1/13">暂无文章</div>';
    return;
  }
  const a = list[0];
  const title = esc(a.title);
  // 标题做两行,最后一段压红下划线
  const len = Math.ceil(title.length / 2);
  const t1 = title.slice(0, len);
  const t2 = title.slice(len);

  sec.innerHTML = `
    <div class="feature-label">
      <span class="flag">Featured · 头条精选</span>
      <span class="vol">VOL. 01</span>
    </div>
    <h2 class="feature-title">
      <span class="ln">${t1}</span>
      <span class="ln">${t2}<span class="red-line"></span></span>
    </h2>
    <div class="feature-foot">
      <div class="feature-meta">
        <span>${esc(a.author?.nickname || '作者')}</span><span class="sep">·</span>
        <span>${fmtDate(a.createdAt)}</span><span class="sep">·</span>
        <span>${esc(a.category?.name || '未分类')}</span><span class="sep">·</span>
        <span>阅读 ${fmtMinutes(a.summary)} 分钟</span>
      </div>
      <a class="feature-link" href="article.html?id=${a.id}">阅读全文 <span class="arrow">→</span></a>
    </div>
  `;
}

function renderDir(list, total) {
  document.getElementById('dir-total').textContent = total ?? list.length;
  const wrap = document.getElementById('dir-list');
  if (!list || !list.length) {
    wrap.innerHTML = '<div class="empty" style="grid-column:1/13">暂无文章</div>';
    return;
  }
  wrap.innerHTML = list.map((a, i) => `
    <a class="entry" href="article.html?id=${a.id}">
      <span class="num">${String(i + 1).padStart(3, '0')}</span>
      <span class="title"><span>${esc(a.title)}</span><span class="arrow">→</span></span>
      <span class="cat">${esc(a.category?.name || '未分类')}</span>
      <span class="date">${fmtDate(a.createdAt)}<span class="views">${fmtNum(a.viewCount)} views</span></span>
    </a>
  `).join('');
}

// 滚动字幕(用分类拼接)
setTimeout(() => {
  api.get('/api/categories').then(cats => {
    const track = document.getElementById('marquee-track');
    const names = cats.length ? cats.map(c => c.name) : ['Go Backend', 'MySQL Index', 'GORM', 'Gin', 'Swiss Design'];
    const items = [...names, ...names].map(n => `<span>${esc(n)}</span>`).join('');
    track.innerHTML = items;
  }).catch(() => {});
}, 0);
