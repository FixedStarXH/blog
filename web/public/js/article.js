/* ============================================================
   文章详情页 — 正文/TOC/浏览量/点赞/评论/上一篇下一篇/相关
   支持:私密文章解锁、夜间模式、阅读时长
   ============================================================ */

initPage('archive').then(initArticle);

const id = getParam('id');
let article = null;

async function initArticle() {
  if (!id) {
    showError('缺少文章 ID');
    return;
  }
  try {
    // 文章本体 + 上一篇/下一篇/相关阅读（/nav 接口），并行请求一次渲染
    const [detail, nav] = await Promise.all([
      api.get('/api/articles/' + id),
      api.get('/api/articles/' + id + '/nav').catch(() => null),
    ]);
    article = Object.assign({}, detail, nav || {});
    document.title = article.title + ' — LUMI';
    // SEO：动态注入 Open Graph / Twitter Card meta，让社交分享有标题、摘要、封面
    injectSEO(article);
    renderArticle();

    // 浏览量自增(前台调用一次)
    api.put('/api/articles/' + id + '/view').then(d => {
      if (d && d.viewCount) {
        const el = document.getElementById('view-num');
        if (el) {
          el.textContent = fmtNum(d.viewCount);
          pulse(el, 'bump'); // 数字跳动反馈
        }
      }
    }).catch(() => { });
    loadComments();
  } catch (e) {
    showError(e.message);
  }
}

// SEO meta 注入：Open Graph + Twitter Card + JSON-LD 结构化数据
// 让文章被分享到微信/Twitter/搜索引擎时有标题、摘要、封面图
function injectSEO(a) {
  const url = location.href;
  const title = a.title + ' — LUMI';
  const desc = a.summary || (a.content || '').replace(/<[^>]*>/g, '').slice(0, 120);
  const cover = a.coverImage || '';
  const metas = [
    { name: 'description', content: desc },
    { property: 'og:title', content: title },
    { property: 'og:description', content: desc },
    { property: 'og:type', content: 'article' },
    { property: 'og:url', content: url },
    { property: 'og:site_name', content: 'LUMI BLOG' },
    { name: 'twitter:card', content: 'summary_large_image' },
    { name: 'twitter:title', content: title },
    { name: 'twitter:description', content: desc },
  ];
  if (cover) {
    metas.push({ property: 'og:image', content: cover });
    metas.push({ name: 'twitter:image', content: cover });
  }
  // 文章发布时间
  if (a.createdAt) {
    metas.push({ property: 'article:published_time', content: a.createdAt });
  }
  metas.forEach(m => {
    let el = document.querySelector(`meta[${m.name ? 'name' : 'property'}="${m.name || m.property}"]`);
    if (!el) {
      el = document.createElement('meta');
      if (m.name) el.setAttribute('name', m.name);
      else el.setAttribute('property', m.property);
      document.head.appendChild(el);
    }
    el.setAttribute('content', m.content);
  });

  // JSON-LD 结构化数据：帮助搜索引擎理解文章结构
  const ld = {
    '@context': 'https://schema.org',
    '@type': 'BlogPosting',
    headline: a.title,
    datePublished: a.createdAt,
    author: { '@type': 'Person', name: a.author?.nickname || '匿名' },
    description: desc,
    url,
  };
  if (cover) ld.image = cover;
  let ldScript = document.getElementById('ld-json');
  if (!ldScript) {
    ldScript = document.createElement('script');
    ldScript.id = 'ld-json';
    ldScript.type = 'application/ld+json';
    document.head.appendChild(ldScript);
  }
  ldScript.textContent = JSON.stringify(ld);
}

function showError(msg) {
  document.getElementById('article-wrap').innerHTML =
    `<div class="empty" style="grid-column:1/13">${esc(msg)}</div>`;
}

