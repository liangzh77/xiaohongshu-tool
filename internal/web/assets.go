package web

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>小红书内容工作台</title>
  <style>
    :root {
      --bg: #f6f7f9;
      --surface: #ffffff;
      --surface-2: #f0f3f6;
      --text: #18202a;
      --muted: #687382;
      --border: #d8dee6;
      --accent: #1967d2;
      --accent-dark: #124ea0;
      --danger: #b42318;
      --ok: #16794f;
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
    header {
      background: var(--surface);
      border-bottom: 1px solid var(--border);
      padding: 16px 24px;
      position: sticky;
      top: 0;
      z-index: 10;
    }
    h1 { margin: 0; font-size: 20px; font-weight: 700; }
    h2 { margin: 0 0 12px; font-size: 15px; }
    h3 { margin: 0 0 6px; font-size: 14px; }
    main { padding: 20px 24px 40px; max-width: 1500px; margin: 0 auto; }
    .subhead { color: var(--muted); margin-top: 4px; }
    .toolbar, .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
    }
    .toolbar { padding: 16px; margin-bottom: 16px; }
    .grid { display: grid; grid-template-columns: repeat(12, 1fr); gap: 16px; }
    .span-4 { grid-column: span 4; }
    .span-6 { grid-column: span 6; }
    .span-8 { grid-column: span 8; }
    .span-12 { grid-column: span 12; }
    .panel { padding: 14px; min-height: 160px; }
    .row { display: flex; gap: 8px; align-items: end; flex-wrap: wrap; }
    .field { display: grid; gap: 5px; min-width: 150px; flex: 1; }
    label { color: var(--muted); font-size: 12px; font-weight: 600; }
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
      font-weight: 600;
      cursor: pointer;
    }
    button.primary { background: var(--accent); border-color: var(--accent); color: #fff; }
    button.primary:hover { background: var(--accent-dark); }
    button:disabled { opacity: .6; cursor: not-allowed; }
    .item {
      border-top: 1px solid var(--border);
      padding: 10px 0;
    }
    .item:first-child { border-top: 0; padding-top: 0; }
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
      max-height: 280px;
      overflow: auto;
    }
    .empty { color: var(--muted); padding: 20px 0; }
    @media (max-width: 980px) {
      header, main { padding-left: 14px; padding-right: 14px; }
      .span-4, .span-6, .span-8 { grid-column: span 12; }
    }
  </style>
