// devstack dashboard. Polls /api/state and redraws only the parts that changed,
// so open dropdowns and focus survive the refresh.
"use strict";

const $ = (sel) => document.querySelector(sel);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

const ICONS = {
  play: '<path d="M5 3.5v9l7.5-4.5z" fill="currentColor"/>',
  stop: '<rect x="4" y="4" width="8" height="8" rx="1" fill="currentColor"/>',
  folder: '<path d="M2 4.5A1.5 1.5 0 0 1 3.5 3h3l1.5 1.5h4.5A1.5 1.5 0 0 1 14 6v5.5a1.5 1.5 0 0 1-1.5 1.5h-9A1.5 1.5 0 0 1 2 11.5z" fill="none" stroke="currentColor" stroke-width="1.2"/>',
  reload: '<path d="M13 8a5 5 0 1 1-1.5-3.6M13 2.5v3h-3" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>',
  open: '<path d="M9 3h4v4M13 3 7.5 8.5M11 9.5V12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h2.5" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>',
  check: '<path d="m3.5 8.5 3 3 6-7" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>',
  cross: '<path d="m4.5 4.5 7 7m0-7-7 7" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>',
  ring: '<circle cx="8" cy="8" r="5" fill="none" stroke="currentColor" stroke-width="1.4"/>',
  spin: '<path d="M8 3a5 5 0 1 0 5 5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><animateTransform attributeName="transform" type="rotate" from="0 8 8" to="360 8 8" dur=".9s" repeatCount="indefinite"/></path>',
};
const icon = (name, cls = "") => `<svg class="${cls}" viewBox="0 0 16 16" aria-hidden="true">${ICONS[name]}</svg>`;

async function api(path, body) {
  const opts = body === undefined ? {} : {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Devstack": "1" },
    body: JSON.stringify(body),
  };
  const r = await fetch(path, opts);
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || r.statusText);
  return data;
}

let toastTimer;
function toast(msg, isError) {
  const t = $("#toast");
  t.textContent = msg;
  t.className = "show" + (isError ? " error" : "");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (t.className = ""), isError ? 7000 : 2800);
}

// act runs an action, shows its error, and refreshes state right away.
async function act(path, body, okMsg) {
  try {
    const res = await api(path, body ?? {});
    if (okMsg) toast(okMsg);
    poll(true);
    return res;
  } catch (e) {
    toast(e.message, true);
    poll(true);
    throw e;
  }
}

const open = (kind, target) => act("/api/open", { kind, target }).catch(() => {});
// Binary units, matching the CLI.
const size = (n) => n >= 2 ** 30 ? (n / 2 ** 30).toFixed(1) + " GB" : n >= 2 ** 20 ? (n / 2 ** 20).toFixed(1) + " MB" : n >= 2 ** 10 ? (n / 2 ** 10).toFixed(0) + " KB" : n + " B";

// ---- state + rendering -------------------------------------------------------

let S = null;
const TABS = ["sites", "servers", "logs", "settings"];
let tab = [location.hash.slice(1), localStorageGet("tab")].find((t) => TABS.includes(t)) || "sites";
const drawn = {}; // section -> key of what is on screen

function localStorageGet(k) { try { return localStorage.getItem(k); } catch { return null; } }
function localStorageSet(k, v) { try { localStorage.setItem(k, v); } catch {} }

// draw replaces a section only when its content key changes.
function draw(section, key, fn) {
  const k = JSON.stringify(key);
  if (drawn[section] === k) return;
  drawn[section] = k;
  fn();
}

let pollTimer;
async function poll(now) {
  clearTimeout(pollTimer);
  try {
    S = await api("/api/state");
    render();
  } catch (e) {
    $("#state-line") && ($("#state-line").textContent = "Lost contact with the devstack app: " + e.message);
  }
  const busy = S && S.job && !S.job.finished;
  pollTimer = setTimeout(poll, busy ? 400 : 1500);
}