function renderArticle() {
  const a = article;
  window.__article = article; // 供 fixBrokenImages 读取原文链接（sourceUrl）
  const wrap = document.getElementById('article-wrap');

  // 有密码且未解锁 → 显示锁
  if (a.hasPassword && !sessionStorage.getItem('unlock_' + a.id)) {
    wrap.innerHTML = `
      <div class="lock" style="grid-column:1/13">
        <div class="box">
          <div class="ico">🔒</div>
          <h3>${esc(a.title)}</h3>
          <p>这是一篇私密文章,请输入访问密码</p>
          <input type="password" id="lock-input" placeholder="访问密码">
          <button onclick="tryUnlock()">解锁阅读</button>
          <div class="err-msg" id="lock-err">密码错误,请重试</div>
        </div>
      </div>`;
    document.getElementById('lock-input').addEventListener('keydown', e => {
      if (e.key === 'Enter') tryUnlock();
    });
    return;
  }

  const content = (a.needPassword && sessionStorage.getItem('unlock_' + a.id) === '1')
    ? sessionStorage.getItem('unlock_content_' + a.id) || a.content
    : a.content;

  // 字数统计：去掉 HTML 标签后的纯文本字数
  const wordCount = (content || '').replace(/<[^>]*>/g, ' ').replace(/\s+/g, '').length;

  wrap.innerHTML = `
    <div class="article-content-wrap">
      <div class="article-head">
        <span class="a-cat">${esc(a.category?.name || '未分类')}</span>
        <h1>${esc(a.title)}</h1>
        <div class="a-meta">
          <span>${esc(a.author?.nickname || '作者')}</span><span class="sep">·</span>
          <span>${fmtDate(a.createdAt)}</span><span class="sep">·</span>
          <span>阅读 ${fmtMinutes(content)} 分钟</span><span class="sep">·</span>
          <span>${fmtNum(wordCount)} 字</span><span class="sep">·</span>
          <span><span id="view-num">${fmtNum(a.viewCount)}</span> 阅读</span><span class="sep">·</span>
          <a href="javascript:;" onclick="doLike()" class="like-link ${localStorage.getItem('liked_' + a.id) ? 'liked' : ''}" title="点赞"><span class="heart">${localStorage.getItem('liked_' + a.id) ? '❤' : '♡'}</span><span id="like-num">${a.likeCount || 0}</span></a>
          <span class="sep">·</span>
          <a href="javascript:;" onclick="shareArticle()" class="share-link" title="分享文章">↗ 分享</a>
          <span class="sep">·</span>
          <a href="javascript:window.print()" class="share-link" title="打印文章">⎙ 打印</a>
        </div>
        ${(a.tags && a.tags.length) ? `<div class="a-tags">${a.tags.map(t => `<a class="tag" href="articles.html?tag=${encodeURIComponent(t.name)}"># ${esc(t.name)}</a>`).join('')}</div>` : ''}
      </div>

      <article class="article-content" id="article-content">${content}</article>

      <div class="pn">
        ${a.prev && a.prev.id ? `<a href="article.html?id=${a.prev.id}"><span class="lbl">← 上一篇</span><span class="t">${esc(a.prev.title)}</span></a>` : '<a style="visibility:hidden"></a>'}
        ${a.next && a.next.id ? `<a href="article.html?id=${a.next.id}"><span class="lbl">下一篇 →</span><span class="t">${esc(a.next.title)}</span></a>` : '<a style="visibility:hidden"></a>'}
      </div>

      ${(a.related && a.related.length) ? `
      <div class="related">
        <h3>相关阅读</h3>
        ${a.related.map(r => `
          <a class="r-item" href="article.html?id=${r.id}">
            <span class="t">${esc(r.title)}</span><span class="v">${fmtNum(r.viewCount)} views</span>
          </a>`).join('')}
      </div>` : ''}
    </div>

    <aside class="article-side" id="toc-wrap">
      <nav class="toc" id="toc"></nav>
    </aside>

    <section class="comments">
      <div class="head"><h3>评论</h3><span class="n" id="comment-total">0 条</span></div>
      <form class="comment-form" id="comment-form" onsubmit="submitComment(event)">
        <div class="row">
          <input type="text" id="c-name" placeholder="昵称(可选，不填默认游客)" maxlength="20">
          <input type="email" id="c-email" placeholder="邮箱(可选)" style="display:none">
        </div>
        <div class="emoji-picker">
          <textarea id="c-content" placeholder="写下你的评论…" maxlength="1000"></textarea>
          <button type="button" class="emoji-trigger" id="emoji-trigger" title="插入表情">☺</button>
          <div class="emoji-panel" id="emoji-panel"></div>
        </div>
        <div style="margin-top:12px;display:flex;justify-content:space-between;align-items:center;">
          <span style="font-size:12px;color:var(--gray-a);">评论免审核，直接展示</span>
          <button class="submit" type="submit">发表评论</button>
        </div>
      </form>
      <div id="comment-list"></div>
    </section>`;

  renderTOC();
  highlightCode();
  initReadingProgress();
  initKeyboardNav();
  initLightbox();
  initEmojiPicker();
  fixBrokenImages();  // 图片加载失败 → 彩色占位符
}

