package web

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>小红书内容工作台</title>
  <style>
    :root {
      --bg: #f5f7fa;
      --nav: #101820;
      --nav-muted: #94a3b8;
      --surface: #ffffff;
      --surface-2: #f0f3f7;
      --text: #18202a;
      --muted: #667386;
      --border: #d9e0e8;
      --accent: #1769d1;
      --accent-dark: #11519f;
      --danger: #b42318;
      --ok: #157347;
      --radius: 8px;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      font-size: 14px;
      line-height: 1.45;
    }
    .app { min-height: 100vh; display: grid; grid-template-columns: 232px 1fr; }
    aside {
      background: var(--nav);
      color: white;
      padding: 18px 14px;
      position: sticky;
      top: 0;
      height: 100vh;
    }
    .brand { padding: 4px 8px 18px; border-bottom: 1px solid rgba(255,255,255,.12); margin-bottom: 14px; }
    .brand h1 { font-size: 18px; margin: 0 0 4px; }
    .brand p { margin: 0; color: var(--nav-muted); font-size: 12px; }
    nav { display: grid; gap: 4px; }
    .nav-btn {
      width: 100%;
      border: 0;
      border-radius: 6px;
      background: transparent;
      color: var(--nav-muted);
      padding: 10px 10px;
      text-align: left;
      font-weight: 650;
      cursor: pointer;
    }
    .nav-btn:hover, .nav-btn.active { background: rgba(255,255,255,.1); color: white; }
    main { min-width: 0; }
    header {
      background: var(--surface);
      border-bottom: 1px solid var(--border);
      padding: 18px 24px;
      position: sticky;
      top: 0;
      z-index: 5;
    }
    .page-title { margin: 0; font-size: 21px; }
    .page-subtitle { color: var(--muted); margin-top: 4px; }
    .content { padding: 20px 24px 40px; max-width: 1480px; }
    .view { display: none; }
    .view.active { display: block; }
    .grid { display: grid; grid-template-columns: repeat(12, 1fr); gap: 16px; }
    .span-3 { grid-column: span 3; }
    .span-4 { grid-column: span 4; }
    .span-5 { grid-column: span 5; }
    .span-6 { grid-column: span 6; }
    .span-7 { grid-column: span 7; }
    .span-8 { grid-column: span 8; }
    .span-12 { grid-column: span 12; }
    .panel, .metric {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 16px;
    }
    .panel h2, .metric h2 { margin: 0 0 12px; font-size: 15px; }
    .metric-value { font-size: 28px; font-weight: 800; color: var(--accent); }
    .metric-label { color: var(--muted); font-size: 12px; }
    .row { display: flex; gap: 10px; align-items: end; flex-wrap: wrap; }
    .field { display: grid; gap: 5px; min-width: 160px; flex: 1; }
    label { color: var(--muted); font-size: 12px; font-weight: 650; }
    input, select {
      width: 100%;
      border: 1px solid var(--border);
      border-radius: 6px;
      background: #fff;
      color: var(--text);
      padding: 8px 10px;
      font: inherit;
      min-height: 36px;
    }
    button {
      border: 1px solid var(--border);
      border-radius: 6px;
      min-height: 36px;
      padding: 8px 12px;
      background: var(--surface);
      color: var(--text);
      font-weight: 650;
      cursor: pointer;
    }
    button.primary { background: var(--accent); border-color: var(--accent); color: #fff; }
    button.primary:hover { background: var(--accent-dark); }
    .item { border-top: 1px solid var(--border); padding: 11px 0; }
    .item:first-child { border-top: 0; padding-top: 0; }
    .item h3 { margin: 0 0 6px; font-size: 14px; }
    .item.selectable { cursor: pointer; border-radius: 6px; padding-left: 8px; padding-right: 8px; }
    .item.selectable:hover, .item.selectable.active { background: var(--surface-2); }
    .item-head { display: flex; align-items: start; justify-content: space-between; gap: 8px; }
    .item-check { display: flex; align-items: flex-start; gap: 9px; }
    .item-check input { width: 16px; min-height: 16px; margin-top: 2px; flex: 0 0 auto; }
    .item-actions { display: inline-flex; gap: 6px; flex: 0 0 auto; }
    .icon-btn {
      min-width: 30px;
      width: 30px;
      height: 30px;
      min-height: 30px;
      padding: 0;
      display: inline-grid;
      place-items: center;
    }
    .icon-btn.danger { color: var(--danger); }
    .meta { color: var(--muted); font-size: 12px; display: flex; gap: 10px; flex-wrap: wrap; }
    .score { font-size: 24px; font-weight: 800; color: var(--accent); }
    .pill { display: inline-flex; border: 1px solid var(--border); border-radius: 999px; padding: 2px 8px; font-size: 12px; color: var(--muted); }
    .status { min-height: 22px; margin-top: 10px; color: var(--muted); }
    .status.error { color: var(--danger); }
    .status.ok { color: var(--ok); }
    pre {
      white-space: pre-wrap;
      margin: 8px 0 0;
      font-family: inherit;
      color: var(--text);
      background: var(--surface-2);
      border-radius: 6px;
      padding: 10px;
      max-height: 290px;
      overflow: auto;
    }
    .run-log { max-height: 360px; }
    .empty { color: var(--muted); padding: 20px 0; }
    .detail-title { margin: 0 0 8px; font-size: 18px; }
    .detail-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin: 12px 0; }
    .detail-field { background: var(--surface-2); border-radius: 6px; padding: 10px; min-width: 0; }
    .detail-field strong { display: block; color: var(--muted); font-size: 12px; margin-bottom: 4px; }
    .detail-compact { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0; }
    .detail-compact span { background: var(--surface-2); border-radius: 999px; padding: 5px 9px; color: var(--muted); font-size: 12px; }
    .detail-line { background: var(--surface-2); border-radius: 6px; padding: 10px; margin: 8px 0; word-break: break-word; }
    .detail-line strong { color: var(--muted); font-size: 12px; margin-right: 8px; }
    .detail-pair { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin: 8px 0; }
    .detail-link { color: var(--accent); word-break: break-all; }
    .qr-box { display: grid; gap: 10px; align-items: start; }
    .qr-box img { width: 220px; height: 220px; border: 1px solid var(--border); border-radius: 6px; background: white; padding: 8px; }
    .selectable pre { cursor: text; user-select: text; }
    @media (max-width: 980px) {
      .app { grid-template-columns: 1fr; }
      aside { position: static; height: auto; }
      nav { grid-template-columns: repeat(3, 1fr); }
      header, .content { padding-left: 14px; padding-right: 14px; }
      .span-3, .span-4, .span-5, .span-6, .span-7, .span-8 { grid-column: span 12; }
    }
  </style>