function render() {
  const setupMode = S.needs_setup || (S.job && S.job.title === "Setting up devstack");
  $("#setup").hidden = !setupMode;
  $("#dash").hidden = setupMode;
  if (setupMode) return renderSetup();
  renderStrip();
  renderTabs();
  if (tab === "sites") renderSites();
  if (tab === "servers") renderServers();
  if (tab === "logs") renderLogs();
  if (tab === "settings") renderSettings();
}

function overall() {
  const job = S.job && !S.job.finished ? S.job : null;
  if (job) return { word: job.title.replace(/ servers$/, "").toUpperCase() + "…", color: "var(--warn)", busy: true };
  if (!S.running) return { word: "STOPPED", color: "var(--text-3)" };
  if (S.phase === "starting") return { word: "STARTING…", color: "var(--warn)", busy: true };
  if (S.phase === "stopping") return { word: "STOPPING…", color: "var(--warn)", busy: true };
  if (S.groups.some((g) => g.state !== "running")) return { word: "NEEDS ATTENTION", color: "var(--danger)" };
  return { word: "RUNNING", color: "var(--live)", live: true };
}

function renderStrip() {
  const o = overall();
  const strip = $("#strip");
  strip.style.setProperty("--state", o.color);
  strip.classList.toggle("live", !!o.live);
  strip.classList.toggle("busy", !!o.busy);
  $("#state-word").textContent = o.word;

  const n = S.sites.length;
  const sites = `${n} site${n === 1 ? "" : "s"}`;
  let line;
  const job = S.job && !S.job.finished ? S.job : null;
  if (job) {
    const cur = [...job.steps].reverse().find((s) => s.state !== "pending");
    line = cur ? cur.label : "";
  } else if (S.job && S.job.error) {
    line = S.job.error.split("\n")[0];
  } else if (S.errors.length) {
    line = S.errors[0].split("\n")[0];
  } else if (S.running) {
    line = `Serving ${sites} · ${S.root_url}`;
  } else {
    line = `Start to serve ${sites} from ${S.www || S.root}`;
  }
  $("#state-line").textContent = line;
  $("#state-line").title = line;

  const services = byRole(S.running && S.groups.length
    ? S.groups.map((g) => ({ name: g.name, version: g.version, state: g.state }))
    : S.planned.map((p) => ({ name: p.name, version: p.version, state: "stopped" })));
  draw("pills", services, () => {
    $("#pills").innerHTML = services.map((s) =>
      `<span class="pill ${esc(s.state)}" title="${esc(s.name)} ${esc(s.version)}: ${esc(s.state)}"><i></i>${esc(product(s.name))} ${esc(short(s.version))}</span>`).join("");
  });

  const btn = $("#power");
  const busy = !!job || S.phase === "starting" || S.phase === "stopping";
  draw("power", [S.running, busy], () => {
    btn.className = S.running ? "btn" : "btn primary";
    btn.innerHTML = S.running ? icon("stop") + "Stop" : icon("play") + "Start";
    btn.disabled = busy;
    btn.onclick = () => act(S.running ? "/api/stop" : "/api/start").catch(() => {});
  });
}

const label = (name) => name.replace(/^php-/, "PHP ").replace(/^mysql$/, "MySQL");
const product = (name) => name.startsWith("php-") ? "PHP" : name === "mysql" ? "MySQL" : name;
const short = (v) => v.split(".").slice(0, 2).join(".");
// byRole orders services the way requests flow: web server, PHP, database.
const role = (name) => name === "nginx" ? 0 : name.startsWith("php-") ? 1 : 2;
const byRole = (list) => [...list].sort((a, b) => role(a.name) - role(b.name) || a.name.localeCompare(b.name));