// 图片加载失败兜底：CSDN 防盗链导致裂图 → 直接隐藏，并插入跳转链接
// 优先跳转到该文章的 CSDN 原文地址（article.sourceUrl），无原文时回退当前文章链接
function fixBrokenImages() {
  const imgs = document.querySelectorAll('#article-content img');
  if (!imgs.length) return;
  const target = (window.__article && window.__article.sourceUrl) || location.href;
  imgs.forEach(img => {
    if (img.dataset.fixed) return;
    img.dataset.fixed = '1';
    img.addEventListener('error', () => {
      if (img.dataset.fallback) return;
      img.dataset.fallback = '1';
      // 隐藏裂图
      img.style.display = 'none';
      // 在图片后插入原文链接提示（CSDN 原文地址，而非博客主页）
      const note = document.createElement('div');
      note.className = 'img-fallback';
      note.innerHTML = '<a href="' + target + '" target="_blank" rel="noopener" style="font-size:12px;color:var(--gray-a);border:1px dashed var(--line);display:inline-block;padding:8px 14px;margin:8px 0;">[图片加载失败] 查看原文 →</a>';
      img.parentNode.insertBefore(note, img.nextSibling);
    });
    // 立即检查已失败的图
    if (img.complete && img.naturalWidth === 0) {
      img.dispatchEvent(new Event('error'));
    }
  });
}

// 代码高亮：在文章渲染完成后调用 hljs
function highlightCode() {
  // 等待 highlight.js 库加载完成（CDN 用了 defer）
  const run = () => {
    if (typeof hljs === 'undefined') {
      setTimeout(run, 50);
      return;
    }
    document.querySelectorAll('#article-content pre code').forEach(b => {
      try { hljs.highlightElement(b); } catch (e) { }
    });
    addCopyButtons();
  };
  run();
}

// 给每个代码块加"复制"按钮：技术博客必备，方便读者抄代码
function addCopyButtons() {
  document.querySelectorAll('#article-content pre').forEach(pre => {
    if (pre.querySelector('.code-copy')) return; // 避免重复添加
    const btn = document.createElement('button');
    btn.className = 'code-copy';
    btn.textContent = '复制';
    btn.title = '复制代码';
    btn.addEventListener('click', async () => {
      const code = pre.querySelector('code');
      const text = code ? code.textContent : pre.textContent;
      try {
        await navigator.clipboard.writeText(text);
        btn.textContent = '已复制 ✓';
        btn.classList.add('done');
      } catch (e) {
        // 兜底：临时 textarea + execCommand
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand('copy'); btn.textContent = '已复制 ✓'; btn.classList.add('done'); }
        catch (err) { toast('复制失败', true); }
        document.body.removeChild(ta);
      }
      setTimeout(() => { btn.textContent = '复制'; btn.classList.remove('done'); }, 1800);
    });
    // pre 需要 position:relative 让按钮绝对定位到右上角
    pre.style.position = 'relative';
    pre.appendChild(btn);
  });
}