</head>
<body>
  <div class="app">
    <aside>
      <div class="brand">
        <h1>小红书内容工作台</h1>
        <p>采集 / 选题 / 草稿 / 复盘</p>
      </div>
      <nav aria-label="工作区导航">
        <button class="nav-btn active" data-view="overview">总览</button>
        <button class="nav-btn" data-view="collect">采集</button>
        <button class="nav-btn" data-view="content">内容</button>
        <button class="nav-btn" data-view="topics">选题</button>
        <button class="nav-btn" data-view="drafts">草稿</button>
        <button class="nav-btn" data-view="review">复盘</button>
        <button class="nav-btn" data-view="settings">设置</button>
      </nav>
    </aside>
    <main>
      <header>
        <h1 class="page-title" id="pageTitle">总览</h1>
        <div class="page-subtitle" id="pageSubtitle">查看采集、选题、草稿和复盘状态</div>
      </header>
      <div class="content">
        <section class="view active" id="view-overview">
          <div class="grid">
            <div class="metric span-3"><h2>采集内容</h2><div class="metric-value" id="countItems">0</div><div class="metric-label">最近入库笔记</div></div>
            <div class="metric span-3"><h2>候选选题</h2><div class="metric-value" id="countCandidates">0</div><div class="metric-label">已评分选题</div></div>
            <div class="metric span-3"><h2>草稿</h2><div class="metric-value" id="countDrafts">0</div><div class="metric-label">待审核内容</div></div>
            <div class="metric span-3"><h2>复盘</h2><div class="metric-value" id="countReports">0</div><div class="metric-label">表现报告</div></div>
            <div class="panel span-6"><h2>高分选题</h2><div id="overviewCandidates"></div></div>
            <div class="panel span-6"><h2>最新草稿</h2><div id="overviewDrafts"></div></div>
          </div>
        </section>

        <section class="view" id="view-collect">
          <div class="grid">
            <div class="panel span-8">
              <h2>新增采集目标</h2>
              <div class="row">
                <div class="field"><label for="targetName">名称</label><input id="targetName" value="AI工具"></div>
                <div class="field"><label for="targetKeyword">关键词</label><input id="targetKeyword" value="AI工具"></div>
                <div class="field"><label for="targetInterval">间隔秒</label><input id="targetInterval" type="number" value="300"></div>
                <button id="addTarget">新增目标</button>
                <button id="cancelTargetEdit" hidden>取消</button>
              </div>
              <div class="status" id="collectStatus"></div>
            </div>
            <div class="panel span-4">
              <h2>小红书登录</h2>
              <div class="row">
                <button id="startXHSLogin">生成登录二维码</button>
                <button id="logoutXHS">登出</button>
              </div>
              <div class="qr-box" id="xhsLoginBox"></div>
              <div class="status" id="xhsLoginStatus">仅在登录失效时使用</div>
            </div>
            <div class="panel span-5">
              <h2>采集控制</h2>
              <div class="row">
                <div class="field"><label for="collectItemLimit">每个目标采集条数</label><input id="collectItemLimit" type="number" min="1" max="50" value="3"></div>
                <button class="primary" id="collect">采集一次</button>
              </div>
              <div class="status" id="collectRunStatus"></div>
            </div>
            <div class="panel span-7"><h2>采集目标</h2><div id="targets"></div></div>
            <div class="panel span-4"><h2>运行记录</h2><div id="runs"></div></div>
            <div class="panel span-8"><h2>运行详情</h2><div id="runDetail"></div></div>
          </div>
        </section>

        <section class="view" id="view-content">
          <div class="grid">
            <div class="panel span-5"><h2>内容列表</h2><div id="contentItems"></div></div>
            <div class="panel span-7"><h2>内容详情</h2><div id="itemDetail"></div></div>
          </div>
        </section>

        <section class="view" id="view-topics">
          <div class="grid">
            <div class="panel span-12">
              <h2>拆解与评分</h2>
              <div class="row">
                <div class="field"><label for="topicEngine">模型</label><select id="topicEngine"><option value="rule">规则版</option><option value="llm">大模型版</option></select></div>
                <div class="field"><label for="topicLimit">数量</label><input id="topicLimit" type="number" min="1" max="100" value="10"></div>
                <button id="analyze">拆解笔记</button>
                <button class="primary" id="score">生成评分</button>
              </div>
              <div class="status" id="topicStatus"></div>
            </div>
            <div class="panel span-5"><h2>拆解结果</h2><div id="analyses"></div></div>
            <div class="panel span-7"><h2>候选选题</h2><div id="candidates"></div></div>
          </div>
        </section>

        <section class="view" id="view-drafts">
          <div class="grid">
            <div class="panel span-12">
              <h2>草稿生成</h2>
              <div class="row">
                <div class="field"><label for="draftEngine">模型</label><select id="draftEngine"><option value="rule">规则版</option><option value="llm">大模型版</option></select></div>
                <div class="field"><label for="draftLimit">数量</label><input id="draftLimit" type="number" min="1" max="100" value="10"></div>
                <button class="primary" id="draft">生成草稿</button>
              </div>
              <div class="status" id="draftStatus"></div>
            </div>
            <div class="panel span-12"><h2>草稿库</h2><div id="drafts"></div></div>
          </div>
        </section>

        <section class="view" id="view-review">
          <div class="grid">
            <div class="panel span-12">
              <h2>复盘评分</h2>
              <div class="row">
                <div class="field"><label for="reviewLimit">数量</label><input id="reviewLimit" type="number" min="1" max="100" value="10"></div>
                <button class="primary" id="reviewScore">生成复盘评分</button>
              </div>
              <div class="status" id="reviewStatus"></div>
            </div>
            <div class="panel span-5"><h2>发布记录</h2><div id="publishes"></div></div>
            <div class="panel span-7"><h2>复盘报告</h2><div id="reports"></div></div>
          </div>
        </section>

        <section class="view" id="view-settings">
          <div class="grid">
            <div class="panel span-12">
              <h2>密钥分发服务</h2>
              <div class="row">
                <div class="field"><label for="keyBase">服务地址</label><input id="keyBase" placeholder="https://distribute-keys.vercel.app"></div>
                <div class="field"><label for="keyUser">用户名</label><input id="keyUser" autocomplete="username"></div>
                <div class="field"><label for="keyPass">密码</label><input id="keyPass" type="password" autocomplete="current-password"></div>
                <div class="field"><label for="keyName">可用密钥</label><select id="keyName"><option value="">先测试密钥分发服务</option></select></div>
                <button class="primary" id="saveKey">测试并保存</button>
              </div>
              <div class="status" id="keyStatus"></div>
            </div>
          </div>
        </section>
      </div>
    </main>
  </div>
  <script>
    const qs = (id) => document.getElementById(id);
    const state = { data: null, editingTargetId: 0, selectedItemId: 0, selectedTargetIds: new Set(), selectedRunId: 0, pendingCollectBody: null, retryingCollectAfterLogin: false };
    const viewMeta = {
      overview: ["总览", "查看采集、选题、草稿和复盘状态"],
      collect: ["采集", "管理关键词目标，低频抓取最新内容"],
      content: ["内容", "查看已采集内容列表和完整详情"],
      topics: ["选题", "拆解笔记并生成可排序的候选选题"],
      drafts: ["草稿", "把高分选题生成可审核的内容草稿"],
      review: ["复盘", "记录发布表现并沉淀复盘评分"],
      settings: ["设置", "配置密钥分发服务和大模型密钥"]
    };

    async function api(path, options = {}) {
      const resp = await fetch(path, { ...options, headers: { "Content-Type": "application/json", ...(options.headers || {}) } });
      const data = await resp.json();
      if (!resp.ok) throw new Error(data.error || "请求失败");
      return data;
    }
    function setStatus(id, message, kind = "") {
      const el = qs(id);
      el.className = "status " + kind;
      el.textContent = message;
    }
    async function refresh() {
      state.data = await api("/api/state?limit=30");
      const cfg = state.data.config || {};
      qs("keyBase").value = cfg.key_dist_base_url || qs("keyBase").value;
      qs("keyUser").value = cfg.key_dist_username || qs("keyUser").value;
      renderKeyOptions(cfg.available_keys || [], cfg.key_name || "");
      render();
    }
    function switchView(name) {
      document.querySelectorAll(".view").forEach(el => el.classList.remove("active"));
      document.querySelectorAll(".nav-btn").forEach(el => el.classList.toggle("active", el.dataset.view === name));
      qs("view-" + name).classList.add("active");
      qs("pageTitle").textContent = viewMeta[name][0];
      qs("pageSubtitle").textContent = viewMeta[name][1];
    }
    function render() {
      const data = state.data || {};
      qs("countItems").textContent = (data.items || []).length;
      qs("countCandidates").textContent = (data.candidates || []).length;
      qs("countDrafts").textContent = (data.drafts || []).length;
      qs("countReports").textContent = (data.reports || []).length;
      renderList("overviewCandidates", (data.candidates || []).slice(0, 3), renderCandidate);
      renderList("overviewDrafts", (data.drafts || []).slice(0, 2), renderDraft);
      renderList("targets", data.targets, renderTarget);
      renderList("contentItems", data.items, renderContentListItem);
      renderItemDetail();
      renderList("runs", data.runs, renderRun);
      renderRunDetail();
      renderList("analyses", data.analyses, renderAnalysis);
      renderList("candidates", data.candidates, renderCandidate);
      renderList("drafts", data.drafts, renderDraft);
      renderList("publishes", data.publishes, renderPublish);
      renderList("reports", data.reports, renderReport);
    }
    function renderTarget(t) {
      const id = Number(field(t, "id", "ID"));
      const checked = state.selectedTargetIds.has(id) ? " checked" : "";
      return '<div class="item"><div class="item-head"><label class="item-check"><input type="checkbox" onchange="toggleTargetSelection(' + id + ', this.checked)"' + checked + '><span><h3>' + esc(field(t, "name", "Name")) + '</h3><div class="meta"><span>' + esc(field(t, "kind", "Kind")) + '</span><span>' + esc(field(t, "keyword", "Keyword") || field(t, "url", "URL")) + '</span><span>' + field(t, "min_interval_seconds", "MinIntervalSeconds") + 's</span></div></span></label><div class="item-actions"><button class="icon-btn" title="编辑" aria-label="编辑采集目标" onclick="editTarget(' + id + ')">改</button><button class="icon-btn danger" title="删除" aria-label="删除采集目标" onclick="deleteTarget(' + id + ')">×</button></div></div></div>';
    }
    function renderItem(item) { return '<div class="item"><h3>' + esc(item.title || "无标题") + '</h3><div class="meta"><span>' + esc(item.author_name || "未知作者") + '</span><span>赞 ' + num(item.like_count) + '</span><span>藏 ' + num(item.collect_count) + '</span><span>评 ' + num(item.comment_count) + '</span></div>' + (item.body ? '<pre>' + esc(item.body) + '</pre>' : '') + '</div>'; }
    function renderContentListItem(item) {
      const active = Number(item.id) === Number(state.selectedItemId) ? " active" : "";
      return '<div class="item selectable' + active + '" onclick="selectItem(' + Number(item.id) + ')"><h3>' + esc(item.title || "无标题") + '</h3><div class="meta"><span>' + esc(item.author_name || "未知作者") + '</span><span>赞 ' + num(item.like_count) + '</span><span>藏 ' + num(item.collect_count) + '</span><span>评 ' + num(item.comment_count) + '</span></div></div>';
    }
    function renderItemDetail() {
      const items = (state.data && state.data.items) || [];
      if (!state.selectedItemId && items.length > 0) state.selectedItemId = items[0].id;
      const item = items.find(it => Number(it.id) === Number(state.selectedItemId));
      const el = qs("itemDetail");
      if (!item) { el.innerHTML = '<div class="empty">请选择左侧内容</div>'; return; }
      const tags = (item.tags || []).join(" / ");
      el.innerHTML = '<h3 class="detail-title">' + esc(item.title || "无标题") + '</h3>'
        + '<div class="meta"><span>ID ' + item.id + '</span><span>目标 ' + esc(item.target_name || "") + '</span><span>采集 ' + esc(item.captured_at || "") + '</span></div>'
        + detailMetaBlock(item)
        + '<div class="detail-compact"><span>作者 ' + esc(item.author_name || "-") + '</span><span>赞 ' + num(item.like_count) + '</span><span>藏 ' + num(item.collect_count) + '</span><span>评 ' + num(item.comment_count) + '</span><span>发布 ' + esc(item.published_at || "-") + '</span></div>'
        + '<div class="detail-line"><strong>标签</strong>' + esc(tags || "-") + '</div>'
        + '<div class="detail-line"><strong>链接</strong>' + (item.url ? '<a class="detail-link" href="' + esc(item.url) + '" target="_blank" rel="noreferrer">' + esc(item.url) + '</a>' : "-") + '</div>'
        + (shouldShowDetailFailure(item) ? '<h3>详情失败原因</h3><pre>' + esc(item.detail_message) + '</pre>' : '')
        + '<h3>正文</h3>' + (item.body ? '<pre>' + esc(item.body) + '</pre>' : '<div class="empty">无正文</div>')
        + '<h3>原始 JSON</h3><pre>' + esc(JSON.stringify(item.raw || {}, null, 2)) + '</pre>';
    }
    function detailField(label, value) { return '<div class="detail-field"><strong>' + esc(label) + '</strong><span>' + value + '</span></div>'; }
    function renderAnalysis(a) { return '<div class="item"><h3>' + esc(a.topic) + '</h3><div class="meta"><span>' + esc(a.model_name) + '</span><span>' + esc(a.reusable_pattern) + '</span></div><pre>' + esc(a.audience_pain || "") + '</pre></div>'; }
    function renderCandidate(c) { return '<div class="item"><div class="score">' + c.total_score + '</div><h3>' + esc(c.topic) + '</h3><div class="meta"><span>趋势 ' + c.trend_score + '</span><span>涨粉 ' + c.growth_score + '</span><span>风险 ' + c.risk_score + '</span><span>' + esc(c.scoring_model) + '</span></div><pre>' + esc(c.reason || "") + '</pre></div>'; }
    function renderDraft(d) { return '<div class="item"><h3>' + esc((d.title_options || [])[0] || "草稿") + '</h3><div class="meta"><span>' + esc(d.generator) + '</span><span>' + (d.tags || []).map(esc).join(" / ") + '</span></div><pre>' + esc(d.opening + "\n\n" + d.body) + '</pre></div>'; }
    function renderRun(r) {
      const active = Number(r.id) === Number(state.selectedRunId) ? " active" : "";
      const detail = runDetailFor(r.id);
      const count = detail ? (detail.items || []).length : 0;
      return '<div class="item selectable' + active + '" onclick="selectRunIfNoText(' + Number(r.id) + ')"><h3>' + esc(r.status) + ' <span class="pill">' + esc(r.mode) + '</span></h3><div class="meta"><span>' + esc(r.target_name) + '</span><span>' + esc(r.started_at) + '</span><span>内容 ' + count + '</span>' + (r.message ? '<span>有日志</span>' : '') + '</div></div>';
    }
    function renderRunDetail() {
      const details = (state.data && state.data.run_details) || [];
      if (!state.selectedRunId && details.length > 0) state.selectedRunId = details[0].id;
      const detail = details.find(r => Number(r.id) === Number(state.selectedRunId));
      const el = qs("runDetail");
      if (!detail) { el.innerHTML = '<div class="empty">请选择左侧运行记录</div>'; return; }
      const items = detail.items || [];
      const titles = items.length ? items.map((it, idx) => '<div class="item"><h3>' + (idx + 1) + '. ' + esc(it.title || "无标题") + '</h3><div class="meta"><span>' + esc(it.author_name || "未知作者") + '</span>' + (it.url ? '<span><a class="detail-link" href="' + esc(it.url) + '" target="_blank" rel="noreferrer">打开链接</a></span>' : '') + '</div></div>').join("") : '<div class="empty">这条运行没有关联内容。旧运行记录可能没有详情。</div>';
      el.innerHTML = '<div class="meta"><span>目标 ' + esc(detail.target_name || "") + '</span><span>状态 ' + esc(detail.status || "") + '</span><span>模式 ' + esc(detail.mode || "") + '</span><span>开始 ' + esc(detail.started_at || "") + '</span><span>结束 ' + esc(detail.finished_at || "-") + '</span><span>采集 ' + items.length + ' 条</span></div>'
        + '<h3>本次采集内容</h3>' + titles
        + (detail.message ? '<h3>运行日志</h3><pre id="runLogPre" class="run-log">' + esc(detail.message) + '</pre>' : '<div class="empty">这次运行没有日志。</div>');
      const logPre = qs("runLogPre");
      if (logPre) requestAnimationFrame(() => { logPre.scrollTop = logPre.scrollHeight; });
    }
    function runDetailFor(id) {
      return ((state.data && state.data.run_details) || []).find(r => Number(r.id) === Number(id));
    }
    function renderPublish(p) { return '<div class="item"><h3>' + esc(p.status || "published") + '</h3><div class="meta"><span>draft ' + p.draft_id + '</span><span>' + esc(p.published_at) + '</span></div>' + (p.note_url ? '<pre>' + esc(p.note_url) + '</pre>' : '') + '</div>'; }
    function renderReport(r) { return '<div class="item"><div class="score">' + r.performance_score + '</div><h3>' + esc(r.summary) + '</h3><div class="meta"><span>互动基点 ' + r.engagement_rate_basis + '</span><span>涨粉基点 ' + r.follow_rate_basis + '</span></div><pre>' + esc(r.suggested_adjustment) + '</pre></div>'; }
    function renderList(id, items, renderItem) {
      const el = qs(id);
      if (!items || items.length === 0) { el.innerHTML = '<div class="empty">暂无数据</div>'; return; }
      el.innerHTML = items.map(renderItem).join("");
    }
    function renderKeyOptions(keys, selected) {
      const select = qs("keyName");
      const current = selected || select.value;
      if (!keys || keys.length === 0) { select.innerHTML = '<option value="' + esc(current) + '">' + esc(current || "先测试密钥分发服务") + '</option>'; return; }
      select.innerHTML = keys.map(name => '<option value="' + esc(name) + '">' + esc(name) + '</option>').join("");
      if (current && keys.includes(current)) select.value = current;
    }
    function esc(value) { return String(value ?? "").replace(/[&<>"']/g, ch => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch])); }
    function field(obj, snake, pascal) { return obj && obj[snake] !== undefined ? obj[snake] : (obj ? obj[pascal] : ""); }
    function num(value) { return value === null || value === undefined ? "-" : value; }
    function detailStatusText(item) {
      const status = item.detail_status || "unknown";
      if (status === "succeeded") return "详情成功";
      if (status === "failed") return "详情失败";
      if (status === "search_only") return "仅搜索卡片";
      if (status === "local_completed") return "本机已补全";
      if (status === "local_failed") return "本机补全失败";
      return status;
    }
    function detailMetaBlock(item) {
      const missing = item.missing_fields || [];
      const status = item.detail_status || "";
      if (missing.length === 0 && status !== "failed" && status !== "local_failed" && status !== "search_only") return "";
      return '<div class="detail-pair"><div class="detail-line"><strong>详情状态</strong>' + esc(detailStatusText(item)) + '</div><div class="detail-line"><strong>缺失字段</strong>' + esc(missing.join(" / ") || "-") + '</div></div>';
    }
    function shouldShowDetailFailure(item) {
      const status = item.detail_status || "";
      return Boolean(item.detail_message) && (status.includes("failed") || status === "failed");
    }
    function topicBody() { return { engine: qs("topicEngine").value, limit: Number(qs("topicLimit").value || 10) }; }
    function draftBody() { return { engine: qs("draftEngine").value, limit: Number(qs("draftLimit").value || 10) }; }
    async function runAction(statusId, label, path, body) {
      setStatus(statusId, label + "中...");
      await api(path, { method: "POST", body: JSON.stringify(body) });
      await refresh();
      setStatus(statusId, label + "完成", "ok");
    }
    document.querySelectorAll(".nav-btn").forEach(btn => btn.onclick = () => switchView(btn.dataset.view));
    window.selectItem = (id) => { state.selectedItemId = id; render(); };
    window.selectRun = (id) => { state.selectedRunId = id; render(); };
    window.selectRunIfNoText = (id) => {
      const selection = window.getSelection ? String(window.getSelection()) : "";
      if (selection.trim()) return;
      selectRun(id);
    };
    window.toggleTargetSelection = (id, checked) => {
      if (checked) state.selectedTargetIds.add(id); else state.selectedTargetIds.delete(id);
    };
    qs("saveKey").onclick = async () => {
      try {
        setStatus("keyStatus", "测试中...");
        const result = await api("/api/key-config/test", { method: "POST", body: JSON.stringify({ base_url: qs("keyBase").value, username: qs("keyUser").value, password: qs("keyPass").value, key_name: qs("keyName").value }) });
        renderKeyOptions(result.available_keys || [], result.key_name || "");
        setStatus("keyStatus", "密钥分发服务可用，当前密钥：" + (result.key_name || "-"), "ok");
        await refresh();
      } catch (err) { setStatus("keyStatus", err.message, "error"); }
    };
    qs("keyName").onchange = async () => {
      try {
        await api("/api/key-config", { method: "POST", body: JSON.stringify({ key_name: qs("keyName").value }) });
        setStatus("keyStatus", "当前密钥：" + qs("keyName").value, "ok");
        await refresh();
      } catch (err) { setStatus("keyStatus", err.message, "error"); }
    };
    function targetPayload() {
      return { id: state.editingTargetId, kind: "keyword", name: qs("targetName").value, keyword: qs("targetKeyword").value, interval: Number(qs("targetInterval").value || 300) };
    }
    function resetTargetForm() {
      state.editingTargetId = 0;
      qs("addTarget").textContent = "新增目标";
      qs("cancelTargetEdit").hidden = true;
      qs("targetName").value = "AI工具";
      qs("targetKeyword").value = "AI工具";
      qs("targetInterval").value = "300";
    }
    window.editTarget = (id) => {
      const target = (state.data.targets || []).find(t => Number(field(t, "id", "ID")) === id);
      if (!target) return;
      state.editingTargetId = id;
      qs("targetName").value = field(target, "name", "Name");
      qs("targetKeyword").value = field(target, "keyword", "Keyword");
      qs("targetInterval").value = field(target, "min_interval_seconds", "MinIntervalSeconds") || 300;
      qs("addTarget").textContent = "保存修改";
      qs("cancelTargetEdit").hidden = false;
      setStatus("collectStatus", "正在编辑目标：" + field(target, "name", "Name"));
    };
    window.deleteTarget = async (id) => {
      try {
        await api("/api/targets/delete", { method: "POST", body: JSON.stringify({ id }) });
        if (state.editingTargetId === id) resetTargetForm();
        state.selectedTargetIds.delete(id);
        await refresh();
        setStatus("collectStatus", "目标已删除", "ok");
      } catch (err) { setStatus("collectStatus", err.message, "error"); }
    };
    qs("cancelTargetEdit").onclick = () => { resetTargetForm(); setStatus("collectStatus", ""); };
    qs("addTarget").onclick = async () => {
      const label = state.editingTargetId ? "保存修改" : "新增目标";
      await runAction("collectStatus", label, "/api/targets", targetPayload());
      resetTargetForm();
    };
    function collectRequestBody() {
      const targetIds = Array.from(state.selectedTargetIds);
      if (targetIds.length === 0) return null;
      return { target_ids: targetIds, item_limit: Number(qs("collectItemLimit").value || 3) };
    }
    async function runCollect(body, fromLoginRetry = false) {
      let progressTimer = null;
      try {
        setStatus("collectRunStatus", fromLoginRetry ? "登录成功，继续采集..." : "采集中...");
        state.selectedRunId = 0;
        progressTimer = setInterval(() => refresh().catch(() => {}), 2000);
        const result = await api("/api/collect/once", { method: "POST", body: JSON.stringify(body) });
        clearInterval(progressTimer);
        progressTimer = null;
        await refresh();
        const titles = (result.runs || []).flatMap(r => r.titles || []).filter(Boolean);
        state.pendingCollectBody = null;
        state.retryingCollectAfterLogin = false;
        if (Number(result.item_count || 0) === 0) {
          setStatus("collectRunStatus", "采集完成：没有新增内容。可能是本次检查的结果都已采集，详情见运行记录。", "ok");
        } else {
          setStatus("collectRunStatus", "采集完成：目标 " + result.target_count + " 个，内容 " + result.item_count + " 条" + (titles.length ? "；" + titles.slice(0, 5).join(" / ") : ""), "ok");
        }
      } catch (err) {
        if (progressTimer) clearInterval(progressTimer);
        if (isXHSLoginExpired(err.message) && !state.retryingCollectAfterLogin) {
          state.pendingCollectBody = body;
          setStatus("collectRunStatus", "小红书登录态已失效，请在右侧扫码验证；验证成功后会自动继续采集。");
          startXHSLogin(true);
        } else {
          setStatus("collectRunStatus", err.message, "error");
          state.pendingCollectBody = null;
          state.retryingCollectAfterLogin = false;
        }
      }
    };
    qs("collect").onclick = async () => {
      const body = collectRequestBody();
      if (!body) { setStatus("collectRunStatus", "请先勾选采集目标", "error"); return; }
      await runCollect(body);
    };
    async function startXHSLogin(auto = false) {
      try {
        setStatus("xhsLoginStatus", auto ? "需要扫码验证，正在生成二维码..." : "正在生成二维码...");
        qs("xhsLoginBox").innerHTML = "";
        const loginState = await api("/api/xhs-login/qrcode", { method: "POST", body: JSON.stringify({ force: true, auto }) });
        renderXHSLogin(loginState);
        if (loginState.status === "waiting" || loginState.status === "starting") pollXHSLogin();
        if (loginState.status === "succeeded" && state.pendingCollectBody) {
          const pending = state.pendingCollectBody;
          state.retryingCollectAfterLogin = true;
          await runCollect(pending, true);
        }
      } catch (err) { setStatus("xhsLoginStatus", err.message, "error"); }
    }
    qs("startXHSLogin").onclick = () => startXHSLogin(false);
    qs("logoutXHS").onclick = async () => {
      try {
        setStatus("xhsLoginStatus", "正在登出...");
        qs("xhsLoginBox").innerHTML = "";
        const loginState = await api("/api/xhs-login/logout", { method: "POST", body: "{}" });
        renderXHSLogin(loginState);
      } catch (err) { setStatus("xhsLoginStatus", err.message, "error"); }
    };
    function renderXHSLogin(login) {
      const statusKind = login.status === "succeeded" ? "ok" : (login.status === "failed" ? "error" : "");
      setStatus("xhsLoginStatus", login.message || login.status || "", statusKind);
      if (login.status === "succeeded") {
        qs("xhsLoginBox").innerHTML = "";
      } else if (login.qrcode_data_url) {
        qs("xhsLoginBox").innerHTML = '<img alt="小红书登录二维码" src="' + esc(login.qrcode_data_url) + '"><div class="meta"><span>扫码后等待页面提示成功</span><span>' + esc(login.cookie_path || "data/cookies.json") + '</span></div>';
      }
    }
    async function pollXHSLogin() {
      for (let i = 0; i < 60; i++) {
        await new Promise(resolve => setTimeout(resolve, 3000));
        const login = await api("/api/xhs-login/status");
        renderXHSLogin(login);
        if (login.status === "succeeded") {
          if (state.pendingCollectBody) {
            const pending = state.pendingCollectBody;
            state.retryingCollectAfterLogin = true;
            await runCollect(pending, true);
          }
          return;
        }
        if (login.status === "failed") return;
      }
    }
    function isXHSLoginExpired(message) {
      const text = String(message || "");
      return text.includes("小红书登录态已失效") ||
        text.includes("小红书需要扫码验证") ||
        text.includes("请打开小红书App扫码查看") ||
        text.includes("Sorry, This Page Isn't Available Right Now");
    }
    qs("analyze").onclick = () => runAction("topicStatus", "拆解", "/api/analyze/batch", topicBody());
    qs("score").onclick = () => runAction("topicStatus", "评分", "/api/score/batch", topicBody());
    qs("draft").onclick = () => runAction("draftStatus", "生成草稿", "/api/draft/batch", draftBody());
    qs("reviewScore").onclick = () => runAction("reviewStatus", "复盘评分", "/api/review/score", { limit: Number(qs("reviewLimit").value || 10) });
    refresh().catch(err => setStatus("collectRunStatus", err.message, "error"));
  </script>
</body>
</html>`