function renderTabs() {
  const update = S.update && S.update.version;
  draw("tabs", [tab, update], () => {
    document.querySelectorAll("#tabs button").forEach((b) => b.setAttribute("aria-selected", b.dataset.tab === tab));
    const set = $('#tabs [data-tab="settings"]');
    set.innerHTML = "Settings" + (update ? '<i class="tab-dot" title="Update available"></i>' : "");
    set.setAttribute("aria-label", update ? "Settings, update available" : "Settings");
    document.querySelectorAll(".view").forEach((v) => v.classList.toggle("active", v.dataset.view === tab));
  });
  draw("bar-actions", [tab, S.running, S.www], () => {
    const a = $("#bar-actions");
    if (tab === "sites") {
      a.innerHTML = `<button class="btn" id="open-www">${icon("folder")}Open www folder</button>
        <button class="btn" id="reload" ${S.running ? "" : "disabled"} title="Pick up new folders and config changes">${icon("reload")}Reload</button>`;
      $("#open-www").onclick = () => open("folder", S.www);
      $("#reload").onclick = () => act("/api/reload", {}, "Reloaded").catch(() => {});
    } else {
      a.innerHTML = "";
    }
  });
}

function renderSites() {
  draw("sites", [S.sites, S.phps, S.default_php, S.www], () => {
    const el = $("#sites");
    if (!S.sites.length) {
      el.innerHTML = `<div class="empty"><p>No sites yet. Every folder in <span class="mono">${esc(S.www)}</span> becomes a site.</p>
        <button class="btn" id="empty-open">${icon("folder")}Open www folder</button></div>`;
      $("#empty-open").onclick = () => open("folder", S.www);
      return;
    }
    const phpOptions = (site) => {
      const opts = [`<option value="" ${site.pinned ? "" : "selected"}>Default (${esc(S.default_php)})</option>`];
      for (const v of S.phps || []) opts.push(`<option value="${esc(v)}" ${site.pinned && site.php === v ? "selected" : ""}>PHP ${esc(v)}</option>`);
      return opts.join("");
    };
    el.innerHTML = `<table>
      <thead><tr><th>Site</th><th>Address</th><th>PHP</th><th></th></tr></thead>
      <tbody>${S.sites.map((s, i) => `
        <tr>
          <td><div class="site-name">${esc(s.slug)}</div>${s.name !== s.slug ? `<div class="site-sub">${esc(s.name)}</div>` : ""}</td>
          <td><button class="link" data-url="${i}">${esc(s.url.replace(/^http:\/\//, ""))}</button></td>
          <td><select data-php="${i}" aria-label="PHP version for ${esc(s.slug)}">${phpOptions(s)}</select></td>
          <td class="actions"><button class="btn small ghost" data-folder="${i}" title="${esc(s.dir)}">${icon("folder")}Folder</button></td>
        </tr>`).join("")}
      </tbody></table>
      <p class="muted" style="margin:12px 0 0;font-size:12px">New folders in <span class="mono">${esc(S.www)}</span> show up after Reload.</p>`;
    el.querySelectorAll("[data-url]").forEach((b) => (b.onclick = () => open("url", S.sites[b.dataset.url].url)));
    el.querySelectorAll("[data-folder]").forEach((b) => (b.onclick = () => open("folder", S.sites[b.dataset.folder].dir)));
    el.querySelectorAll("[data-php]").forEach((sel) => (sel.onchange = () => {
      const s = S.sites[sel.dataset.php];
      act("/api/site/php", { slug: s.slug, php: sel.value }, `${s.slug} now uses ${sel.value ? "PHP " + sel.value : "the default PHP"}`).catch(() => {});
    }));
  });
}

let packages = null, db = null, packagesFor = "";
async function loadServerData() {
  try {
    [packages, db] = await Promise.all([api("/api/packages"), api("/api/db")]);
    drawn.packages = drawn.db = null;
    renderServers();
  } catch (e) { toast(e.message, true); }
}

function renderServers() {
  // Refresh package and DB info when the tab opens or a job finishes.
  const marker = [S.running, S.job ? S.job.title + S.job.finished : ""].join("|");
  if (packagesFor !== marker) { packagesFor = marker; loadServerData(); }

  const rows = byRole(S.running && S.groups.length
    ? S.groups
    : S.planned.map((p) => ({ name: p.name, version: p.version, ports: p.ports, state: "stopped", restarts: 0 })));
  draw("services", rows, () => {
    $("#services").innerHTML = `<div class="rows">${rows.map((g) => `
      <div class="row">
        <span class="svc-dot ${esc(g.state)}"></span>
        <div class="grow"><b>${esc(label(g.name))}</b> <span class="muted num">${esc(g.version)}</span>
          ${g.last_error && g.state !== "running" ? `<div class="err">${esc(g.last_error.split("\n")[0])}</div>` : ""}</div>
        <span class="mono muted num">:${esc(g.ports)}</span>
        <span class="state ${esc(g.state)}">${esc(g.state)}${g.restarts ? ` · ${g.restarts} restart${g.restarts === 1 ? "" : "s"}` : ""}</span>
        ${S.running ? `<button class="btn small ghost" data-restart="${esc(g.name)}" title="Restart ${esc(g.name)}">${icon("reload")}</button>` : ""}
      </div>`).join("")}</div>
      ${S.warnings.map((w) => `<div class="notice warn">${esc(w)}</div>`).join("")}`;
    document.querySelectorAll("[data-restart]").forEach((b) => (b.onclick = () => act("/api/restart", { unit: b.dataset.restart }, "Restarted " + b.dataset.restart).catch(() => {})));
  });

  draw("db", [db], () => {
    const el = $("#db");
    if (!db) { el.innerHTML = '<span class="muted">Loading…</span>'; return; }
    if (!db.enabled) { el.innerHTML = '<span class="muted">MySQL is turned off in stack.toml.</span>'; return; }
    if (!db.ready) { el.innerHTML = '<span class="muted">A root password is generated the first time MySQL starts.</span>'; return; }
    el.innerHTML = `<dl class="kv">
      <dt>Host</dt><dd>${esc(db.host)}</dd><span></span>
      <dt>Port</dt><dd class="num">${esc(db.port)}</dd><span></span>
      <dt>User</dt><dd>${esc(db.user)}</dd><span></span>
      <dt>Password</dt><dd id="pw">••••••••••••</dd>
        <span><button class="btn small ghost" id="pw-show">Show</button><button class="btn small ghost" id="pw-copy">Copy</button></span>
    </dl>`;
    $("#pw-show").onclick = (e) => {
      const shown = e.target.textContent === "Hide";
      $("#pw").textContent = shown ? "••••••••••••" : db.password;
      e.target.textContent = shown ? "Show" : "Hide";
    };
    $("#pw-copy").onclick = () => navigator.clipboard.writeText(db.password).then(() => toast("Password copied"), () => toast("Couldn't copy", true));
  });

  const job = S.job && S.job.title.startsWith("Installing") ? S.job : null;
  draw("packages", [packages, job], () => {
    const el = $("#packages");
    if (!packages) { el.innerHTML = '<span class="muted">Loading…</span>'; return; }
    const busy = S.job && !S.job.finished;
    const kinds = [["php", "PHP"], ["mysql", "MySQL"], ["nginx", "nginx"]];
    el.innerHTML = kinds.map(([kind, title]) => `
      <div class="pkg-group"><h3>${title}</h3><div class="rows">
      ${packages.filter((p) => p.kind === kind).map((p) => {
        const installing = job && job.title === `Installing ${p.kind} ${p.version}` && !job.finished;
        const step = installing && job.steps[0];
        let action;
        if (installing) {
          const pct = step && step.total ? Math.round((step.done / step.total) * 100) : 0;
          action = `<div style="width:160px"><div class="bar-track"><div class="bar-fill" style="transform:scaleX(${pct / 100})"></div></div><div class="muted num" style="font-size:11.5px">${pct}% of ${size(p.size)}</div></div>`;
        } else if (p.in_use) {
          action = '<span class="badge live">In use</span>';
        } else if (p.installed) {
          action = `<button class="btn small ghost danger" data-remove="${esc(p.kind)}|${esc(p.version)}" ${busy ? "disabled" : ""}>Remove</button>`;
        } else {
          action = `<button class="btn small" data-install="${esc(p.kind)}|${esc(p.version)}" ${busy ? "disabled" : ""}>Install</button>`;
        }
        return `<div class="row">
          <div class="grow"><b class="num">${esc(p.version)}</b> ${p.label ? `<span class="badge">${esc(p.label)}</span>` : ""}
            ${p.installed && !p.in_use ? '<span class="muted"> · installed</span>' : ""}</div>
          <span class="muted num">${size(p.size)}</span>
          ${action}
        </div>`;
      }).join("")}</div></div>`).join("");
    el.querySelectorAll("[data-install]").forEach((b) => (b.onclick = () => {
      const [kind, version] = b.dataset.install.split("|");
      act("/api/packages/install", { kind, version }).catch(() => {});
    }));
    el.querySelectorAll("[data-remove]").forEach((b) => (b.onclick = () => {
      const [kind, version] = b.dataset.remove.split("|");
      if (!confirm(`Remove ${kind} ${version}? You can download it again later.`)) return;
      act("/api/packages/remove", { kind, version }, `Removed ${kind} ${version}`).then(loadServerData).catch(() => {});
    }));
  });
  if (S.job && S.job.finished && S.job.title.startsWith("Installing")) {
    if (S.job.error) toast(S.job.error, true); else toast(S.job.title.replace("Installing", "Installed"));
    api("/api/job/dismiss", {}).then(loadServerData).catch(() => {});
  }
}

let logTimer, logFiles = [];
async function renderLogs() {
  if (drawn.logsActive) return;
  drawn.logsActive = true;
  await refreshLog();
}
async function refreshLog() {
  clearTimeout(logTimer);
  if (tab !== "logs") { drawn.logsActive = false; return; }
  const sel = $("#log-name");
  const want = sel.value || localStorageGet("log") || "devstack.log";
  try {
    const r = await api("/api/logs?name=" + encodeURIComponent(want));
    if (JSON.stringify(r.files) !== JSON.stringify(logFiles)) {
      logFiles = r.files;
      sel.innerHTML = r.files.map((f) => `<option ${f === want ? "selected" : ""}>${esc(f)}</option>`).join("");
    }
    const pre = $("#log-text");
    const atBottom = pre.scrollTop + pre.clientHeight >= pre.scrollHeight - 30;
    if (pre.textContent !== r.text) {
      pre.textContent = r.text || "(empty)";
      if (atBottom) pre.scrollTop = pre.scrollHeight;
    }
  } catch (e) { toast(e.message, true); }
  if ($("#log-follow").checked) logTimer = setTimeout(refreshLog, 2000);
  else drawn.logsActive = false;
}

function renderSettings() {
  draw("settings", [S.settings, S.root, S.version], () => {
    const sw = (id, on) => `<label class="switch"><input type="checkbox" id="${id}" ${on ? "checked" : ""}><span></span></label>`;
    $("#settings").innerHTML = `
      <h3>Stack</h3>
      <div class="set-card"><div class="grow"><div class="set-title">Stack folder</div><div class="set-desc mono">${esc(S.root)}</div></div>
        <button class="btn small" id="open-root">${icon("folder")}Open</button></div>
      <div class="set-card"><div class="grow"><div class="set-title">stack.toml</div>
        <div class="set-desc">Ports, PHP extensions and php.ini settings. Reload after saving.</div></div>
        <button class="btn small" id="edit-config">${icon("open")}Edit</button></div>
      <h3>Startup</h3>
      <div class="set-card"><div class="grow"><div class="set-title">Start servers when devstack opens</div>
        <div class="set-desc">Your sites are up as soon as the app is.</div></div>${sw("set-start", S.settings.start_servers)}</div>
      <div class="set-card"><div class="grow"><div class="set-title">Open devstack when you sign in</div>
        <div class="set-desc">Starts in the tray, without a window.</div></div>${sw("set-autostart", S.settings.autostart)}</div>
      <h3>Updates</h3>
      <div class="set-card" id="update-card"></div>
      <div class="set-card"><div class="grow"><div class="set-title">Check for updates automatically</div>
        <div class="set-desc">Once a day, from devstack.holostack.in. Updates install only when you choose.</div></div>${sw("set-updates", S.settings.check_updates)}</div>
      <h3>About</h3>
      <div class="set-card"><div class="grow"><div class="set-title">devstack ${esc(S.version)}</div>
        <div class="set-desc">Open source. Runs unmodified builds of nginx (<button class="link" data-link="https://nginx.org/LICENSE">BSD</button>),
        PHP (<button class="link" data-link="https://www.php.net/license/">PHP License</button>) and
        MySQL (<button class="link" data-link="https://www.mysql.com/about/legal/">GPLv2</button>), downloaded from their publishers.</div></div>
        <button class="btn small" data-link="https://github.com/harsha2805/devstack">${icon("open")}Source</button></div>`;
    $("#open-root").onclick = () => open("folder", S.root);
    $("#edit-config").onclick = () => open("config", "");
    $("#set-start").onchange = (e) => act("/api/settings", { start_servers: e.target.checked }).catch(() => {});
    $("#set-autostart").onchange = (e) => act("/api/settings", { autostart: e.target.checked }).catch(() => {});
    $("#set-updates").onchange = (e) => act("/api/settings", { check_updates: e.target.checked }).catch(() => {});
    document.querySelectorAll("#settings [data-link]").forEach((b) => (b.onclick = () => open("url", b.dataset.link)));
    drawn.update = null;
  });
  renderUpdate();
}

// renderUpdate draws the update card: up to date, available, downloading, or failed.
function renderUpdate() {
  const u = S.update;
  const job = S.job && S.job.title === "Updating devstack" ? S.job : null;
  draw("update", [u, job], () => {
    const el = $("#update-card");
    const checked = u.checked ? new Date(u.checked).toLocaleString([], { dateStyle: "medium", timeStyle: "short" }) : "";
    let text, action = "";
    if (!u.release) {
      text = `<div class="set-title">Development build</div><div class="set-desc">Builds without a release version don't update.</div>`;
    } else if (job && !job.finished) {
      const st = job.steps[job.steps.length - 1] || { label: "Preparing", done: 0, total: 0 };
      const pct = st.total ? Math.min(100, Math.round((st.done / st.total) * 100)) : 0;
      text = `<div class="set-title">${esc(st.label)}…</div>
        ${st.total ? `<div class="bar-track update-bar"><div class="bar-fill" style="transform:scaleX(${pct / 100})"></div></div>
        <div class="set-desc num">${size(st.done || 0)} / ${size(st.total)}</div>`
        : `<div class="set-desc">devstack closes and reopens itself once the installer starts.</div>`}`;
    } else if (u.version) {
      text = `<div class="set-title">devstack ${esc(u.version)} is available</div>
        ${u.notes ? `<div class="set-desc notes">${esc(u.notes)}</div>` : ""}
        ${job && job.error ? `<div class="set-desc error-text">${esc(job.error)}</div>` : ""}
        <div class="set-desc">${u.installed ? "Running servers stop for the update and start again afterwards." : "This copy isn't installed, so the download page opens."}</div>`;
      action = `<button class="btn small primary" id="update-now">${u.installed ? "Update now" : "Download"}</button>`;
    } else {
      const status = u.checking ? "Checking…" : u.error ? esc(u.error) : checked ? `You're up to date. Last checked ${esc(checked)}.` : "Not checked yet.";
      text = `<div class="set-title">devstack ${esc(S.version)}</div><div class="set-desc ${u.error && !u.checking ? "error-text" : ""}">${status}</div>`;
      action = `<button class="btn small" id="update-check" ${u.checking ? "disabled" : ""}>${icon("reload")}Check now</button>`;
    }
    el.innerHTML = `<div class="grow">${text}</div>${action}`;
    $("#update-check") && ($("#update-check").onclick = () => {
      $("#update-check").disabled = true;
      api("/api/update/check", {}).then((v) => { if (!v.version) toast("devstack is up to date"); })
        .catch((e) => toast(e.message, true)).finally(() => poll(true));
    });
    $("#update-now") && ($("#update-now").onclick = () => {
      if (job && job.error) api("/api/job/dismiss", {}).catch(() => {});
      act("/api/update/install", {}).catch(() => {});
    });
  });
}

// ---- first-run setup ----------------------------------------------------------

const choice = { php: null, mysql: null, root: null };

function renderSetup() {
  const job = S.job && S.job.title === "Setting up devstack" ? S.job : null;
  if (job) return drawSetupProgress(job);
  const o = S.setup;
  if (!o) return;
  if (choice.php === null) choice.php = (o.php.find((x) => x.default) || o.php[0] || {}).value;
  if (choice.mysql === null) choice.mysql = (o.mysql.find((x) => x.default) || o.mysql[0] || {}).value || "off";
  if (choice.root === null) choice.root = o.root;
  draw("setup", ["form", o], () => {
    const seg = (name, opts) => `<div class="segmented" role="radiogroup">${opts.map((x) =>
      `<button type="button" data-${name}="${esc(x.value)}" aria-selected="${choice[name] === x.value}">${esc(x.text)}</button>`).join("")}</div>`;
    const phpOpts = o.php.map((x) => ({ value: x.value, text: x.value }));
    const mysqlOpts = o.mysql.map((x) => ({ value: x.value, text: x.value + (x.label ? " " + x.label.toUpperCase() : "") }))
      .concat([{ value: "off", text: "None" }]);
    $("#setup-card").innerHTML = `
      <h1>Set up devstack</h1>
      <p class="lead">nginx, PHP and MySQL for every project folder, from one place. Pick where it lives and which versions to run.</p>
      <div class="field"><label for="root">Folder for your sites and servers</label>
        <input type="text" id="root" value="${esc(choice.root)}" spellcheck="false">
        <div class="hint" id="root-hint">Projects go in <span class="mono" id="root-www"></span>. Use a short path without spaces.</div></div>
      <div class="field"><span class="label">PHP</span>${seg("php", phpOpts)}</div>
      <div class="field"><span class="label">MySQL</span>${seg("mysql", mysqlOpts)}</div>
      <div class="plan" id="plan"></div>
      ${o.vc_runtime ? "" : `<div class="notice warn" style="margin-bottom:12px">PHP and MySQL need Microsoft's Visual C++ runtime, which isn't installed.
        <button class="link" id="vc">Get it from Microsoft</button>, then continue.</div>`}
      <button class="btn primary wide" id="go">${icon("play")}Download and start</button>`;
    const update = () => {
      choice.root = $("#root").value.trim();
      $("#root-www").textContent = choice.root.replace(/[\\/]+$/, "") + "\\www";
      const bad = !/^[A-Za-z]:\\[\x21-\x7e]*$/.test(choice.root);
      $("#root-hint").classList.toggle("bad", bad);
      $("#go").disabled = bad;
      const php = o.php.find((x) => x.value === choice.php);
      const my = o.mysql.find((x) => x.value === choice.mysql);
      const items = [o.nginx, php, my].filter(Boolean);
      const names = ["nginx " + o.nginx.version, php && "PHP " + php.version, my && "MySQL " + my.version].filter(Boolean);
      const total = items.reduce((n, x) => n + x.size, 0);
      $("#plan").textContent = `Downloads ${names.join(", ").replace(/, ([^,]*)$/, " and $1")} (${size(total)}) from their official sites, checks each file, then starts them.`;
      document.querySelectorAll("[data-php]").forEach((b) => b.setAttribute("aria-selected", b.dataset.php === choice.php));
      document.querySelectorAll("[data-mysql]").forEach((b) => b.setAttribute("aria-selected", b.dataset.mysql === choice.mysql));
    };
    document.querySelectorAll("[data-php]").forEach((b) => (b.onclick = () => { choice.php = b.dataset.php; update(); }));
    document.querySelectorAll("[data-mysql]").forEach((b) => (b.onclick = () => { choice.mysql = b.dataset.mysql; update(); }));
    $("#root").oninput = update;
    $("#vc") && ($("#vc").onclick = () => open("url", S.links.vc_redist));
    $("#go").onclick = () => act("/api/setup", { root: choice.root, php: choice.php, mysql: choice.mysql }).catch(() => {});
    update();
  });
}

function drawSetupProgress(job) {
  draw("setup", ["job", job], () => {
    const stepIcon = (s) => s.state === "done" ? icon("check", "icon") : s.state === "failed" ? icon("cross", "icon")
      : s.state === "running" ? icon("spin", "icon") : icon("ring", "icon");
    // Setup itself can succeed while starting the servers fails (a busy port, say).
    const created = job.steps.some((s) => s.label.startsWith("Create ") && s.state === "done")
      || (job.steps.length > 0 && !job.steps.some((s) => s.label.startsWith("Create ")));
    const startFailed = job.error && created;
    let title = "Setting up devstack", lead = "This takes a minute or two, mostly downloading.";
    if (startFailed) {
      title = "Set up, but not started";
      lead = "devstack is installed. The servers couldn't start because of the problem below; fix it, then press Start.";
    } else if (job.error) {
      title = "Setup stopped";
      lead = "Finished downloads are kept and nothing partial was left behind. Fix the problem below and try again.";
    } else if (job.finished) {
      title = "devstack is ready";
      lead = "Your servers are running.";
    }
    $("#setup-card").innerHTML = `
      <h1>${title}</h1>
      <p class="lead">${lead}</p>
      <div class="steps">${job.steps.map((s) => {
        const pct = s.total ? Math.min(100, Math.round((s.done / s.total) * 100)) : 0;
        return `<div class="step ${esc(s.state)}">${stepIcon(s)}<span>${esc(s.label)}</span>
          <span class="amount num">${!s.total ? "" : s.state === "pending" ? size(s.total) : `${size(s.done || 0)} / ${size(s.total)}`}</span>
          ${s.total ? `<div class="bar-track"><div class="bar-fill" style="transform:scaleX(${pct / 100})"></div></div>` : ""}</div>`;
      }).join("")}</div>
      ${job.error ? `<div class="notice error" style="white-space:pre-wrap">${esc(job.error)}</div><div style="height:12px"></div>` : ""}
      ${job.error && !startFailed ? `<button class="btn primary wide" id="retry">Back</button>` : ""}
      ${job.finished && (startFailed || !job.error) ? `<button class="btn primary wide" id="done">Open dashboard</button>` : ""}`;
    $("#retry") && ($("#retry").onclick = () => api("/api/job/dismiss", {}).then(() => { drawn.setup = null; poll(true); }));
    $("#done") && ($("#done").onclick = () => api("/api/job/dismiss", {}).then(() => poll(true)));
  });
}

// ---- wiring -------------------------------------------------------------------

document.querySelectorAll("#tabs button").forEach((b) => (b.onclick = () => {
  tab = b.dataset.tab;
  localStorageSet("tab", tab);
  drawn.logsActive = false;
  render();
}));
$("#log-name").onchange = () => { localStorageSet("log", $("#log-name").value); drawn.logsActive = false; renderLogs(); };
$("#log-follow").onchange = () => { drawn.logsActive = false; renderLogs(); };

poll();