// 阅读进度条：根据滚动位置更新顶部细条宽度
function initReadingProgress() {
  const bar = document.getElementById('reading-progress');
  if (!bar) return;
  const update = () => {
    const h = document.documentElement;
    const scrolled = h.scrollTop;
    const total = h.scrollHeight - h.clientHeight;
    const pct = total > 0 ? (scrolled / total) * 100 : 0;
    bar.style.width = pct + '%';
  };
  window.addEventListener('scroll', update, { passive: true });
  update();
}

function renderTOC() {
  const content = document.getElementById('article-content');
  if (!content) return;
  const heads = content.querySelectorAll('h2, h3');
  if (!heads.length) { document.getElementById('toc-wrap').style.display = 'none'; return; }

  heads.forEach((h, i) => {
    h.id = 'sec-' + i;
  });
  const toc = document.getElementById('toc');
  toc.innerHTML = '<h4>目录 CONTENTS</h4><ul>' +
    Array.from(heads).map((h, i) =>
      `<li class="${h.tagName === 'H3' ? 'l2' : ''}"><a href="#sec-${i}" data-i="${i}">${esc(h.textContent)}</a></li>`
    ).join('') + '</ul>';

  const links = toc.querySelectorAll('a');

  // 滚动高亮：优先用 IntersectionObserver（性能更好，比 scroll 监听少抖动）
  // 不支持时回退到 scroll 监听
  if ('IntersectionObserver' in window) {
    const activeMap = new Map(); // 记录每个 head 当前是否在视口"判定区"
    const io = new IntersectionObserver((entries) => {
      entries.forEach(e => {
        activeMap.set(e.target.dataset.i, e.isIntersecting);
      });
      // 取第一个仍"在判定区"的标题作为当前高亮
      let cur = -1;
      heads.forEach((h, i) => {
        if (activeMap.get(String(i)) || h.getBoundingClientRect().top < 120) cur = i;
      });
      if (cur === -1) cur = 0;
      links.forEach((l, i) => l.classList.toggle('active', i === cur));
    }, {
      // 判定区：顶部 120px 以下、底部 60% 以上才算"正在阅读"
      rootMargin: '-120px 0px -60% 0px',
      threshold: 0,
    });
    heads.forEach((h, i) => {
      h.dataset.i = String(i); // 让回调能拿到对应 TOC 索引
      io.observe(h);
    });
  } else {
    // 回退方案：scroll 监听 + getBoundingClientRect
    window.addEventListener('scroll', () => {
      let cur = 0;
      heads.forEach((h, i) => { if (h.getBoundingClientRect().top < 120) cur = i; });
      links.forEach((l, i) => l.classList.toggle('active', i === cur));
    }, { passive: true });
  }

  // 点击 TOC 项：平滑滚动到对应标题（避免默认 hash 跳转的硬切）
  links.forEach(l => {
    l.addEventListener('click', (e) => {
      e.preventDefault();
      const target = document.getElementById('sec-' + l.dataset.i);
      if (target) {
        const top = target.getBoundingClientRect().top + window.scrollY - 80;
        window.scrollTo({ top, behavior: 'smooth' });
        history.replaceState(null, '', '#sec-' + l.dataset.i);
      }
    });
  });
}