</head>
<body>
  <header>
    <h1>小红书内容工作台</h1>
    <div class="subhead">采集、拆解、评分、生成和复盘，使用本地 SQLite/WAL 存储</div>
  </header>
  <main>
    <section class="toolbar" aria-labelledby="config-title">
      <h2 id="config-title">密钥分发服务</h2>
      <div class="row">
        <div class="field"><label for="keyBase">服务地址</label><input id="keyBase" placeholder="https://distribute-keys.vercel.app"></div>
        <div class="field"><label for="keyUser">用户名</label><input id="keyUser" autocomplete="username"></div>
        <div class="field"><label for="keyPass">密码</label><input id="keyPass" type="password" autocomplete="current-password"></div>
        <button class="primary" id="saveKey">测试并保存</button>
      </div>
      <div class="status" id="keyStatus"></div>
    </section>

    <section class="toolbar" aria-labelledby="actions-title">
      <h2 id="actions-title">工作流</h2>
      <div class="row">
        <div class="field"><label for="engine">模型</label><select id="engine"><option value="rule">规则版</option><option value="llm">大模型版</option></select></div>
        <div class="field"><label for="limit">数量</label><input id="limit" type="number" min="1" max="100" value="10"></div>
        <div class="field"><label for="collectorCmd">采集命令</label><input id="collectorCmd" value="go run ./cmd/xhs-native-collector --limit 3 --details=false"></div>
        <button id="collect">采集一次</button>
        <button id="analyze">拆解</button>
        <button id="score">评分</button>
        <button id="draft">生成草稿</button>
        <button id="reviewScore">复盘评分</button>
        <button class="primary" id="refresh">刷新</button>
      </div>
      <div class="status" id="actionStatus"></div>
    </section>

    <section class="toolbar" aria-labelledby="target-title">
      <h2 id="target-title">采集目标</h2>
      <div class="row">
        <div class="field"><label for="targetName">名称</label><input id="targetName" value="AI工具"></div>
        <div class="field"><label for="targetKeyword">关键词</label><input id="targetKeyword" value="AI工具"></div>
        <div class="field"><label for="targetInterval">间隔秒</label><input id="targetInterval" type="number" value="300"></div>
        <button id="addTarget">新增目标</button>
      </div>
    </section>

    <section class="grid" aria-label="内容工作区">
      <div class="panel span-4"><h2>目标</h2><div id="targets"></div></div>
      <div class="panel span-8"><h2>采集内容</h2><div id="items"></div></div>
      <div class="panel span-6"><h2>候选选题</h2><div id="candidates"></div></div>
      <div class="panel span-6"><h2>草稿</h2><div id="drafts"></div></div>
      <div class="panel span-6"><h2>运行记录</h2><div id="runs"></div></div>
      <div class="panel span-6"><h2>复盘报告</h2><div id="reports"></div></div>
    </section>
  </main>
  <script>
    const qs = (id) => document.getElementById(id);
    const state = { data: null };

    async function api(path, options = {}) {
      const resp = await fetch(path, {
        ...options,
        headers: { "Content-Type": "application/json", ...(options.headers || {}) },
      });
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
      render();
    }

    function render() {
      const data = state.data || {};
      renderList("targets", data.targets, t => '<div class="item"><h3>' + esc(field(t, "name", "Name")) + '</h3><div class="meta"><span>' + esc(field(t, "kind", "Kind")) + '</span><span>' + esc(field(t, "keyword", "Keyword") || field(t, "url", "URL")) + '</span><span>' + field(t, "min_interval_seconds", "MinIntervalSeconds") + 's</span></div></div>');
      renderList("items", data.items, item => '<div class="item"><h3>' + esc(item.title || "无标题") + '</h3><div class="meta"><span>' + esc(item.author_name || "未知作者") + '</span><span>赞 ' + num(item.like_count) + '</span><span>藏 ' + num(item.collect_count) + '</span><span>评 ' + num(item.comment_count) + '</span></div>' + (item.body ? '<pre>' + esc(item.body) + '</pre>' : '') + '</div>');
      renderList("candidates", data.candidates, c => '<div class="item"><div class="score">' + c.total_score + '</div><h3>' + esc(c.topic) + '</h3><div class="meta"><span>趋势 ' + c.trend_score + '</span><span>涨粉 ' + c.growth_score + '</span><span>风险 ' + c.risk_score + '</span><span>' + esc(c.scoring_model) + '</span></div><pre>' + esc(c.reason || "") + '</pre></div>');
      renderList("drafts", data.drafts, d => '<div class="item"><h3>' + esc((d.title_options || [])[0] || "草稿") + '</h3><div class="meta"><span>' + esc(d.generator) + '</span><span>' + (d.tags || []).map(esc).join(" / ") + '</span></div><pre>' + esc(d.opening + "\n\n" + d.body) + '</pre></div>');
      renderList("runs", data.runs, r => '<div class="item"><h3>' + esc(r.status) + ' <span class="pill">' + esc(r.mode) + '</span></h3><div class="meta"><span>' + esc(r.target_name) + '</span><span>' + esc(r.started_at) + '</span></div>' + (r.message ? '<pre>' + esc(r.message) + '</pre>' : '') + '</div>');
      renderList("reports", data.reports, r => '<div class="item"><div class="score">' + r.performance_score + '</div><h3>' + esc(r.summary) + '</h3><div class="meta"><span>互动基点 ' + r.engagement_rate_basis + '</span><span>涨粉基点 ' + r.follow_rate_basis + '</span></div><pre>' + esc(r.suggested_adjustment) + '</pre></div>');
    }

    function renderList(id, items, renderItem) {
      const el = qs(id);
      if (!items || items.length === 0) {
        el.innerHTML = '<div class="empty">暂无数据</div>';
        return;
      }
      el.innerHTML = items.map(renderItem).join("");
    }

    function esc(value) {
      return String(value ?? "").replace(/[&<>"']/g, ch => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch]));
    }
    function field(obj, snake, pascal) { return obj && obj[snake] !== undefined ? obj[snake] : (obj ? obj[pascal] : ""); }
    function num(value) { return value === null || value === undefined ? "-" : value; }
    function actionBody() { return { engine: qs("engine").value, limit: Number(qs("limit").value || 10) }; }
    async function runAction(label, path, body) {
      setStatus("actionStatus", label + "中...");
      await api(path, { method: "POST", body: JSON.stringify(body) });
      await refresh();
      setStatus("actionStatus", label + "完成", "ok");
    }

    qs("saveKey").onclick = async () => {
      try {
        setStatus("keyStatus", "测试中...");
        await api("/api/key-config/test", { method: "POST", body: JSON.stringify({ base_url: qs("keyBase").value, username: qs("keyUser").value, password: qs("keyPass").value }) });
        setStatus("keyStatus", "密钥分发服务可用，配置已保存到当前服务进程", "ok");
        await refresh();
      } catch (err) { setStatus("keyStatus", err.message, "error"); }
    };
    qs("addTarget").onclick = () => runAction("新增目标", "/api/targets", { kind: "keyword", name: qs("targetName").value, keyword: qs("targetKeyword").value, interval: Number(qs("targetInterval").value || 300) });
    qs("collect").onclick = () => runAction("采集", "/api/collect/once", { command: qs("collectorCmd").value, limit: 1 });
    qs("analyze").onclick = () => runAction("拆解", "/api/analyze/batch", actionBody());
    qs("score").onclick = () => runAction("评分", "/api/score/batch", actionBody());
    qs("draft").onclick = () => runAction("生成草稿", "/api/draft/batch", actionBody());
    qs("reviewScore").onclick = () => runAction("复盘评分", "/api/review/score", { limit: Number(qs("limit").value || 10) });
    qs("refresh").onclick = () => refresh().catch(err => setStatus("actionStatus", err.message, "error"));
    refresh().catch(err => setStatus("actionStatus", err.message, "error"));
  </script>
</body>
</html>`
