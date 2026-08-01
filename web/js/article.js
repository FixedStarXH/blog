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
    article = await api.get('/api/articles/' + id);
    document.title = article.title + ' — LUMI';
    renderArticle();

    // 浏览量自增(前台调用一次)
    api.put('/api/articles/' + id + '/view').then(d => {
      if (d && d.viewCount) {
        const el = document.getElementById('view-num');
        if (el) el.textContent = fmtNum(d.viewCount);
      }
    }).catch(() => {});
    loadComments();
  } catch (e) {
    showError(e.message);
  }
}

function showError(msg) {
  document.getElementById('article-wrap').innerHTML =
    `<div class="empty" style="grid-column:1/13">${esc(msg)}</div>`;
}

function renderArticle() {
  const a = article;
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

  const content = (a.hasPassword && sessionStorage.getItem('unlock_' + a.id) === '1')
    ? sessionStorage.getItem('unlock_content_' + a.id) || a.content
    : a.content;

  wrap.innerHTML = `
    <div class="article-content-wrap">
      <div class="article-head">
        <span class="a-cat">${esc(a.category?.name || '未分类')}</span>
        <h1>${esc(a.title)}</h1>
        <div class="a-meta">
          <span>${esc(a.author?.nickname || '作者')}</span><span class="sep">·</span>
          <span>${fmtDate(a.createdAt)}</span><span class="sep">·</span>
          <span>阅读 ${fmtMinutes(content)} 分钟</span><span class="sep">·</span>
          <span><span id="view-num">${fmtNum(a.viewCount)}</span> 阅读</span><span class="sep">·</span>
          <a href="javascript:;" onclick="doLike()" style="color:var(--red);">♡ <span id="like-num">${a.likeCount || 0}</span></a>
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
          <input type="text" id="c-name" placeholder="昵称(2-20 字)" maxlength="20">
          <input type="email" id="c-email" placeholder="邮箱(可选)" style="display:none">
        </div>
        <textarea id="c-content" placeholder="写下你的评论…" maxlength="1000"></textarea>
        <div style="margin-top:12px;display:flex;justify-content:space-between;align-items:center;">
          <span style="font-size:12px;color:var(--gray-a);">评论需审核后展示</span>
          <button class="submit" type="submit">发表评论</button>
        </div>
      </form>
      <div id="comment-list"></div>
    </section>`;

  renderTOC();
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
      `<li class="${h.tagName === 'H3' ? 'l2' : ''}"><a href="#sec-${i}">${esc(h.textContent)}</a></li>`
    ).join('') + '</ul>';

  // 滚动高亮
  const links = toc.querySelectorAll('a');
  window.addEventListener('scroll', () => {
    let cur = 0;
    heads.forEach((h, i) => { if (h.getBoundingClientRect().top < 120) cur = i; });
    links.forEach((l, i) => l.classList.toggle('active', i === cur));
  }, { passive: true });
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

// 点赞
async function doLike() {
  try {
    const data = await api.put('/api/articles/' + id + '/like');
    document.getElementById('like-num').textContent = data.likeCount;
    toast(data.already ? '已经点过赞啦 ♡' : '点赞成功');
  } catch (e) { toast(e.message, true); }
}

// 评论
let replyParent = null;

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

  listEl.innerHTML = top.map(c => `
    <div class="comment-item">
      <div class="c-meta"><span class="c-name">${esc(c.nickname || '匿名')}</span><span class="c-time">${fmtDate(c.createdAt)}</span></div>
      <div class="c-body">${esc(c.content)}</div>
      <button class="c-reply-btn" onclick="setReply(${c.id},'${esc(c.nickname || '匿名')}')">回复</button>
      ${reply.filter(r => r.parentId === c.id).map(r => `
        <div class="reply">
          <div class="c-meta"><span class="c-name">${esc(r.nickname || '匿名')}</span><span class="c-time">${fmtDate(r.createdAt)}</span></div>
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
  if (!name || name.length < 2) return toast('请填写昵称(2-20 字)', true);
  if (!content) return toast('评论内容不能为空', true);

  try {
    await api.post(`/api/articles/${id}/comments`, {
      content, nickname: name, parentId: replyParent,
    });
    toast('评论已提交,待审核后展示');
    document.getElementById('c-content').value = '';
    replyParent = null;
    document.getElementById('c-content').placeholder = '写下你的评论…';
  } catch (err) {
    toast(err.message, true);
  }
}