// 键盘快捷键：← 上一篇 / → 下一篇 / c 聚焦评论框
function initKeyboardNav() {
  document.addEventListener('keydown', (e) => {
    // 在输入框内不拦截（避免影响打字）
    const tag = (e.target.tagName || '').toLowerCase();
    if (tag === 'input' || tag === 'textarea') return;
    if (e.key === 'ArrowLeft' && article && article.prev && article.prev.id) {
      location.href = 'article.html?id=' + article.prev.id;
    } else if (e.key === 'ArrowRight' && article && article.next && article.next.id) {
      location.href = 'article.html?id=' + article.next.id;
    } else if (e.key.toLowerCase() === 'c') {
      const ta = document.getElementById('c-content');
      if (ta) { ta.focus(); ta.scrollIntoView({ behavior: 'smooth', block: 'center' }); }
    }
  });
}

// 触发一次"弹跳"动画（重复触发可重放）
function pulse(el, cls) {
  if (!el) return;
  el.classList.remove(cls);
  void el.offsetWidth;
  el.classList.add(cls);
}

// 分享文章：优先用 Web Share API（移动端原生分享），回退到复制链接
async function shareArticle() {
  const url = location.href;
  const shareData = {
    title: article ? article.title : document.title,
    text: article ? (article.summary || article.title) : document.title,
    url,
  };
  // 现代浏览器 + 移动端：调原生分享面板
  if (navigator.share) {
    try { await navigator.share(shareData); return; } catch (e) { /* 用户取消，静默 */ }
  }
  // 回退：复制链接到剪贴板
  try {
    await navigator.clipboard.writeText(url);
    toast('链接已复制到剪贴板');
    pulse(document.querySelector('.share-link'), 'pop'); // 分享按钮反馈
  } catch (e) {
    // 老浏览器兜底：用临时 input + execCommand
    const tmp = document.createElement('input');
    tmp.value = url;
    document.body.appendChild(tmp);
    tmp.select();
    try { document.execCommand('copy'); toast('链接已复制'); pulse(document.querySelector('.share-link'), 'pop'); }
    catch (err) { toast('复制失败，请手动复制地址栏链接', true); }
    document.body.removeChild(tmp);
  }
}

// 私密解锁
async function tryUnlock() {
  const pwd = document.getElementById('lock-input').value;
  try {
    const data = await api.post('/api/articles/' + id + '/unlock', { password: pwd });
    sessionStorage.setItem('unlock_' + id, '1');
    sessionStorage.setItem('unlock_content_' + id, data.content);
    location.reload();
  } catch (e) {
    document.getElementById('lock-err').style.display = 'block';
  }
}

// 点赞/取消点赞：爱心实心⇄空心切换 + 弹跳动画 + 数字缩放（本地记住已赞状态）
async function doLike() {
  try {
    const data = await api.put('/api/articles/' + id + '/like');
    const num = document.getElementById('like-num');
    num.textContent = data.likeCount;
    const link = document.querySelector('.like-link');
    const heart = link.querySelector('.heart');
    if (data.already) {
      // 原来已赞 → 本次是取消：空心 + 移除已赞样式
      if (heart) heart.textContent = '♡';
      link.classList.remove('liked');
      localStorage.removeItem('liked_' + id);
      toast('已取消点赞 ♡');
    } else {
      // 未赞 → 本次是点赞：实心红心
      if (heart) heart.textContent = '❤';
      link.classList.add('liked');
      localStorage.setItem('liked_' + id, '1');
      toast('点赞成功 ❤');
    }
    pulse(link, 'like-anim'); // 爱心弹跳
    pulse(num, 'bump');       // 数字缩放
  } catch (e) { toast(e.message, true); }
}

// 评论
let replyParent = null;

