/* ============================================================
   文章列表页 — 分类/标签/搜索/排序/分页
   ============================================================ */

initPage('archive').then(initList);

let curPage = 1;
const pageSize = 10;

function initList() {
  // 读取 URL 参数
  const categoryId = getParam('categoryId');
  const tag = getParam('tag');
  const keyword = getParam('keyword');
  const sort = getParam('sort') || 'latest';
  curPage = parseInt(getParam('page') || '1', 10);

  // 页面标题
  const title = document.getElementById('page-title');
  const sub = document.getElementById('page-sub');
  if (categoryId) { title.textContent = '分类文章'; }
  else if (tag) { title.textContent = '标签: ' + tag; sub.textContent = 'TAG: ' + tag.toUpperCase(); }
  else if (keyword) { title.textContent = '搜索: ' + keyword; sub.textContent = 'SEARCH RESULT'; }
  else { title.textContent = '文章归档'; sub.textContent = 'ALL ARTICLES'; }

  // 搜索框
  const input = document.getElementById('search-input');
  if (keyword) input.value = keyword;
  document.getElementById('search-form').addEventListener('submit', (e) => {
    e.preventDefault();
    const q = input.value.trim();
    if (q) location.href = 'articles.html?keyword=' + encodeURIComponent(q);
    else location.href = 'articles.html';
  });

  // 排序按钮
  document.querySelectorAll('.sort-btn').forEach(btn => {
    btn.style.fontWeight = btn.dataset.sort === sort ? '700' : '400';
    btn.style.color = btn.dataset.sort === sort ? 'var(--red)' : 'var(--gray-a)';
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      const p = new URLSearchParams(location.search);
      p.set('sort', btn.dataset.sort);
      p.delete('page');
      location.href = 'articles.html?' + p.toString();
    });
  });

  loadCats(categoryId, tag);
  loadArticles();
}

async function loadCats(categoryId, tag) {
  const wrap = document.getElementById('filter-cats');
  try {
    const cats = await api.get('/api/categories');
    wrap.innerHTML = `
      <a class="cat" href="articles.html" style="--cat-bar:var(--ink)">
        <span class="idx">ALL</span><span class="name">全部</span><span class="cnt">${cats.reduce((s,c)=>s+(c.articleCount||0),0)} posts</span><span class="bar"></span>
      </a>
      ${cats.map((c, i) => `
        <a class="cat" href="articles.html?categoryId=${c.id}" style="--cat-bar:${['var(--red)','var(--ink)','var(--gray-6)','var(--red)'][i%4]}">
          <span class="idx">${String(i+1).padStart(2,'0')}</span><span class="name">${esc(c.name)}</span><span class="cnt">${c.articleCount ?? 0} posts</span><span class="bar"></span>
        </a>`).join('')}`;
    // 高亮当前分类
    if (categoryId) {
      wrap.querySelectorAll('.cat').forEach(a => {
        if (new URLSearchParams(a.href.split('?')[1]).get('categoryId') === categoryId) {
          a.style.background = 'var(--red)'; a.style.color = '#fff'; a.style.borderColor = 'var(--red)';
        }
      });
    }
  } catch (e) {
    wrap.innerHTML = '<div class="empty" style="grid-column:1/13;padding:10px 0">分类加载失败</div>';
  }
}

async function loadArticles() {
  const wrap = document.getElementById('dir-list');
  wrap.innerHTML = '<div class="loading" style="grid-column:1/13">加载中…</div>';
  try {
    const p = new URLSearchParams(location.search);
    p.set('page', curPage);
    p.set('pageSize', pageSize);
    if (!p.get('sort')) p.set('sort', 'latest');
    const data = await api.get('/api/articles?' + p.toString());

    document.getElementById('dir-total').textContent = data.total;
    if (!data.list || !data.list.length) {
      wrap.innerHTML = '<div class="empty" style="grid-column:1/13">没有找到文章</div>';
      renderPagination(0);
      return;
    }
    wrap.innerHTML = data.list.map((a, i) => `
      <a class="entry" href="article.html?id=${a.id}">
        <span class="num">${String((curPage-1)*pageSize + i + 1).padStart(3,'0')}</span>
        <span class="title"><span>${esc(a.title)}</span><span class="arrow">→</span></span>
        <span class="entry-cat">${esc(a.category?.name || '未分类')}</span>
        <span class="date">${fmtDate(a.createdAt)}<span class="views">${fmtNum(a.viewCount)} views</span></span>
      </a>`).join('');
    renderPagination(data.total);
  } catch (e) {
    wrap.innerHTML = `<div class="empty" style="grid-column:1/13">加载失败: ${esc(e.message)}</div>`;
  }
}

function renderPagination(total) {
  const wrap = document.getElementById('pagination');
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const p = new URLSearchParams(location.search);

  const btn = (label, page, cls = '') => {
    const q = new URLSearchParams(location.search);
    q.set('page', page);
    return `<a href="articles.html?${q.toString()}" class="${cls}">${label}</a>`;
  };

  let html = '';
  html += btn('←', Math.max(1, curPage - 1), curPage <= 1 ? 'disabled' : '');
  // 页码窗口
  const start = Math.max(1, curPage - 2);
  const end = Math.min(totalPages, start + 4);
  for (let i = start; i <= end; i++) {
    html += i === curPage ? `<span class="cur">${i}</span>` : btn(i, i);
  }
  html += btn('→', Math.min(totalPages, curPage + 1), curPage >= totalPages ? 'disabled' : '');
  wrap.innerHTML = html;
}