// emoji 选择器：点击表情插入到评论框光标位置
function initEmojiPicker() {
  const trigger = document.getElementById('emoji-trigger');
  const panel = document.getElementById('emoji-panel');
  const ta = document.getElementById('c-content');
  if (!trigger || !panel || !ta) return;

  // 常用表情集合（不依赖外部库，纯前端）
  const emojis = [
    '😀', '😂', '🤣', '😊', '😍', '🥰', '😎', '🤔',
    '😴', '🤯', '😱', '😭', '😅', '😄', '🙂', '🙃',
    '👍', '👎', '👏', '🙌', '🤝', '✌️', '🤞', '💪',
    '❤️', '🧡', '💛', '💚', '💙', '💜', '🖤', '💔',
    '🔥', '✨', '⭐', '🌟', '💡', '🎯', '🎉', '🎊',
    '✅', '❌', '❓', '❗', '📝', '📌', '📎', '🔗',
    '🚀', '💻', '⌨️', '🖥️', '🐛', '☕', '📚', '🎓',
  ];

  panel.innerHTML = emojis.map(e => `<button type="button" data-e="${e}">${e}</button>`).join('');

  trigger.addEventListener('click', (e) => {
    e.stopPropagation();
    panel.classList.toggle('show');
  });

  panel.addEventListener('click', (e) => {
    const btn = e.target.closest('button[data-e]');
    if (!btn) return;
    const emoji = btn.dataset.e;
    // 插入到光标位置（而非简单 append）
    const start = ta.selectionStart;
    const end = ta.selectionEnd;
    ta.value = ta.value.slice(0, start) + emoji + ta.value.slice(end);
    ta.focus();
    // 光标移到插入的表情后面
    const pos = start + emoji.length;
    ta.setSelectionRange(pos, pos);
  });

  // 点击面板外部关闭
  document.addEventListener('click', (e) => {
    if (!panel.contains(e.target) && e.target !== trigger) {
      panel.classList.remove('show');
    }
  });
}

async function loadComments() {
  const listEl = document.getElementById('comment-list');
  try {
    const data = await api.get(`/api/articles/${id}/comments?page=1&pageSize=50`);
    document.getElementById('comment-total').textContent = data.total + ' 条';
    renderComments(data.list);
  } catch (e) {
    listEl.innerHTML = '<div class="empty">评论加载失败</div>';
  }
}

function renderComments(list) {
  const listEl = document.getElementById('comment-list');
  if (!list || !list.length) {
    listEl.innerHTML = '<div class="empty">还没有评论,来抢沙发~</div>';
    return;
  }
  const top = list.filter(c => !c.parentId);
  const reply = list.filter(c => c.parentId);

  listEl.innerHTML = top.map((c, idx) => `
    <div class="comment-item">
      <div class="c-meta">
        <span class="c-floor">#${idx + 1}</span>
        <span class="c-name">${esc(c.nickname || '匿名')}</span>
        <span class="c-time">${fmtDate(c.createdAt)}</span>
      </div>
      <div class="c-body">${esc(c.content)}</div>
      <button class="c-reply-btn" onclick="setReply(${c.id},'${esc(c.nickname || '匿名')}')">回复</button>
      ${reply.filter(r => r.parentId === c.id).map(r => `
        <div class="reply">
          <div class="c-meta">
            <span class="c-name">${esc(r.nickname || '匿名')}</span>
            <span class="c-time">${fmtDate(r.createdAt)}</span>
          </div>
          <div class="c-body">${esc(r.content)}</div>
        </div>`).join('')}
    </div>`).join('');
}

function setReply(pid, name) {
  replyParent = pid;
  const ta = document.getElementById('c-content');
  ta.focus();
  ta.placeholder = '回复 ' + name + '…';
}

async function submitComment(e) {
  e.preventDefault();
  const name = document.getElementById('c-name').value.trim();
  const content = document.getElementById('c-content').value.trim();
  // 昵称非必填：留空时后端默认"游客"；仅校验内容
  if (!content) return toast('评论内容不能为空', true);

  try {
    await api.post(`/api/articles/${id}/comments`, {
      content, nickname: name, parentId: replyParent,
    });
    toast('评论已发布');
    document.getElementById('c-content').value = '';
    replyParent = null;
    document.getElementById('c-content').placeholder = '写下你的评论…';
    loadComments(); // 立即刷新评论列表（免审核，直接展示）
  } catch (err) {
    toast(err.message, true);
  }
}
