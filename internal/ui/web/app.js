// devstack dashboard. Polls /api/state and redraws only the parts that changed,
// so open dropdowns and focus survive the refresh.
"use strict";

const $ = (sel) => document.querySelector(sel);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

const ICONS = {
  play: '<path d="M5 3.5v9l7.5-4.5z" fill="currentColor"/>',
  stop: '<rect x="4" y="4" width="8" height="8" rx="1" fill="currentColor"/>',
  folder: '<path d="M2 4.5A1.5 1.5 0 0 1 3.5 3h3l1.5 1.5h4.5A1.5 1.5 0 0 1 14 6v5.5a1.5 1.5 0 0 1-1.5 1.5h-9A1.5 1.5 0 0 1 2 11.5z" fill="none" stroke="currentColor" stroke-width="1.2"/>',
  terminal: '<rect x="2" y="3" width="12" height="10" rx="1.5" fill="none" stroke="currentColor" stroke-width="1.2"/><path d="m4.8 6.3 2 1.7-2 1.7M8.5 10h2.7" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>',
  reload: '<path d="M13 8a5 5 0 1 1-1.5-3.6M13 2.5v3h-3" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>',
  heart: '<path d="M8 13.5S2.5 10.2 2.5 6.3A2.8 2.8 0 0 1 8 5a2.8 2.8 0 0 1 5.5 1.3c0 3.9-5.5 7.2-5.5 7.2z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>',
  open: '<path d="M9 3h4v4M13 3 7.5 8.5M11 9.5V12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h2.5" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>',
  check: '<path d="m3.5 8.5 3 3 6-7" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>',
  cross: '<path d="m4.5 4.5 7 7m0-7-7 7" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>',
  ring: '<circle cx="8" cy="8" r="5" fill="none" stroke="currentColor" stroke-width="1.4"/>',
  spin: '<path d="M8 3a5 5 0 1 0 5 5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><animateTransform attributeName="transform" type="rotate" from="0 8 8" to="360 8 8" dur=".9s" repeatCount="indefinite"/></path>',
  globe: '<g fill="none" stroke="currentColor" stroke-width="1.2"><circle cx="8" cy="8" r="5.8"/><path d="M2.2 8h11.6M8 2.2c1.9 1.9 1.9 9.7 0 11.6M8 2.2c-1.9 1.9-1.9 9.7 0 11.6"/></g>',
  copy: '<g fill="none" stroke="currentColor" stroke-width="1.2"><rect x="5.5" y="5.5" width="8" height="8" rx="1.3"/><path d="M3.5 10.5V3.8c0-.4.3-.8.8-.8h6.2"/></g>',
  eye: '<g fill="none" stroke="currentColor" stroke-width="1.2"><path d="M1.8 8S4 3.8 8 3.8 14.2 8 14.2 8 12 12.2 8 12.2 1.8 8 1.8 8z"/><circle cx="8" cy="8" r="2"/></g>',
  eyeOff: '<g fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"><path d="M6.3 4.1A6.6 6.6 0 0 1 8 3.8C12 3.8 14.2 8 14.2 8a10.6 10.6 0 0 1-1.7 2.3M10.1 11.6a5.9 5.9 0 0 1-2.1.6C4 12.2 1.8 8 1.8 8a10.9 10.9 0 0 1 2.4-2.9"/><path d="M6.6 6.6a2 2 0 0 0 2.8 2.8M2.5 2.5l11 11"/></g>',
  plus: '<path d="M8 3.5v9M3.5 8h9" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>',
  chevron: '<path d="m4.5 6.5 3.5 3.5 3.5-3.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>',
  sites: '<g fill="none" stroke="currentColor" stroke-width="1.2"><rect x="1.8" y="2.8" width="12.4" height="10.4" rx="1.5"/><path d="M1.8 6h12.4"/></g>',
  php: '<path d="M5.5 4.5 2 8l3.5 3.5M10.5 4.5 14 8l-3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>',
  database: '<g fill="none" stroke="currentColor" stroke-width="1.2"><ellipse cx="8" cy="4" rx="5.2" ry="1.9"/><path d="M2.8 4v8c0 1 2.3 1.9 5.2 1.9s5.2-.9 5.2-1.9V4M2.8 8c0 1 2.3 1.9 5.2 1.9s5.2-.9 5.2-1.9"/></g>',
  mail: '<g fill="none" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"><rect x="1.8" y="3.3" width="12.4" height="9.4" rx="1.5"/><path d="m2.4 4.3 5.6 4.4 5.6-4.4"/></g>',
  logs: '<path d="M3 4h10M3 8h10M3 12h6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>',
  settings: '<g fill="none" stroke="currentColor" stroke-width="1.2"><circle cx="8" cy="8" r="2.2"/><path d="M8 1.8v1.8M8 12.4v1.8M1.8 8h1.8M12.4 8h1.8M3.6 3.6l1.3 1.3M11.1 11.1l1.3 1.3M3.6 12.4l1.3-1.3M11.1 4.9l1.3-1.3"/></g>',
};
const icon = (name, cls = "") => `<svg class="${cls}" viewBox="0 0 16 16" aria-hidden="true">${ICONS[name]}</svg>`;

async function api(path, body, signal) {
  const opts = body === undefined ? {} : {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Devstack": "1" },
    body: JSON.stringify(body),
  };
  if (signal) opts.signal = signal;
  const r = await fetch(path, opts);
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw Object.assign(new Error(data.error || r.statusText), { status: r.status, data });
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

// act runs an action, shows its error, and refreshes state right away. The
// same action asked again while it runs (a double-click on Start or Install)
// gets the first one's result rather than a second request and its 409 toast.
const inflight = new Map();
function act(path, body, okMsg) {
  const key = path + " " + JSON.stringify(body ?? {});
  if (inflight.has(key)) return inflight.get(key);
  const p = runAct(path, body, okMsg).finally(() => inflight.delete(key));
  inflight.set(key, p);
  return p;
}

async function runAct(path, body, okMsg) {
  try {
    const res = await api(path, body ?? {});
    if (okMsg) toast(okMsg);
    poll(true);
    return res;
  } catch (e) {
    toast(e.message, true);
    redrawAll(); // a switch or select the user just changed goes back to what the server has
    poll(true);
    throw e;
  }
}

const open = (kind, target) => act("/api/open", { kind, target }).catch(() => {});
const copy = (text, what) => navigator.clipboard.writeText(text).then(() => toast(`${what} copied`), () => toast("Couldn't copy", true));
// Binary units, matching the CLI.
const size = (n) => n >= 2 ** 30 ? (n / 2 ** 30).toFixed(1) + " GB" : n >= 2 ** 20 ? (n / 2 ** 20).toFixed(1) + " MB" : n >= 2 ** 10 ? (n / 2 ** 10).toFixed(0) + " KB" : n + " B";
const plural = (n, word) => `${n} ${word}${n === 1 ? "" : "s"}`;

// ---- state + rendering -------------------------------------------------------

let S = null;
const PAGES = ["sites", "php", "database", "mail", "logs", "settings"];
// "servers" was the old combined tab; its first job, PHP, is the closest page.
let page = [location.hash.slice(1), localStorageGet("page"), { servers: "php" }[localStorageGet("tab")]].find((p) => PAGES.includes(p)) || "sites";
let selected = null; // the site shown in the side panel
let query = "";      // the Sites find box
const drawn = {};    // section -> key of what is on screen

function localStorageGet(k) { try { return localStorage.getItem(k); } catch { return null; } }
function localStorageSet(k, v) { try { localStorage.setItem(k, v); } catch {} }

// draw replaces a section only when its content key changes.
function draw(section, key, fn) {
  const k = JSON.stringify(key);
  if (drawn[section] === k) return;
  drawn[section] = k;
  // A redraw replaces the focused control; put focus back on its replacement.
  const had = document.activeElement;
  const sel = had && had !== document.body ? focusSelector(had) : null;
  fn();
  if (sel && !had.isConnected) document.querySelector(sel)?.focus({ preventScroll: true });
}

function focusSelector(el) {
  if (el.id) return "#" + CSS.escape(el.id);
  const d = [...el.attributes].find((a) => a.name.startsWith("data-"));
  return d ? `${el.tagName.toLowerCase()}[${d.name}="${CSS.escape(d.value)}"]` : null;
}

// redrawAll forgets what is on screen, so the next render repaints every section.
function redrawAll() {
  for (const k of Object.keys(drawn)) if (k !== "logsActive") delete drawn[k];
}

let pollTimer, pollSeq = 0, jobFailed = false;
async function poll() {
  clearTimeout(pollTimer);
  const seq = ++pollSeq; // an action polls at once; an older answer still on its way must not repaint over it
  let next;
  try {
    next = await api("/api/state", undefined, AbortSignal.timeout(10000));
  } catch (e) {
    if (seq !== pollSeq) return;
    const line = $("#state-line");
    if (line) line.textContent = "Lost contact with the devstack app: " + e.message;
    schedule();
    return;
  }
  if (seq !== pollSeq) return;
  S = next;
  if (S.job) S.job.steps ||= [];
  // A failed job (a declined Windows prompt) leaves the server unchanged: repaint
  // so the switch that started it shows that.
  const failed = !!(S.job && S.job.error);
  if (failed && !jobFailed) redrawAll();
  jobFailed = failed;
  try {
    render();
  } catch (e) {
    console.error(e);
    toast("Couldn't draw the page: " + e.message, true);
  }
  schedule();
}

// A hidden window stops asking; it catches up the moment it shows again.
function schedule() {
  clearTimeout(pollTimer);
  if (document.hidden) return;
  const busy = S && S.job && !S.job.finished;
  pollTimer = setTimeout(poll, busy ? 400 : 1500);
}
document.addEventListener("visibilitychange", () => { if (!document.hidden) poll(); });

function render() {
  const setupMode = S.needs_setup || (S.job && S.job.title === "Setting up devstack");
  $("#setup").hidden = !setupMode;
  $("#dash").hidden = setupMode;
  if (setupMode) { renderSetup(); patchProgress(); return; }
  renderShell();
  renderAlerts();
  if (page === "sites") { renderSites(); renderPanel(); }
  if (page === "php") renderPHP();
  if (page === "database") renderDB();
  if (page === "mail") renderMail();
  if (page === "logs") renderLogs();
  if (page === "settings") renderSettings();
  patchProgress();
  finishInstallJob();
}

function go(p) {
  page = p;
  localStorageSet("page", p);
  drawn.logsActive = false;
  closeMenu();
  render();
}

// ---- services ------------------------------------------------------------------

const label = (name) => name.replace(/^php-/, "PHP ").replace(/^mysql$/, "MySQL").replace(/^mailpit$/, "Mailpit");
const short = (v) => String(v ?? "").split(".").slice(0, 2).join(".");
// byRole orders services the way requests flow: web server, PHP, database, mail.
const role = (name) => name === "nginx" ? 0 : name.startsWith("php-") ? 1 : name === "mailpit" ? 3 : 2;
const byRole = (list) => [...list].sort((a, b) => role(a.name) - role(b.name) || a.name.localeCompare(b.name));
const services = () => byRole(S.running && S.groups.length
  ? S.groups
  : S.planned.map((p) => ({ name: p.name, version: p.version, ports: p.ports, state: "stopped", restarts: 0 })));
const failing = (g) => g && (g.state === "failed" || g.state === "backoff");
const moving = (g) => g && (g.state === "starting" || g.state === "stopping");
const portText = (p) => p ? String(p).split(/[\s,]+/).filter(Boolean).map((x) => ":" + x).join(" ") : "";
// mark is a slot's shape: "" filled (running), off, busy, bad (hatched).
const mark = (g) => !g || !S.running || g.state === "stopped" ? "off" : failing(g) ? "bad" : moving(g) ? "busy" : g.state === "running" ? "" : "off";
const stateWord = (g) => !S.running || !g || g.state === "stopped" ? "Stopped"
  : g.state === "running" ? "Running" : g.state === "backoff" ? "Restarting after a crash" : g.state === "failed" ? "Failed"
  : g.state === "starting" ? "Starting…" : g.state === "stopping" ? "Stopping…" : g.state;
const stateCls = (g) => ({ "": "ok", off: "off", busy: "busy", bad: "bad" })[mark(g)];
const phpGroup = (v) => S.groups.find((g) => g.name === "php-" + v);
// logFor names the log a service writes its errors to.
const logFor = (name) => name === "mysql" || name === "mailpit" ? name + ".log" : name + "-error.log";

function overall() {
  const job = S.job && !S.job.finished ? S.job : null;
  if (job) return { word: job.title.replace(/ servers$/, "") + "…", cls: "busy" };
  if (!S.running) return { word: "Stopped", cls: "" };
  if (S.phase === "starting") return { word: "Starting…", cls: "busy" };
  if (S.phase === "stopping") return { word: "Stopping…", cls: "busy" };
  if (S.groups.some((g) => g.state !== "running")) return { word: "Needs attention", cls: "bad" };
  return { word: "Running", cls: "" };
}

function renderShell() {
  const o = overall();
  $("#status").className = "status " + o.cls;
  $("#state-word").textContent = o.word;

  const sites = plural(S.sites.length, "site");
  const job = S.job && !S.job.finished ? S.job : null;
  let line;
  if (job) {
    const cur = [...job.steps].reverse().find((s) => s.state !== "pending");
    line = cur ? cur.label : "";
  } else if (S.job && S.job.error) {
    line = S.job.error.split("\n")[0];
  } else if (S.errors.length) {
    line = S.errors[0].split("\n")[0];
  } else if (S.running) {
    line = `Serving ${sites}` + (S.https?.on ? " · https" : "");
  } else {
    line = `${sites} ready to serve`;
  }
  $("#state-line").textContent = line;
  $("#state-line").title = line;

  const btn = $("#power");
  const busy = !!job || S.phase === "starting" || S.phase === "stopping";
  draw("power", [S.running, busy], () => {
    btn.className = S.running ? "btn small" : "btn small primary";
    btn.innerHTML = (S.running ? icon("stop") + '<span class="label">Stop</span>' : icon("play") + '<span class="label">Start</span>');
    btn.setAttribute("aria-label", S.running ? "Stop the servers" : "Start the servers");
    btn.disabled = busy;
    btn.onclick = () => act(S.running ? "/api/stop" : "/api/start").catch(() => {});
  });

  const svc = services();
  draw("slots", [svc, S.running], () => {
    $("#slots").innerHTML = svc.map((g) => {
      const m = mark(g);
      const right = m === "bad" ? (g.state === "backoff" ? "restarting" : "failed") : m === "busy" ? g.state : S.running && m === "" ? portText(g.ports) : short(g.version);
      const name = g.name === "nginx" ? "nginx" : label(g.name) + (g.name.startsWith("php-") ? "" : " " + short(g.version));
      const to = g.name.startsWith("php-") ? "php" : g.name === "mysql" ? "database" : g.name === "mailpit" ? "mail" : "settings";
      return `<button class="slot ${m === "" ? "on" : m}" data-goto="${to}" title="${esc(label(g.name))} ${esc(g.version)}: ${esc(stateWord(g))}">
        <i class="mark"></i><span class="name">${esc(name)}</span><em>${esc(right)}</em></button>`;
    }).join("");
    $("#slots").querySelectorAll("[data-goto]").forEach((b) => (b.onclick = () => go(b.dataset.goto)));
  });

  const phpBad = S.running && S.groups.some((g) => g.name.startsWith("php-") && failing(g));
  const dbBad = S.running && S.groups.some((g) => g.name === "mysql" && failing(g));
  const mailBad = S.running && S.groups.some((g) => g.name === "mailpit" && failing(g));
  const unread = S.mail && S.mail.counted ? S.mail.unread : 0;
  const update = !!(S.update && S.update.version);
  draw("nav", [page, S.sites.length, phpBad, dbBad, mailBad, unread, update], () => {
    const extra = { sites: `<span class="badge">${S.sites.length}</span>`, php: phpBad ? '<i class="dot" title="A PHP version isn\'t running"></i>' : "",
      database: dbBad ? '<i class="dot" title="MySQL isn\'t running"></i>' : "", settings: update ? '<i class="dot update" title="Update available"></i>' : "",
      mail: mailBad ? '<i class="dot" title="Mailpit isn\'t running"></i>' : unread ? `<span class="badge" title="${plural(unread, "unread message")}">${unread > 99 ? "99+" : unread}</span>` : "" };
    const names = { sites: "Sites", php: "PHP", database: "Database", mail: "Mail", logs: "Logs", settings: "Settings" };
    document.querySelectorAll(".nav [data-page]").forEach((b) => {
      const p = b.dataset.page;
      b.innerHTML = icon(p) + `<span class="label">${names[p]}</span>` + (extra[p] || "");
      b.setAttribute("aria-label", names[p] + (p === "settings" && update ? ", update available" : "") + (p === "mail" && unread ? `, ${plural(unread, "unread message")}` : ""));
      b.toggleAttribute("aria-current", p === page);
      if (p === page) b.setAttribute("aria-current", "page");
    });
    document.querySelectorAll(".page").forEach((v) => v.classList.toggle("active", v.dataset.page === page));
  });
}

// renderAlerts shows what is broken right now, above every page.
function renderAlerts() {
  const bad = S.running ? S.groups.filter(failing) : [];
  const job = S.job && S.job.finished && S.job.error && !/^(Installing|Updating)/.test(S.job.title) ? S.job : null;
  draw("alerts", [bad, S.sites.map((s) => [s.slug, s.php]), S.errors, job?.title, job?.error], () => {
    const rows = bad.map((g) => {
      const v = g.name.startsWith("php-") ? g.name.slice(4) : null;
      const hit = v ? S.sites.filter((s) => s.php === v).map((s) => s.slug) : [];
      const who = hit.length ? ` ${hit.slice(0, 4).join(", ")}${hit.length > 4 ? ` and ${hit.length - 4} more` : ""} ${hit.length === 1 ? "isn't" : "aren't"} answering.` : "";
      const why = g.state === "backoff" ? ` keeps stopping${g.restarts ? ` (${plural(g.restarts, "restart")})` : ""}; devstack is trying again.` : ` stopped${g.restarts ? ` after ${plural(g.restarts, "restart")}` : ""}.`;
      return `<div class="alert"><i class="mark bad"></i><div class="grow"><b>${esc(label(g.name))}</b>${esc(why)}${esc(who)}
          ${g.last_error ? `<div class="muted mono" style="margin-top:2px">${esc(g.last_error.split("\n")[0])}</div>` : ""}</div>
        <button class="btn small" data-log="${esc(logFor(g.name))}">Log</button>
        <button class="btn small primary" data-restart="${esc(g.name)}">Restart</button></div>`;
    });
    // A stopped server already has its own row above.
    for (const e of S.errors.filter((e) => !bad.some((g) => e.startsWith(g.name + ": ")))) rows.push(`<div class="alert"><i class="mark bad"></i><div class="grow">${esc(e.split("\n")[0])}</div>
      <button class="btn small" data-log="devstack.log">Log</button></div>`);
    if (job) rows.push(`<div class="alert"><i class="mark bad"></i><div class="grow"><b>${esc(job.title)} failed.</b> ${esc(job.error.split("\n")[0])}</div>
      <button class="btn small ghost" id="job-dismiss">Dismiss</button></div>`);
    $("#alerts").innerHTML = rows.join("");
    $("#alerts").querySelectorAll("[data-restart]").forEach((b) => (b.onclick = () => act("/api/restart", { unit: b.dataset.restart }, "Restarted " + label(b.dataset.restart)).catch(() => {})));
    $("#alerts").querySelectorAll("[data-log]").forEach((b) => (b.onclick = () => showLog(b.dataset.log)));
    $("#job-dismiss") && ($("#job-dismiss").onclick = () => api("/api/job/dismiss", {}).then(() => poll(true)));
  });
}

// ---- sites ---------------------------------------------------------------------

// adding is a folder picked with "Add folder…" whose own name can't be used
// (another site has it), waiting for the user to pick another.
let adding = null;
async function pickProject() {
  closeMenu();
  try {
    const res = await api("/api/project/pick", {});
    if (res.dir) await addProject(res.dir, "");
  } catch (e) { toast(e.message, true); }
}

// addProject serves dir under name ("" = the folder's own name), and asks for
// a name only when the server says that one is taken. A second click while it
// runs is ignored.
let addingNow = false;
async function addProject(dir, name) {
  if (addingNow) return;
  addingNow = true;
  try {
    const res = await api("/api/project/add", { dir, name });
    adding = null;
    selected = res.name || selected;
    renderSites();
    if (res.warnings?.length) toast(`Added ${res.name}. ${res.warnings.join(" ")}`, true);
    else toast(`Added ${res.name}`);
  } catch (e) {
    if (e.status !== 409) { toast(e.message, true); return; }
    adding = { dir, name: e.data.suggestion || "", problem: e.message };
    renderSites();
    $("#add-name")?.select();
  } finally {
    addingNow = false;
    poll(true);
  }
}

function removeProject(name) {
  if (!confirm(`Stop serving ${name}? The folder and your files stay where they are.`)) return;
  if (selected === name) selected = null;
  act("/api/project/remove", { name }, `${name} is no longer served`).catch(() => {});
}

// siteMark is a site's slot shape: its PHP version's state while serving.
function siteMark(s) {
  if (!S.running) return ["off", "Not served while the servers are stopped"];
  const g = phpGroup(s.php);
  if (!g) return ["off", `PHP ${s.php} isn't running`];
  if (failing(g)) return ["bad", `PHP ${s.php} isn't running, so this site isn't answering`];
  if (moving(g)) return ["busy", stateWord(g)];
  return ["", "Serving"];
}

// The PHP choices for a site: each installed version once, the one the site
// runs selected. The default version is marked ★ (the PHP headings explain
// the star) and choosing it
// unpins the site, so it follows the default when that changes.
const phpOptions = (site) => {
  const vs = [...(S.phps || [])];
  if (S.default_php && !vs.includes(S.default_php)) vs.push(S.default_php);
  if (site.php && !vs.includes(site.php)) vs.push(site.php);
  vs.sort((a, b) => b.localeCompare(a, undefined, { numeric: true }));
  return vs.map((v) => {
    const def = v === S.default_php;
    return `<option value="${def ? "" : esc(v)}" ${site.php === v ? "selected" : ""}>${esc(v)}${def ? " ★" : ""}</option>`;
  }).join("");
};
const host = (url) => url.replace(/^https?:\/\//, "").replace(/\/$/, "");

function setPHP(s, php) {
  return act("/api/site/php", { slug: s.slug, php }, `${s.slug} now uses ${php ? "PHP " + php : "the default PHP"}`).catch(() => {});
}
function setGuide(s, on) {
  return act("/api/site/guide", { slug: s.slug, on }, on ? `Added the AI guide to ${s.slug}` : `Removed the AI guide from ${s.slug}`);
}

function renderSites() {
  $("#site-count").textContent = S.sites.length || "";
  $("#reload").disabled = !S.running;
  const marks = S.sites.map(siteMark);
  draw("sites", [S.sites, S.unserved, S.phps, S.default_php, S.www, S.running, S.dns, S.shims, adding?.dir, adding?.problem, query, selected, marks], () => {
    const el = $("#sites");
    const d = S.dns, sh = S.shims;
    const todo = [];
    // php and composer in terminals pick each site's PHP only while devstack's shims come first on PATH.
    if (sh && !sh.ok && S.sites.length) todo.push(`<div class="todo-item"><i class="mark off" style="margin-top:4px"></i><div class="grow"><b>Terminals don't use each site's PHP yet</b>
      ${esc(sh.problem)} ${sh.needs_admin ? "Fixing it puts an administrators-only copy of devstack's php first on the system PATH; Windows asks for administrator approval once." : "Fixing it puts devstack's php first on your PATH."}
      Terminals opened afterwards pick it up.</div><div class="acts"><button class="btn small primary" id="shims-fix">Fix</button></div></div>`);
    if (d && !d.on && !d.asked && S.sites.length) todo.push(`<div class="todo-item"><i class="mark off" style="margin-top:4px"></i><div class="grow"><b>Use ${esc(S.sites[0].slug)}.${esc(d.tld)} addresses?</b>
      Sites answer at <span class="mono">name.localhost</span> now, which only browsers and curl understand. With <span class="mono">.${esc(d.tld)}</span> addresses
      PHP, Postman and terminals reach them too. Windows asks for administrator approval once; only <span class="mono">*.${esc(d.tld)}</span> lookups go to devstack.</div>
      <div class="acts"><button class="btn small primary" id="dns-yes">Turn on</button><button class="btn small ghost" id="dns-no">Not now</button></div></div>`);
    const todoCard = todo.length ? `<div class="todo"><div class="todo-head"><b>Finish setting up</b><span class="muted">${plural(todo.length, "step")} left, each optional</span></div>${todo.join("")}</div>` : "";
    const addAsk = adding ? `<div class="todo"><div class="todo-item"><i class="mark gone" style="margin-top:4px"></i><div class="grow"><b>Pick another name for this folder</b>
        ${esc(adding.problem)}<div class="mono" title="${esc(adding.dir)}">${esc(adding.dir)}</div>
        <label class="add-name"><input type="text" id="add-name" value="${esc(adding.name)}" spellcheck="false" aria-label="Name for the site"><span class="mono">.${esc(S.dns?.on ? S.dns.tld : "localhost")}</span></label></div>
        <div class="acts"><button class="btn small primary" id="add-yes">Add</button><button class="btn small ghost" id="add-no">Cancel</button></div></div></div>` : "";

    const gone = (S.unserved || []).map((p) => `<div class="site gone"><i class="mark gone" role="img" aria-label="Not served" title="Not served"></i>
        <div class="site-id"><span class="site-name">${esc(p.name)}</span><span class="site-url">${p.missing
          ? `Folder missing: <span class="mono">${esc(p.dir)}</span>. It comes back by itself when the folder does (a drive plugged in, a folder moved back).`
          : `Not served: <span class="mono">${esc(p.dir)}</span> is already served under another name.`}</span></div>
        <button class="btn small ghost" data-unlist="${esc(p.name)}">Remove</button></div>`).join("");

    let body;
    if (!S.sites.length) {
      body = `<div class="empty"><p>No sites yet. Every folder in <span class="mono">${esc(S.www)}</span> becomes a site,
        and you can add folders from anywhere else.</p>
        <button class="btn" id="empty-open">${icon("folder")}Open www folder</button> <button class="btn" id="empty-add">${icon("plus")}Add folder…</button></div>`;
    } else {
      const q = query.trim().toLowerCase();
      const shown = S.sites.map((s, i) => [s, i]).filter(([s]) => !q || [s.slug, s.name, s.url, s.dir].join(" ").toLowerCase().includes(q));
      body = `<div class="site-head" aria-hidden="true"><span></span><span>Site</span><span>PHP <small class="star-key">★ default</small></span><span>AI guide</span><span></span></div>` + shown.map(([s, i]) => {
        const [m, why] = marks[i];
        return `<div class="site ${selected === s.slug ? "selected" : ""}" data-row="${i}">
          <i class="mark ${m}" role="img" aria-label="${esc(why)}" title="${esc(why)}"></i>
          <div class="site-id"><button class="site-name" data-select="${i}" aria-expanded="${selected === s.slug}" title="Show ${esc(s.slug)}'s details">${esc(s.slug)}</button>
            <span class="site-url">${esc(host(s.url))}${s.project ? ` · <span class="from" title="Added from outside www">${esc(s.dir)}</span>` : s.name !== s.slug ? ` · <span class="from">${esc(s.name)}</span>` : ""}</span></div>
          <select data-php="${i}" aria-label="PHP version for ${esc(s.slug)}; ★ marks the default">${phpOptions(s)}</select>
          <label class="switch" title="AI guide for ${esc(s.slug)}: ${s.guide ? "on" : "off"}"><input type="checkbox" data-guide="${i}" ${s.guide ? "checked" : ""} aria-label="AI guide for ${esc(s.slug)}"><span></span></label>
          <div class="tools">
            <button class="btn small ghost icon-only" data-url="${i}" title="${S.running ? `Open ${esc(s.url)}` : "Start the servers to open it"}" aria-label="Open ${esc(s.slug)} in the browser">${icon("globe")}</button>
            <button class="btn small ghost icon-only" data-folder="${i}" title="Open ${esc(s.dir)}" aria-label="Open ${esc(s.slug)}'s folder">${icon("folder")}</button>
            <button class="btn small ghost icon-only" data-terminal="${i}" title="Terminal in ${esc(s.slug)}: php and composer run PHP ${esc(s.php)}" aria-label="Open a terminal in ${esc(s.slug)}">${icon("terminal")}</button>
          </div></div>`;
      }).join("") + (shown.length ? "" : `<div class="no-match">No site matches “${esc(query.trim())}”.</div>`);
    }
    el.innerHTML = addAsk + todoCard + `<div class="site-list">${body}${gone}</div>` +
      (S.sites.length ? `<p class="list-note">New folders in <span class="mono">${esc(S.www)}</span> are served within a few seconds while the servers run.</p>` : "");

    if (adding) {
      $("#add-name").oninput = (e) => (adding.name = e.target.value);
      const add = () => addProject(adding.dir, adding.name.trim());
      $("#add-name").onkeydown = (e) => { if (e.key === "Enter") add(); if (e.key === "Escape") $("#add-no").click(); };
      $("#add-yes").onclick = add;
      $("#add-no").onclick = () => { adding = null; renderSites(); };
    }
    $("#empty-open") && ($("#empty-open").onclick = () => open("folder", S.www));
    $("#empty-add") && ($("#empty-add").onclick = pickProject);
    $("#shims-fix") && ($("#shims-fix").onclick = () => act("/api/shims/fix", {}).catch(() => {}));
    $("#dns-yes") && ($("#dns-yes").onclick = () => act("/api/dns", { on: true }).catch(() => {}));
    $("#dns-no") && ($("#dns-no").onclick = () => act("/api/dns/dismiss", {}, "You can turn it on later in Settings").catch(() => {}));
    el.querySelectorAll("[data-unlist]").forEach((b) => (b.onclick = () => removeProject(b.dataset.unlist)));
    el.querySelectorAll("[data-url]").forEach((b) => (b.onclick = () => open("url", S.sites[b.dataset.url].url)));
    el.querySelectorAll("[data-folder]").forEach((b) => (b.onclick = () => open("folder", S.sites[b.dataset.folder].dir)));
    el.querySelectorAll("[data-terminal]").forEach((b) => (b.onclick = () => act("/api/terminal", { site: S.sites[b.dataset.terminal].slug }).catch(() => {})));
    el.querySelectorAll("[data-select]").forEach((b) => (b.onclick = () => select(S.sites[b.dataset.select].slug)));
    // A click anywhere else on the row (not on one of its controls) selects it too.
    el.querySelectorAll("[data-row]").forEach((r) => (r.onclick = (e) => {
      if (!e.target.closest("button, select, label, input, a")) select(S.sites[r.dataset.row].slug);
    }));
    el.querySelectorAll("[data-guide]").forEach((cb) => (cb.onchange = () => setGuide(S.sites[cb.dataset.guide], cb.checked).catch(() => (cb.checked = !cb.checked))));
    el.querySelectorAll("[data-php]").forEach((sel) => (sel.onchange = () => setPHP(S.sites[sel.dataset.php], sel.value)));
  });
}

function select(slug) {
  selected = selected === slug ? null : slug;
  renderSites();
  renderPanel();
  if (selected) loadServerData(true);
}

// ---- site panel ------------------------------------------------------------------

// Sample lines stand in for per-site errors in development builds only; the
// feed that reads them from each site's logs comes in a later version.
const SAMPLE_ERRORS = [
  ["PHP Fatal error: Allowed memory size of 134217728 bytes exhausted", "app/Http/Kernel.php:88"],
  ["PHP Warning: Undefined array key \"cart\"", "resources/views/cart.blade.php:42"],
];

function renderPanel() {
  const panel = $("#panel");
  const s = selected && S.sites.find((x) => x.slug === selected);
  $(".sites-page").classList.toggle("with-panel", !!s);
  if (!s) { selected = null; panel.hidden = true; drawn.panel = null; return; }
  panel.hidden = false;
  const [m, why] = siteMark(s);
  const dev = !(S.update && S.update.release);
  const mailKey = S.mail && [S.mail.enabled, S.mail.installed, S.mail.inbox, S.mail.smtp_port];
  draw("panel", [s, S.phps, S.default_php, S.running, m, db, dev, jobBusy(), packages, progressKey(installJob()), pwShown, mailKey], () => {
    const dbHTML = panelDB(s);
    const errs = dev
      ? `<div class="errors">${SAMPLE_ERRORS.map(([e, at]) => `<span class="e">${esc(e)}</span><br>in ${esc(at)}`).join("<br>")}</div>`
      : `<div class="errors soon">Per-site errors are coming in a later version. Until then, PHP ${esc(s.php)}'s error log has them.</div>`;
    $("#panel").innerHTML = `
      <div class="panel-head"><div class="grow"><h2>${esc(s.slug)}</h2><div class="panel-path" title="${esc(s.dir)}">${esc(s.dir)}</div></div>
        <button class="btn small ghost icon-only" id="p-close" aria-label="Close the details" title="Close (Esc)">${icon("cross")}</button></div>
      <div class="panel-body">
        <div>
          <div class="addr"><div class="field"><span title="${esc(s.url)}">${esc(s.url)}</span>
            <button class="btn small ghost icon-only" id="p-copy" aria-label="Copy the address" title="Copy the address">${icon("copy")}</button></div>
            <button class="btn primary" id="p-open" ${S.running ? "" : `disabled title="Start the servers to open it"`}>Open</button></div>
          <div class="hint"><i class="mark ${m}" style="vertical-align:-1px;margin-right:6px"></i>${esc(why)}</div>
        </div>
        <div class="row-btns">
          <button class="btn small" id="p-folder">${icon("folder")}Folder</button>
          <button class="btn small" id="p-term">${icon("terminal")}Terminal</button>
          ${s.project ? `<button class="btn small ghost danger" id="p-remove">Stop serving</button>` : ""}
        </div>
        <div class="php-line"><span class="group-label">PHP version <small class="star-key">★ default</small></span><select id="p-php" class="php-pick" aria-label="PHP version for ${esc(s.slug)}; ★ marks the default" title="php and composer in this site's terminal use it too">${phpOptions(s)}</select></div>
        <div><div class="group-label">AI guide</div>
          <label class="toggle-row" title="Writes .devstack/ai-guide.md (kept out of git) and a short pointer in AGENTS.md and CLAUDE.md. Your own text there is left alone."><span class="switch"><input type="checkbox" id="p-guide" ${s.guide ? "checked" : ""}><span></span></span>
            ${s.guide ? "On: coding agents know this site's URL, PHP, database and logs." : "Off: tell coding agents how this site runs here."}</label>
          </div>
        <div><div class="group-label">Database <span class="muted" style="font-weight:400">shared by every site</span></div>${dbHTML}</div>
        <div><div class="group-label">Mail</div>${panelMail(s)}</div>
        <div><div class="group-label">Recent errors ${dev ? '<span class="tag">sample</span>' : ""}</div>${errs}
          <div class="row-btns" style="margin-top:8px">
            <button class="link" id="p-errlog">PHP ${esc(s.php)} error log</button><span class="muted">·</span>
            <button class="link" id="p-access">Access log</button></div></div>
      </div>`;
    $("#p-close").onclick = () => select(s.slug);
    $("#p-copy").onclick = () => copy(s.url, "Address");
    $("#p-open").onclick = () => open("url", s.url);
    $("#p-folder").onclick = () => open("folder", s.dir);
    $("#p-term").onclick = () => act("/api/terminal", { site: s.slug }).catch(() => {});
    $("#p-remove") && ($("#p-remove").onclick = () => removeProject(s.slug));
    $("#p-guide").onchange = (e) => setGuide(s, e.target.checked).catch(() => (e.target.checked = !e.target.checked));
    $("#p-pw-show") && ($("#p-pw-show").onclick = togglePassword);
    $("#p-pw-copy") && ($("#p-pw-copy").onclick = copyPassword);
    $("#p-pma") && ($("#p-pma").onclick = () => open("url", pmaURL(s)));
    $("#p-dbuser") && ($("#p-dbuser").onclick = (e) => createSiteUser(s, !!e.currentTarget.dataset.new));
    $("#p-pma-install") && ($("#p-pma-install").onclick = (e) => {
      const [kind, version] = e.currentTarget.dataset.pkg.split("|");
      act("/api/packages/install", { kind, version }).catch(() => {});
    });
    $("#p-mysql-on") && ($("#p-mysql-on").onclick = () => setMySQL(""));
    $("#p-mail") && ($("#p-mail").onclick = () => siteToMailpit(s));
    $("#p-inbox") && ($("#p-inbox").onclick = () => open("url", S.mail.inbox));
    $("#p-mail-on") && ($("#p-mail-on").onclick = () => setMail(true));
    $("#p-errlog").onclick = () => showLog(logFor("php-" + s.php));
    $("#p-access").onclick = () => showLog(`sites/${s.slug}.access.log`);
    $("#p-php").onchange = (e) => setPHP(s, e.target.value);
  });
}

// CONN_NAMES spell Laravel's DB_CONNECTION values for people.
const CONN_NAMES = { sqlite: "SQLite", pgsql: "PostgreSQL", sqlsrv: "SQL Server", mysql: "MySQL", mariadb: "MariaDB" };

// envDB says which database the site's .env (Laravel) uses, when it sets one.
function envDB(e) {
  if (!e) return "";
  let text;
  if (e.ours) {
    text = e.name
      ? `This site's .env uses the database <b class="mono">${esc(e.name)}</b>.`
      : "This site's .env uses this MySQL but doesn't name a database yet (DB_DATABASE).";
  } else if (e.conn === "mysql" || e.conn === "mariadb") {
    const at = `${e.host || "127.0.0.1"}:${e.port || "3306"}`;
    text = `This site's .env points at <span class="mono">${esc(at)}</span>${db && db.enabled ? `, not devstack's MySQL on <span class="mono">127.0.0.1:${esc(db.port)}</span>` : ""}.`;
  } else {
    text = `This site's .env uses ${esc(CONN_NAMES[e.conn] || e.conn)}${e.name ? ` (<span class="mono">${esc(e.name)}</span>)` : ""}, not MySQL.`;
  }
  return `<div class="env-db">${text}</div>`;
}

// pmaURL opens phpMyAdmin at the site's own database when its .env names one.
function pmaURL(s) {
  const base = db.phpmyadmin.url;
  const e = s.db;
  return e && e.ours && e.name ? `${base}/index.php?route=/database/structure&db=${encodeURIComponent(e.name)}` : base;
}

// siteUser offers a MySQL user of the site's own, limited to its database,
// in place of root. Shown when the .env names a database on devstack's MySQL.
function siteUser(s, busy) {
  const e = s.db;
  if (!e || !e.ours || !e.name) return "";
  const own = s.slug.replace(/-/g, "_").slice(0, 32);
  if (e.username === own) {
    return `<div class="site-user"><span>Signs in as <b class="mono">${esc(own)}</b>, which can only reach <b class="mono">${esc(e.name)}</b>.</span>
      <button class="btn small ghost" id="p-dbuser" data-new="1" ${busy ? "disabled" : ""} title="Give ${esc(own)} a new random password and put it in .env">New password</button></div>`;
  }
  const who = e.username ? `Signs in as <b class="mono">${esc(e.username)}</b>${e.username === "root" ? ", which can reach every database" : ""}.` : "The .env sets no DB_USERNAME.";
  return `<div class="site-user"><span>${who}</span>
    <button class="btn small" id="p-dbuser" ${busy ? "disabled" : ""} title="Create MySQL user ${esc(own)} with access to only ${esc(e.name)}, and put its login in .env">Create a user for this site</button></div>`;
}

function createSiteUser(s, renew) {
  const own = s.slug.replace(/-/g, "_").slice(0, 32);
  const q = renew
    ? `Give ${own} a new password? devstack writes it to ${s.slug}'s .env (DB_PASSWORD).`
    : `Create MySQL user ${own} with access to only the ${s.db.name} database (created if it's missing)?

devstack writes its login to ${s.slug}'s .env as DB_USERNAME and DB_PASSWORD.`;
  if (!confirm(q)) return;
  act("/api/site/dbuser", { slug: s.slug }).then((r) => {
    toast(r.config_cached
      ? `${r.user} is ready. Laravel has cached its config: run php artisan config:clear`
      : `${r.user} is ready, and ${s.slug}'s .env uses it`);
  }).catch(() => {});
}

// panelDB is the Database section of a site's details: the database its .env
// uses, the login of the shared MySQL, and phpMyAdmin.
function panelDB(s) {
  if (!db) return '<div class="muted">Loading…</div>';
  const busy = jobBusy();
  let out = envDB(s.db);
  if (!db.enabled) {
    return out + (db.installed
      ? `<div class="muted">MySQL is off. phpMyAdmin and your sites' databases need it.</div><div class="row-btns" style="margin-top:6px"><button class="btn small" id="p-mysql-on" ${busy ? "disabled" : ""}>Turn on MySQL ${esc(db.installed)}</button></div>`
      : '<div class="muted">MySQL is off. Install a version on the Database page.</div>');
  }
  if (!db.ready) return out + '<div class="muted">A root password is generated the first time MySQL starts.</div>';
  out += siteUser(s, busy);
  out += `<div class="kv-cap">Root login · every database</div><dl class="kv"><dt>Host</dt><dd>${esc(db.host)}:${esc(db.port)}</dd><span></span><dt>User</dt><dd>${esc(db.user)}</dd><span></span>
    <dt>Password</dt>${pwCells("p-pw")}</dl>`;
  let pma;
  const job = installJob();
  const pkg = packages && packages.find((p) => p.kind === "phpmyadmin");
  if (db.phpmyadmin) {
    const e = s.db;
    const label = e && e.ours && e.name ? `Open ${esc(e.name)} in phpMyAdmin` : "phpMyAdmin";
    pma = `<button class="btn small" id="p-pma" ${S.running ? "" : `disabled title="Start the servers to open it"`}>${icon("open")}${label}</button>`;
  } else if (job && !job.finished && pkg && job.title === `Installing ${pkg.kind} ${pkg.version}`) {
    const step = job.steps[0];
    pma = `<span class="muted">Installing phpMyAdmin… ${pctText(step, "pct", `${stepPct(step)}%`)}</span>`;
  } else if (pkg && !pkg.installed) {
    pma = `<button class="btn small" id="p-pma-install" data-pkg="${esc(pkg.kind)}|${esc(pkg.version)}" ${busy ? "disabled" : ""} title="Download phpMyAdmin ${esc(pkg.version)} (${size(pkg.size)}) to manage databases in the browser">${icon("open")}Install phpMyAdmin</button>`;
  }
  return out + (pma ? `<div class="row-btns" style="margin-top:8px">${pma}</div>` : "");
}

// ---- PHP, Database ---------------------------------------------------------------

let packages = null, db = null, packagesFor = "";
async function loadServerData(onlyIfMissing) {
  if (onlyIfMissing && db && packages) return;
  try {
    [packages, db] = await Promise.all([api("/api/packages"), api("/api/db")]);
    drawn.packages = drawn.php = drawn.db = drawn.settings = drawn.panel = null;
    render();
  } catch (e) { toast(e.message, true); }
}
// ensureServerData refreshes package and DB info when a page needs it or a job finishes.
function ensureServerData() {
  const marker = [S.running, S.job ? S.job.title + S.job.finished : ""].join("|");
  if (packagesFor !== marker) { packagesFor = marker; loadServerData(); }
}

const installJob = () => S.job && S.job.title.startsWith("Installing") ? S.job : null;
const jobBusy = () => !!(S.job && !S.job.finished);
// progressKey is a job as far as a page's layout goes: its steps and their
// states, not the byte counts. Downloads report every 400 ms; patchProgress
// moves the bars and numbers in place instead of redrawing the whole section.
const progressKey = (job) => job && [job.title, job.finished, job.error, job.steps.map((s) => [s.label, s.state, s.total > 0, s.done > 0])];
const stepPct = (s) => s && s.total ? Math.min(100, Math.round((s.done / s.total) * 100)) : 0;
const barFill = (s) => `<div class="bar-fill"${s ? ` data-bar="${esc(s.label)}"` : ""} style="transform:scaleX(${stepPct(s) / 100})"></div>`;
// pctText marks text that patchProgress keeps current; fmt says how it reads.
const pctText = (s, fmt, text, attrs = "") => s ? `<span data-pct="${esc(s.label)}" data-fmt="${fmt}" ${attrs}>${text}</span>` : text;

function patchProgress() {
  const job = S.job;
  if (!job || job.finished) return;
  const step = (label) => job.steps.find((s) => s.label === label);
  for (const el of document.querySelectorAll("[data-bar]")) {
    const s = step(el.dataset.bar);
    if (s) el.style.transform = `scaleX(${stepPct(s) / 100})`;
  }
  for (const el of document.querySelectorAll("[data-pct]")) {
    const s = step(el.dataset.pct);
    if (!s) continue;
    const pct = stepPct(s);
    const text = {
      "pct-of": () => `${pct}% of ${size(+el.dataset.size)}`,
      pct: () => `${pct}%`,
      slash: () => `${size(s.done || 0)} / ${size(s.total)}`,
      update: () => s.total > 0 ? `${size(s.done || 0)} of ${size(s.total)} · ${pct}%` : s.done > 0 ? `${size(s.done)} downloaded` : el.textContent,
    }[el.dataset.fmt];
    if (text) el.textContent = text();
  }
}

// versionRows lists the catalog's builds of one kind: in use, installed, or to install.
function versionRows(kind) {
  if (!packages) return '<div class="rows"><div class="rowi muted">Loading…</div></div>';
  const job = installJob(), busy = jobBusy();
  const rows = packages.filter((p) => p.kind === kind).map((p) => {
    const installing = job && job.title === `Installing ${p.kind} ${p.version}` && !job.finished;
    const step = installing && job.steps[0];
    let action;
    if (installing) {
      action = `<div class="progress"><div class="bar-track">${barFill(step)}</div><div class="muted num">${pctText(step, "pct-of", `${stepPct(step)}% of ${size(p.size)}`, `data-size="${p.size}"`)}</div></div>`;
    } else if (p.in_use) {
      action = '<span class="in-use"><i class="mark"></i>In use</span>';
    } else if (p.installed) {
      const use = p.kind === "mysql"
        ? `<button class="btn small" data-use-mysql="${esc(p.version.split(".").slice(0, 2).join("."))}" ${busy ? "disabled" : ""} title="Run MySQL ${esc(p.version)}. A version used for the first time starts with empty databases.">Use</button>`
        : "";
      action = `${use}<button class="btn small ghost danger" data-remove="${esc(p.kind)}|${esc(p.version)}" ${busy ? "disabled" : ""}>Remove</button>`;
    } else {
      action = `<button class="btn small" data-install="${esc(p.kind)}|${esc(p.version)}" ${busy ? "disabled" : ""}>Install</button>`;
    }
    return `<div class="rowi"><div class="grow"><b class="num">${esc(p.version)}</b> ${p.label ? `<span class="note">${esc(p.label)}</span>` : ""}
        ${p.installed && !p.in_use ? '<span class="muted"> · installed</span>' : ""}</div>
      <span class="muted num">${size(p.size)}</span>${action}</div>`;
  });
  return `<div class="rows">${rows.join("") || '<div class="rowi muted">Nothing in the catalog.</div>'}</div>`;
}

// setMySQL turns MySQL on ("" = the newest installed version), off ("off"), or
// moves it to another installed major.minor.
function setMySQL(version) {
  const msg = version === "off" ? "Turned off MySQL" : version ? `MySQL ${version} is in use` : "Turned on MySQL";
  return act("/api/mysql", { version }, msg).then(() => loadServerData()).catch(() => {});
}

function wireVersions(el) {
  el.querySelectorAll("[data-use-mysql]").forEach((b) => (b.onclick = () => {
    const v = b.dataset.useMysql;
    if (db && db.enabled && !confirm(`Switch MySQL to ${v}? Each MySQL version keeps its own databases, so ${v} won't have the databases of ${db.version}. They stay on disk for when you switch back.`)) return;
    setMySQL(v);
  }));
  el.querySelectorAll("[data-install]").forEach((b) => (b.onclick = () => {
    const [kind, version] = b.dataset.install.split("|");
    act("/api/packages/install", { kind, version }).catch(() => {});
  }));
  el.querySelectorAll("[data-remove]").forEach((b) => (b.onclick = () => {
    const [kind, version] = b.dataset.remove.split("|");
    if (!confirm(`Remove ${kind} ${version}? You can download it again later.`)) return;
    act("/api/packages/remove", { kind, version }, `Removed ${kind} ${version}`).then(() => loadServerData()).catch(() => {});
  }));
}

// finishInstallJob reports a finished install and clears it.
function finishInstallJob() {
  const job = installJob();
  if (!job || !job.finished) return;
  S.job = null; // report it once: the reload below renders again before the next poll
  if (job.error) toast(job.error, true);
  api("/api/job/dismiss", {}).then(() => loadServerData()).catch(() => {});
}

// serverCard is one server's slot on the PHP or Database page.
function serverCard(g, body, buttons) {
  const m = mark(g);
  return `<div class="server ${m === "bad" ? "bad" : m === "off" ? "off" : ""}">
    <div class="server-top"><i class="mark ${m}"></i><h2>${esc(label(g.name))}</h2><span class="muted num">${esc(g.version)}</span>
      <div class="grow"></div><span class="state ${stateCls(g)}">${esc(stateWord(g))}${g.restarts ? ` · ${plural(g.restarts, "restart")}` : ""}</span></div>
    ${body}
    ${g.last_error && failing(g) ? `<div class="err mono">${esc(g.last_error.split("\n")[0])}</div>` : ""}
    <div class="row-btns">${buttons}</div></div>`;
}

function renderPHP() {
  ensureServerData();
  const groups = services().filter((g) => g.name.startsWith("php-"));
  $("#php-default").textContent = S.default_php ? `New sites use PHP ${S.default_php}` : "";
  const job = installJob();
  draw("php", [groups, S.running, S.sites.map((s) => [s.slug, s.php]), S.default_php, S.warnings, packages, progressKey(job), S.extensions, S.odbc, jobBusy()], () => {
    const el = $("#php");
    const cards = groups.map((g) => {
      const v = g.name.slice(4);
      const users = S.sites.filter((s) => s.php === v).map((s) => s.slug);
      const body = `<dl class="facts">
          ${S.running && g.ports ? `<dt>Port</dt><dd class="mono">${esc(portText(g.ports))}</dd>` : ""}
          <dt>Sites</dt><dd>${users.length ? esc(users.join(", ")) : '<span class="muted">None yet</span>'}${v === S.default_php ? ' <span class="note">default for new sites</span>' : ""}</dd></dl>`;
      const buttons = `<button class="btn small" data-phpini="${esc(v)}" title="Change PHP ${esc(v)}'s settings and extensions in a form. Saving restarts it.">Settings and extensions</button>
        <button class="btn small ghost" data-phpini-text="${esc(v)}" title="Open the php.ini of PHP ${esc(v)} in your text editor">${icon("open")}php.ini as text</button>
        <button class="btn small ghost" data-log="${esc(logFor(g.name))}">${icon("logs")}Error log</button>
        ${S.running ? `<button class="btn small ${failing(g) ? "primary" : "ghost"}" data-restart="${esc(g.name)}">${icon("reload")}Restart</button>` : ""}`;
      return serverCard(g, body, buttons);
    }).join("");
    el.innerHTML = `
      <div class="server-list">${cards || '<div class="muted">No PHP version is set up yet. Install one below.</div>'}</div>
      ${S.warnings.map((w) => `<div class="notice warn"><b>Note</b>${esc(w)}</div>`).join("")}
      <div class="section"><h2>Versions</h2><p>Downloaded from windows.php.net and checked against a pinned SHA-256. A site picks its version on the Sites page.</p>${versionRows("php")}</div>
      ${extensionsSection()}`;
    el.querySelectorAll("[data-phpini]").forEach((b) => (b.onclick = () => openINI(b.dataset.phpini)));
    el.querySelectorAll("[data-phpini-text]").forEach((b) => (b.onclick = () => open("phpini", b.dataset.phpiniText)));
    el.querySelectorAll("[data-log]").forEach((b) => (b.onclick = () => showLog(b.dataset.log)));
    el.querySelectorAll("[data-restart]").forEach((b) => (b.onclick = () => act("/api/restart", { unit: b.dataset.restart }, "Restarted " + label(b.dataset.restart)).catch(() => {})));
    el.querySelectorAll("[data-ext]").forEach((b) => (b.onclick = () => {
      const [name, on] = b.dataset.ext.split("|");
      if (on === "0" && !confirm(`Turn off ${name}? PHP restarts without it; the files stay.`)) return;
      act("/api/ext", { name, on: on === "1" }, on === "0" ? `Turned off ${name}` : "").catch(() => {});
    }));
    $("#odbc-get") && ($("#odbc-get").onclick = () => open("url", S.links.odbc));
    wireVersions(el);
  });
}

// extensionsSection lists the PHP extensions devstack downloads itself (php.net's
// zips leave them out), with the ODBC driver they need.
const EXT_TITLES = { sqlsrv: "Microsoft SQL Server", pdo_sqlsrv: "Microsoft SQL Server for PDO" };
function extensionsSection() {
  const exts = S.extensions || [];
  if (!exts.length) return "";
  const job = installJob(), busy = jobBusy();
  const rows = exts.map((x) => {
    const installing = job && job.title === `Installing ${x.name}` && !job.finished;
    let note = "", action;
    if (installing) {
      const step = job.steps.find((s) => s.state === "running") || job.steps[job.steps.length - 1];
      action = `<div class="progress"><div class="bar-track">${barFill(step)}</div><div class="muted">${esc(step ? step.label.replace("Download ", "") : "Starting…")}</div></div>`;
    } else if (x.enabled && x.missing && x.missing.length) {
      note = ` · not downloaded for PHP ${esc(x.missing.join(", "))}`;
      action = `<button class="btn small" data-ext="${esc(x.name)}|1" ${busy ? "disabled" : ""}>Download</button>`;
    } else if (x.enabled) {
      action = `<span class="in-use"><i class="mark"></i>On</span><button class="btn small ghost" data-ext="${esc(x.name)}|0" ${busy ? "disabled" : ""}>Turn off</button>`;
    } else {
      action = `<button class="btn small" data-ext="${esc(x.name)}|1" ${busy ? "disabled" : ""}>Install</button>`;
    }
    return `<div class="rowi"><div class="grow"><b class="mono">${esc(x.name)}</b><div class="sub">${esc(EXT_TITLES[x.name] || "")}${note}</div></div>${action}</div>`;
  }).join("");
  const o = S.odbc;
  let odbc = "";
  if (o && o.needed && !o.driver) {
    odbc = `<div class="notice warn" style="margin-top:8px"><div class="grow">SQL Server needs Microsoft's ODBC Driver for SQL Server (17 or 18). PHP loads sqlsrv without it, but connections fail until it is installed. Its installer asks for administrator rights.</div>
      <button class="btn small" id="odbc-get">${icon("open")}Get the driver</button></div>`;
  } else if (o && o.needed) {
    odbc = `<p class="muted" style="margin:6px 2px 0;font-size:12px">Connects through ${esc(o.driver)}.</p>`;
  }
  return `<div class="section"><h2>Extra extensions</h2><p>Not in php.net's builds. Downloaded from php.net's PECL builds for each PHP version and checked against a pinned SHA-256.</p>
    <div class="rows">${rows}</div>${odbc}</div>`;
}

// The MySQL password reaches the page only after Windows confirms the user
// (Settings can turn that off). getPassword asks once, then the server keeps
// it unlocked for a minute; after that the page forgets it again.
let pwShown = false;
let relockTimer;
async function getPassword() {
  if (db && db.password) return db.password;
  try {
    const r = await api("/api/db/reveal", {});
    db.password = r.password;
    clearTimeout(relockTimer);
    if (r.unlocked_ms) relockTimer = setTimeout(() => {
      pwShown = false;
      if (db) { delete db.password; db.locked = true; }
      delete drawn.db; delete drawn.panel;
      if (S) render();
      loadServerData();
    }, r.unlocked_ms);
    return r.password;
  } catch (e) {
    if (!/cancel/i.test(e.message)) toast(e.message, true);
    return null;
  }
}
async function togglePassword() {
  if (!pwShown && !(await getPassword())) return;
  pwShown = !pwShown;
  drawn.db = drawn.panel = null;
  render();
}
async function copyPassword() {
  const pw = await getPassword();
  if (pw) copy(pw, "Password");
}
// pwCells is the password value and its eye and copy buttons, for a dl.kv row.
const pwCells = (pre) => `<dd class="mono">${pwShown && db.password ? esc(db.password) : "••••••••••••"}</dd>
  <span class="pw-btns"><button class="btn small ghost icon-only" id="${pre}-show" aria-label="${pwShown ? "Hide" : "Show"} the password" title="${pwShown ? "Hide" : "Show"} the password${db.locked && !pwShown ? " (asks Windows that it's you)" : ""}">${icon(pwShown ? "eyeOff" : "eye")}</button><button class="btn small ghost icon-only" id="${pre}-copy" aria-label="Copy the password" title="Copy the password">${icon("copy")}</button></span>`;

function renderDB() {
  ensureServerData();
  const g = services().find((x) => x.name === "mysql");
  draw("db", [g, S.running, db, packages, progressKey(installJob()), jobBusy(), pwShown], () => {
    const el = $("#db");
    let card;
    if (db && !db.enabled) {
      const body = db.installed
        ? `<div class="muted">MySQL is off. Turn it on to give your sites a database.</div>`
        : `<div class="muted">MySQL is off, and no version is installed yet. Install one below, then turn it on.</div>`;
      const btn = db.installed
        ? `<div class="row-btns"><button class="btn small primary" id="mysql-on" ${jobBusy() ? "disabled" : ""}>Turn on MySQL ${esc(db.installed)}</button></div>`
        : "";
      card = `<div class="server off"><div class="server-top"><i class="mark off"></i><h2>MySQL</h2><div class="grow"></div><span class="state">Off</span></div>${body}${btn}</div>`;
    } else if (g) {
      let facts;
      if (!db) facts = '<div class="muted">Loading…</div>';
      else if (!db.ready) facts = '<div class="muted">A root password is generated the first time MySQL starts.</div>';
      else facts = `<dl class="kv"><dt>Host</dt><dd>${esc(db.host)}</dd><span></span><dt>Port</dt><dd class="num">${esc(db.port)}</dd><span></span>
        <dt>User</dt><dd>${esc(db.user)}</dd><span></span>
        <dt>Password</dt>${pwCells("pw")}</dl>`;
      const pma = db && db.phpmyadmin;
      const buttons = `${pma ? `<button class="btn small" id="pma-open" ${S.running ? "" : `disabled title="Start the servers to open it"`}>${icon("open")}Open phpMyAdmin</button>` : ""}
        <button class="btn small ghost" data-log="mysql.log">${icon("logs")}Log</button>
        ${S.running ? `<button class="btn small ${failing(g) ? "primary" : "ghost"}" data-restart="mysql">${icon("reload")}Restart</button>` : ""}
        <button class="btn small ghost" id="mysql-off" title="Stop MySQL and don't start it with the other servers. Your databases stay on disk.">Turn off</button>`;
      card = serverCard(g, facts, buttons);
    } else {
      card = '<div class="muted">MySQL isn\'t set up in this stack.</div>';
    }
    el.innerHTML = `${card}
      <p class="muted" style="margin:0;font-size:12.5px">Every site shares this MySQL server. The AI guide tells coding agents how to reach it.</p>
      <div class="section"><h2>Versions</h2><p>Downloaded from mysql.com and checked against a pinned SHA-256. Each version keeps its own databases.</p>${versionRows("mysql")}</div>
      <div class="section"><h2>phpMyAdmin</h2><p>${pmaNote()}</p>${versionRows("phpmyadmin")}</div>`;
    $("#mysql-on") && ($("#mysql-on").onclick = () => setMySQL(""));
    $("#mysql-off") && ($("#mysql-off").onclick = () => {
      if (confirm("Turn off MySQL? Sites that use a database stop working until you turn it on again. Your databases stay on disk.")) setMySQL("off");
    });
    $("#pma-open") && ($("#pma-open").onclick = () => open("url", db.phpmyadmin.url));
    $("#pw-show") && ($("#pw-show").onclick = togglePassword);
    $("#pw-copy") && ($("#pw-copy").onclick = copyPassword);
    el.querySelectorAll("[data-log]").forEach((b) => (b.onclick = () => showLog(b.dataset.log)));
    el.querySelectorAll("[data-restart]").forEach((b) => (b.onclick = () => act("/api/restart", { unit: b.dataset.restart }, "Restarted MySQL").catch(() => {})));
    wireVersions(el);
  });
}

// pmaNote says where phpMyAdmin is, or what it's waiting for.
function pmaNote() {
  const base = "Manage your databases in the browser. Downloaded from phpmyadmin.net and checked against a pinned SHA-256.";
  if (!db) return base;
  if (db.phpmyadmin) return `${base} It runs at ${esc(host(db.phpmyadmin.url))} with the default PHP and logs in as root by itself.`;
  const installed = packages && packages.some((p) => p.kind === "phpmyadmin" && p.installed);
  if (installed && !db.enabled) return `${base} Turn on MySQL to use it.`;
  return `${base} Once it's installed, the MySQL card above opens it, logged in as root.`;
}

// ---- Mail ------------------------------------------------------------------------
// Mailpit catches what sites send. The page shows its address, the newest
// messages (fetched again whenever the inbox counts change) and its versions.

let mailList = null, mailListFor = "";
async function loadMailList() {
  try { mailList = await api("/api/mail/messages"); } catch { mailList = null; }
  drawn.mail = null;
  render();
}

function setMail(on) {
  return act("/api/mail", { on }, on ? "Turned on Mailpit" : "Turned off Mailpit").then(() => loadServerData()).catch(() => {});
}

const mailEnv = (m) => `MAIL_MAILER=smtp\nMAIL_HOST=127.0.0.1\nMAIL_PORT=${m.smtp_port}`;

// ago is a short "5 min ago" for a time today, else the date.
function ago(t) {
  const d = new Date(t), s = (Date.now() - d) / 1000;
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  return d.toLocaleDateString([], { dateStyle: "medium" });
}
const who = (a) => a ? (a.name || a.address) : "";

function renderMail() {
  ensureServerData();
  const m = S.mail;
  const g = services().find((x) => x.name === "mailpit");
  const marker = m && m.enabled && S.running && m.counted ? `${m.total}|${m.unread}` : "";
  if (marker !== mailListFor) {
    mailListFor = marker;
    if (marker) loadMailList(); else mailList = null;
  }
  draw("mail", [m, g, S.running, packages, progressKey(installJob()), jobBusy(), mailList], () => {
    const el = $("#mail");
    let card, recent = "";
    if (!m || !m.enabled) {
      const body = m && m.installed
        ? '<div class="muted">Mailpit is off, so mail your sites send goes out for real, or fails.</div>'
        : '<div class="muted">Mailpit catches every email your sites send, so none reach real inboxes. Install it below.</div>';
      const btn = m && m.installed
        ? `<div class="row-btns"><button class="btn small primary" id="mail-on" ${jobBusy() ? "disabled" : ""}>Turn on Mailpit ${esc(short(m.installed))}</button></div>` : "";
      card = `<div class="server off"><div class="server-top"><i class="mark off"></i><h2>Mailpit</h2><div class="grow"></div><span class="state">Off</span></div>${body}${btn}</div>`;
    } else {
      const unit = g || { name: "mailpit", version: m.version, state: "stopped", restarts: 0 };
      const caught = !S.running ? '<span class="muted">Start the servers to catch mail</span>'
        : m.counted ? `${plural(m.total, "message")}${m.unread ? `, ${m.unread} unread` : ""}` : '<span class="muted">Not answering yet</span>';
      const facts = `<dl class="kv"><dt>SMTP</dt><dd>127.0.0.1:${esc(m.smtp_port)}</dd><span></span>
        <dt>Inbox</dt><dd>${esc(host(m.inbox))}</dd><span></span>
        <dt>Caught</dt><dd style="font-family:inherit">${caught}</dd><span></span></dl>`;
      const buttons = `<button class="btn small" id="mail-inbox" ${S.running ? "" : `disabled title="Start the servers to open it"`}>${icon("open")}Open inbox</button>
        <button class="btn small ghost" data-log="mailpit.log">${icon("logs")}Log</button>
        ${S.running ? `<button class="btn small ${failing(unit) ? "primary" : "ghost"}" data-restart="mailpit">${icon("reload")}Restart</button>` : ""}
        <button class="btn small ghost" id="mail-off" title="Stop Mailpit and don't start it with the other servers. Caught mail stays on disk.">Turn off</button>`;
      card = serverCard(unit, facts, buttons);
      if (S.running) {
        const rows = (mailList || []).map((x) => `<button class="rowi mail-row${x.read ? "" : " unread"}" data-url="${esc(x.url)}" title="Open in Mailpit">
            <div class="grow"><b>${esc(x.subject || "(no subject)")}</b><div class="sub">${esc(who(x.from))} → ${esc(x.to.map(who).join(", "))}${x.snippet ? " · " + esc(x.snippet) : ""}</div></div>
            <span class="muted num">${esc(ago(x.created))}</span></button>`).join("");
        recent = `<div class="section"><h2>Newest mail</h2><div class="rows">${mailList === null ? '<div class="rowi muted">Loading…</div>'
          : rows || '<div class="rowi muted">Nothing caught yet. Mail your sites send shows up here.</div>'}</div></div>`;
      }
    }
    const how = m && m.enabled ? `<div class="section"><h2>Sending from a site</h2>
        <p>PHP's mail() reaches Mailpit by itself. Laravel and other apps that read .env need these lines; each site's details can write them for you.</p>
        <div class="rows"><div class="rowi"><pre class="mono env-lines">${esc(mailEnv(m))}</pre><div class="grow"></div>
          <button class="btn small ghost" id="mail-copy-env">${icon("copy")}Copy</button></div></div></div>` : "";
    el.innerHTML = `${card}${recent}${how}
      <div class="section"><h2>Versions</h2><p>Downloaded from Mailpit's GitHub releases and checked against a pinned SHA-256.</p>${versionRows("mailpit")}</div>`;
    $("#mail-on") && ($("#mail-on").onclick = () => setMail(true));
    $("#mail-off") && ($("#mail-off").onclick = () => {
      if (confirm("Turn off Mailpit? Mail your sites send then goes out for real, or fails. Caught mail stays on disk.")) setMail(false);
    });
    $("#mail-inbox") && ($("#mail-inbox").onclick = () => open("url", m.inbox));
    $("#mail-copy-env") && ($("#mail-copy-env").onclick = () => copy(mailEnv(m), "Settings"));
    el.querySelectorAll("[data-url]").forEach((b) => (b.onclick = () => open("url", b.dataset.url)));
    el.querySelectorAll("[data-log]").forEach((b) => (b.onclick = () => showLog(b.dataset.log)));
    el.querySelectorAll("[data-restart]").forEach((b) => (b.onclick = () => act("/api/restart", { unit: b.dataset.restart }, "Restarted Mailpit").catch(() => {})));
    wireVersions(el);
  });
}

// panelMail is the Mail section of a site's details: where its mail goes.
function panelMail(s) {
  const m = S.mail;
  if (!m || !m.enabled) {
    return m && m.installed
      ? `<div class="muted">Mailpit is off, so this site's mail goes out for real.</div><div class="row-btns" style="margin-top:6px"><button class="btn small" id="p-mail-on" ${jobBusy() ? "disabled" : ""}>Turn on Mailpit</button></div>`
      : '<div class="muted">Install Mailpit on the Mail page to catch the mail this site sends.</div>';
  }
  const e = s.mail;
  let text, btn = "";
  if (!e) {
    text = "PHP's mail() goes to Mailpit. This site has no .env to set up.";
  } else if (e.ours) {
    text = "This site's .env sends mail to Mailpit.";
  } else {
    const to = e.mailer === "smtp" && e.host ? ` to <span class="mono">${esc(e.host)}${e.port ? ":" + esc(e.port) : ""}</span>` : "";
    text = e.mailer ? `This site's .env sends mail with <b class="mono">${esc(e.mailer)}</b>${to}, not to Mailpit.` : "This site's .env doesn't say how to send mail (MAIL_MAILER).";
    btn = `<button class="btn small" id="p-mail" ${jobBusy() ? "disabled" : ""} title="Set MAIL_MAILER, MAIL_HOST and MAIL_PORT in .env">Send mail to Mailpit</button>`;
  }
  return `<div class="env-db">${text}</div><div class="row-btns">${btn}<button class="btn small ${btn ? "ghost" : ""}" id="p-inbox" ${S.running ? "" : `disabled title="Start the servers to open it"`}>${icon("open")}Inbox</button></div>`;
}

function siteToMailpit(s) {
  const m = S.mail;
  if (!confirm(`Send ${s.slug}'s mail to Mailpit? devstack sets these in its .env:\n\n${mailEnv(m)}`)) return;
  act("/api/site/mail", { slug: s.slug }).then((r) => {
    toast(r.config_cached
      ? "Done. Laravel has cached its config: run php artisan config:clear"
      : `${s.slug}'s mail now goes to Mailpit`);
  }).catch(() => {});
}

// ---- php.ini settings editor ----------------------------------------------------
// A form over a PHP version's own php.ini (etc-custom\php\X.Y\php.ini). Nothing is
// written until Save, and the server edits the lines in place, so the file stays
// the same one the "as text" button opens.

// INI_GROUPS are the settings beginners reach for. def is PHP's built-in default,
// shown when the file doesn't set one.
const INI_GROUPS = [
  { title: "Errors", items: [
    { key: "display_errors", type: "bool", label: "Show errors on the page", help: "Print PHP errors in the browser. Keep it on while developing.", def: "Off" },
    { key: "error_reporting", type: "select", label: "Which problems to report", def: "E_ALL", options: [
      ["E_ALL", "Everything (best while developing)"],
      ["E_ALL & ~E_DEPRECATED", "Everything except deprecation notices"],
      ["E_ALL & ~E_NOTICE & ~E_DEPRECATED", "Errors and warnings only"],
      ["E_ERROR | E_PARSE", "Fatal errors only"]] },
    { key: "display_startup_errors", type: "bool", label: "Show startup errors", help: "Problems PHP hits while starting, such as an extension that won't load.", def: "On" },
    { key: "log_errors", type: "bool", label: "Write errors to the log", help: "To this PHP version's error log on the Logs page.", def: "On" },
  ] },
  { title: "Uploads and forms", items: [
    { key: "upload_max_filesize", type: "size", label: "Largest file upload", help: "Per file. Use K, M or G, like 64M.", def: "2M" },
    { key: "post_max_size", type: "size", label: "Largest form submission", help: "The whole request, every file in it included. Keep it at least as large as the upload limit.", def: "8M" },
    { key: "max_file_uploads", type: "int", label: "Files per upload", help: "How many files one form can send.", def: "20" },
    { key: "max_input_vars", type: "int", label: "Form fields per request", help: "Raise it if big forms (WordPress menus, admin panels) lose fields.", def: "1000" },
  ] },
  { title: "Limits", items: [
    { key: "memory_limit", type: "size", label: "Memory per request", help: "Like 512M or 1G. -1 means no limit.", def: "128M" },
    { key: "max_execution_time", type: "int", label: "Time limit per request", help: "In seconds. 0 means no limit. php in a terminal has none.", def: "30" },
    { key: "max_input_time", type: "int", label: "Time to receive a request", help: "In seconds, for slow uploads. -1 uses the time limit above.", def: "-1", signed: true },
  ] },
  { title: "Date and time", items: [
    { key: "date.timezone", type: "tz", label: "Time zone", help: "What date() and now() use, like Asia/Kolkata or Europe/London.", def: "UTC" },
  ] },
  { title: "Speed (OPcache)", items: [
    { key: "opcache.enable", type: "bool", label: "Cache compiled PHP", help: "Makes every request faster.", def: "1", num: true },
    { key: "opcache.validate_timestamps", type: "bool", label: "Notice changed files", help: "Turn off only to test production speed: code edits then need a PHP restart.", def: "1", num: true },
    { key: "opcache.revalidate_freq", type: "int", label: "Seconds between checks", help: "How often changed files are noticed. 0 notices them on the next request.", def: "2" },
    { key: "opcache.memory_consumption", type: "int", label: "Cache size (MB)", help: "Raise it for very large projects.", def: "128" },
  ] },
  { title: "Sessions", items: [
    { key: "session.gc_maxlifetime", type: "int", label: "Session lifetime", help: "Seconds of inactivity before a session can be cleaned up.", def: "1440" },
    { key: "session.cookie_httponly", type: "bool", label: "Hide session cookies from JavaScript", def: "0", num: true },
  ] },
  { title: "Language", items: [
    { key: "short_open_tag", type: "bool", label: "Allow <? short tags", help: "Old code sometimes uses <? instead of <?php.", def: "On" },
    { key: "allow_url_fopen", type: "bool", label: "Open URLs like files", help: "Lets file_get_contents() fetch https:// addresses.", def: "On" },
  ] },
];
const INI_KNOWN = new Set(INI_GROUPS.flatMap((g) => g.items.map((i) => i.key)));

const EXT_INFO = {
  bz2: "Read and write .bz2 archives", com_dotnet: "Windows COM and .NET objects", curl: "HTTP requests (APIs, Guzzle, Laravel's HTTP client)",
  dba: "Berkeley-style key-value databases", enchant: "Spell checking", exif: "Photo metadata (camera, orientation)", ffi: "Call C libraries from PHP",
  fileinfo: "Detect file types; Laravel needs it for uploads", ftp: "FTP client", gd: "Resize, crop and draw images", gettext: "Translations with gettext",
  gmp: "Very large numbers", imap: "Read mailboxes over IMAP", intl: "Language, number and date formatting (Laravel, Symfony)", ldap: "LDAP and Active Directory",
  mbstring: "UTF-8 strings; most frameworks need it", mysqli: "MySQL (WordPress, plain PHP)", odbc: "ODBC databases", openssl: "HTTPS and encryption; Composer needs it",
  pdo_firebird: "Firebird through PDO", pdo_mysql: "MySQL through PDO (Laravel, Symfony)", pdo_odbc: "ODBC through PDO", pdo_pgsql: "PostgreSQL through PDO",
  pdo_sqlite: "SQLite through PDO", pdo_sqlsrv: "Microsoft SQL Server through PDO", pgsql: "PostgreSQL", shmop: "Shared memory", snmp: "SNMP network monitoring",
  soap: "SOAP web services", sockets: "Low-level network sockets", sodium: "Modern encryption (libsodium)", sqlite3: "SQLite", sqlsrv: "Microsoft SQL Server",
  sysvshm: "System V shared memory", tidy: "Clean up and repair HTML", xsl: "XSLT transforms", zip: "Read and write .zip files; Composer needs it",
};
const extBase = (n) => n.replace(/_\d+_n?ts_x(64|86)$/, "");

const isOn = (v) => /^(on|1|true|yes)$/i.test(String(v ?? "").trim());
const SIZE_RE = /^(-1|\d+[KMG]?)$/i;
const toBytes = (v) => {
  const m = /^(\d+)([KMG]?)$/i.exec(String(v ?? "").trim());
  return m ? +m[1] * 1024 ** " KMG".indexOf((m[2] || " ").toUpperCase()) : Infinity;
};
let TIMEZONES = [];
try { TIMEZONES = Intl.supportedValuesOf("timeZone"); if (!TIMEZONES.includes("UTC")) TIMEZONES.unshift("UTC"); } catch {}

// ed is the open editor: the file as read (orig*) and as edited (cur*).
// A setting's value is null when the file doesn't set it.
let ed = null;

async function openINI(php) {
  const dlg = $("#ini");
  ed = { php, loading: true };
  $("#ini-title").textContent = `PHP ${php} settings`;
  $("#ini-path").textContent = "";
  $("#ini-search").value = "";
  $("#ini-body").innerHTML = '<div class="ini-empty">Loading…</div>';
  drawFoot();
  if (!dlg.open) dlg.showModal();
  await loadINI();
}

// loadINI (re)reads the file. quiet: only when it changed (the window
// regaining focus), so coming back doesn't scroll the sheet to the top.
async function loadINI(quiet) {
  const mine = ed; // closed or reopened for another version meanwhile: drop the answer
  try {
    const d = await api("/api/phpini?php=" + encodeURIComponent(mine.php));
    if (ed !== mine) return;
    if (quiet && mine.data && d.rev === mine.data.rev) return;
    const extras = d.order.filter((k) => !INI_KNOWN.has(k) && !d.owned.some((o) => o.toLowerCase() === k.toLowerCase()));
    const keys = [...INI_KNOWN, ...extras];
    const orig = Object.fromEntries(keys.map((k) => [k, d.settings[k] ?? null]));
    const origExt = Object.fromEntries(d.extensions.map((x) => [x.name, x.on]));
    ed = { php: ed.php, data: d, extras, orig, cur: { ...orig }, origExt, curExt: { ...origExt }, bad: new Set(), saved: "" };
    $("#ini-path").textContent = d.path;
    $("#ini-path").title = d.path;
    drawINI();
  } catch (e) {
    if (ed !== mine) return;
    ed = { php: ed.php, failed: e.message };
    $("#ini-body").innerHTML = `<div class="notice error">${esc(e.message)}</div>`;
  }
  drawFoot();
}

// stackValue is what stack.toml [php.ini] sets key to, which wins over the file.
function stackValue(key) {
  const s = ed.data.stack || {};
  const k = Object.keys(s).find((x) => x.toLowerCase() === key.toLowerCase());
  return k === undefined ? undefined : s[k];
}

function iniControl(it) {
  const k = esc(it.key), v = ed.cur[it.key], shown = v ?? it.def ?? "";
  const locked = stackValue(it.key) !== undefined ? "disabled" : "";
  const aria = `aria-label="${esc(it.label || it.key)}"`;
  switch (it.type) {
    case "bool":
      return `<label class="switch"><input type="checkbox" data-ini="${k}" ${isOn(shown) ? "checked" : ""} ${locked} ${aria}><span></span></label>`;
    case "select": {
      const opts = [...it.options];
      if (!opts.some(([val]) => val === shown)) opts.push([shown, shown]);
      return `<select data-ini="${k}" ${locked} ${aria}>${opts.map(([val, txt]) => `<option value="${esc(val)}" ${val === shown ? "selected" : ""}>${esc(txt)}</option>`).join("")}</select>`;
    }
    case "tz":
      return `<input type="text" data-ini="${k}" value="${esc(shown)}" list="ini-tz" spellcheck="false" ${locked} ${aria}>`;
    default:
      return `<input type="text" class="${it.type === "text" ? "" : "short"}" data-ini="${k}" value="${esc(shown)}" spellcheck="false" ${locked} ${aria}>`;
  }
}

function iniRow(it) {
  const sv = stackValue(it.key);
  const words = [it.label, it.key, it.help].join(" ").toLowerCase();
  return `<div class="ini-row" data-row="${esc(it.key)}" data-words="${esc(words)}">
    <div><div class="ini-label">${esc(it.label || it.key)}${it.label ? `<span class="ini-key">${esc(it.key)}</span>` : ""}</div>
      ${it.help ? `<div class="ini-help">${esc(it.help)}</div>` : ""}</div>
    <div class="ini-ctl">${it.extra ? `<button class="btn small ghost icon-only" data-ini-remove="${esc(it.key)}" title="Turn ${esc(it.key)} off (PHP's default applies)" aria-label="Remove ${esc(it.key)}">${icon("cross")}</button>` : ""}
      <button class="link ini-undo" data-ini-undo="${esc(it.key)}" hidden>Undo</button>${iniControl(it)}</div>
    ${sv !== undefined ? `<div class="ini-note locked">stack.toml sets this to <span class="mono">${esc(sv)}</span> under [php.ini] for every PHP version, and that wins. Change it there.</div>` : ""}
    <div class="ini-note" data-note="${esc(it.key)}" hidden></div>
  </div>`;
}

function drawINI() {
  const d = ed.data;
  const exts = [...d.extensions].sort((a, b) => a.name.localeCompare(b.name));
  const groups = INI_GROUPS.map((g) => `<div class="ini-group"><h3>${esc(g.title)}</h3>${g.items.map(iniRow).join("")}</div>`).join("");
  const extRows = exts.map((x) => {
    const info = EXT_INFO[extBase(x.name)] || "";
    return `<div class="ini-row" data-ext-row="${esc(x.name)}" data-words="${esc((x.name + " " + info).toLowerCase())}">
      <div><div class="ini-label mono">${esc(x.name)}</div>${info ? `<div class="ini-help">${esc(info)}</div>` : ""}</div>
      <div class="ini-ctl"><button class="link ini-undo" data-ext-undo="${esc(x.name)}" hidden>Undo</button>
        <label class="switch"><input type="checkbox" data-ext-on="${esc(x.name)}" ${x.on ? "checked" : ""} aria-label="${esc(x.name)} extension"><span></span></label></div>
    </div>`;
  }).join("");
  const extras = ed.extras.map((k) => iniRow({ key: k, type: "text", extra: true })).join("");
  $("#ini-body").innerHTML = `${groups}
    <div class="ini-group" id="ini-exts"><h3>Extensions <span class="h-note">Every extension in PHP ${esc(ed.php)}'s ext folder</span></h3>${extRows || '<div class="ini-empty">No extensions found.</div>'}</div>
    <div class="ini-group" id="ini-more"><h3>More settings <span class="h-note">Everything else your php.ini sets</span></h3>
      <div id="ini-extras">${extras}</div>
      <div class="ini-add"><input type="text" id="ini-add-key" placeholder="setting, like zend.assertions" spellcheck="false" aria-label="New setting name">
        <input type="text" id="ini-add-val" placeholder="value" spellcheck="false" aria-label="New setting value">
        <button class="btn small" id="ini-add">Add</button></div>
      <div class="ini-note bad" id="ini-add-err" hidden></div></div>
    <div class="ini-empty" id="ini-none" hidden>Nothing matches. Add it under More settings, or open the file as text.</div>
    <datalist id="ini-tz">${TIMEZONES.map((z) => `<option value="${esc(z)}">`).join("")}</datalist>`;
  wireINI($("#ini-body"));
  $("#ini-add").onclick = addINISetting;
  $("#ini-add-val").onkeydown = (e) => { if (e.key === "Enter") addINISetting(); };
  checkINI();
  filterINI();
}

function wireINI(root) {
  const item = (key) => INI_GROUPS.flatMap((g) => g.items).find((i) => i.key === key) || { key, type: "text" };
  root.querySelectorAll("[data-ini]").forEach((el) => {
    const it = item(el.dataset.ini);
    const update = () => {
      if (it.type === "bool") {
        const was = ed.orig[it.key] ?? it.def;
        const numeric = it.num || /^[01]$/.test(String(was).trim());
        ed.cur[it.key] = isOn(was) === el.checked ? ed.orig[it.key] // flipped back: no change
          : el.checked ? (numeric ? "1" : "On") : (numeric ? "0" : "Off");
      } else {
        ed.cur[it.key] = el.value.trim();
      }
      checkINI();
    };
    el.addEventListener(it.type === "bool" || it.type === "select" ? "change" : "input", update);
  });
  root.querySelectorAll("[data-ini-undo]").forEach((b) => (b.onclick = () => {
    const k = b.dataset.iniUndo;
    ed.cur[k] = ed.orig[k];
    if (ed.orig[k] === null && !INI_KNOWN.has(k)) { // a setting added here: take its row away
      ed.extras = ed.extras.filter((x) => x !== k);
      delete ed.cur[k]; delete ed.orig[k];
      b.closest(".ini-row").remove();
    } else {
      const row = b.closest(".ini-row"), old = row.querySelector("[data-ini]");
      (old.closest(".switch") || old).replaceWith(document.createRange().createContextualFragment(iniControl(item(k))));
      wireINI(row);
    }
    checkINI();
  }));
  root.querySelectorAll("[data-ini-remove]").forEach((b) => (b.onclick = () => {
    const k = b.dataset.iniRemove;
    ed.cur[k] = null;
    b.closest(".ini-row").querySelector("[data-ini]").value = "";
    checkINI();
  }));
  root.querySelectorAll("[data-ext-on]").forEach((cb) => (cb.onchange = () => { ed.curExt[cb.dataset.extOn] = cb.checked; checkINI(); }));
  root.querySelectorAll("[data-ext-undo]").forEach((b) => (b.onclick = () => {
    const n = b.dataset.extUndo;
    ed.curExt[n] = ed.origExt[n];
    $(`#ini [data-ext-on="${CSS.escape(n)}"]`).checked = ed.origExt[n];
    checkINI();
  }));
}

function addINISetting() {
  const key = $("#ini-add-key").value.trim(), val = $("#ini-add-val").value.trim();
  const err = $("#ini-add-err");
  const fail = (msg) => { err.textContent = msg; err.hidden = false; $("#ini-add-key").classList.add("bad"); };
  err.hidden = true;
  $("#ini-add-key").classList.remove("bad");
  if (!/^[A-Za-z0-9_.\-]+$/.test(key)) return fail("A setting name is letters, digits, _ . and - only, like zend.assertions.");
  if (/^(zend_)?extension$/i.test(key)) return fail("Turn extensions on and off in the Extensions list above.");
  if (ed.data.owned.some((o) => o.toLowerCase() === key.toLowerCase())) return fail(`devstack sets ${key} itself, because it points into devstack's folders.`);
  const have = Object.keys(ed.cur).find((k) => k.toLowerCase() === key.toLowerCase());
  if (have !== undefined) {
    const row = $(`#ini [data-row="${CSS.escape(have)}"]`);
    err.textContent = `Your php.ini already sets ${have}: change it here.`;
    err.hidden = false;
    $("#ini-search").value = "";
    filterINI();
    row?.scrollIntoView({ block: "center" });
    row?.querySelector("input, select")?.focus();
    return;
  }
  ed.extras.push(key);
  ed.orig[key] = null;
  ed.cur[key] = val;
  const frag = document.createRange().createContextualFragment(iniRow({ key, type: "text", extra: true }));
  const row = frag.firstElementChild;
  $("#ini-extras").append(row);
  wireINI(row);
  $("#ini-add-key").value = $("#ini-add-val").value = "";
  checkINI();
}

// changes lists what differs from the file, for the footer and for Save.
function iniChanges() {
  const set = {}, unset = [], extensions = {};
  for (const [k, v] of Object.entries(ed.cur)) {
    if (v === ed.orig[k]) continue;
    if (v === null || (v === "" && !INI_KNOWN.has(k))) { if (ed.orig[k] !== null) unset.push(k); }
    else set[k] = v;
  }
  for (const [n, on] of Object.entries(ed.curExt)) if (on !== ed.origExt[n]) extensions[n] = on;
  return { set, unset, extensions, count: Object.keys(set).length + unset.length + Object.keys(extensions).length };
}

// checkINI validates every value, marks changed rows and redraws the footer.
function checkINI() {
  ed.bad = new Set();
  const note = (key, cls, msg) => {
    const el = $(`#ini [data-note="${CSS.escape(key)}"]`);
    if (!el) return;
    el.className = "ini-note " + cls;
    el.textContent = msg;
    el.hidden = !msg;
  };
  for (const g of INI_GROUPS) for (const it of g.items) {
    const v = ed.cur[it.key] ?? it.def;
    let bad = "";
    if (it.type === "size" && !SIZE_RE.test(v)) bad = "Use a number with K, M or G, like 256M.";
    if (it.type === "int" && !(it.signed ? /^-?\d+$/ : /^\d+$/).test(v)) bad = "Use a whole number.";
    if (it.type === "tz" && TIMEZONES.length && !TIMEZONES.includes(v)) bad = "Pick a time zone from the list, like Asia/Kolkata.";
    $(`#ini [data-ini="${CSS.escape(it.key)}"]`)?.classList.toggle("bad", !!bad);
    if (bad) ed.bad.add(it.key);
    note(it.key, "bad", bad);
  }
  for (const k of ed.extras) {
    const v = ed.cur[k];
    note(k, "locked", v === null || v === "" ? "Off: PHP's default applies after you save." : "");
  }
  const eff = (k, d) => stackValue(k) ?? ed.cur[k] ?? d;
  if (!ed.bad.has("post_max_size") && !ed.bad.has("upload_max_filesize")) {
    const post = eff("post_max_size", "8M"), up = eff("upload_max_filesize", "2M");
    const small = post !== "-1" && post !== "0" && toBytes(post) < toBytes(up);
    note("post_max_size", "warn", small ? `Smaller than the upload limit (${up}), so uploads that big fail. Make it at least ${up}.` : "");
  }
  if (!ed.bad.has("opcache.validate_timestamps")) {
    note("opcache.validate_timestamps", "warn", isOn(eff("opcache.validate_timestamps", "1")) ? "" : "Off: code you edit won't show up until PHP restarts.");
  }
  document.querySelectorAll("#ini [data-row]").forEach((row) => {
    const k = row.dataset.row, changed = ed.cur[k] !== ed.orig[k];
    row.classList.toggle("changed", changed);
    row.querySelector("[data-ini-undo]").hidden = !changed;
  });
  document.querySelectorAll("#ini [data-ext-row]").forEach((row) => {
    const n = row.dataset.extRow, changed = ed.curExt[n] !== ed.origExt[n];
    row.classList.toggle("changed", changed);
    row.querySelector("[data-ext-undo]").hidden = !changed;
  });
  if (iniChanges().count) ed.saved = "";
  drawFoot();
}

function filterINI() {
  const q = $("#ini-search").value.trim().toLowerCase();
  let any = false;
  document.querySelectorAll("#ini-body .ini-group").forEach((g) => {
    let shown = 0;
    g.querySelectorAll(".ini-row").forEach((r) => { const hit = !q || r.dataset.words.includes(q); r.hidden = !hit; shown += hit; });
    const more = g.id === "ini-more";
    g.hidden = !!q && !shown && !more;
    any = any || shown > 0;
  });
  $("#ini-none") && ($("#ini-none").hidden = !q || any);
}

const phpRunning = () => S && S.running && S.groups.some((g) => g.name === "php-" + ed.php && g.state === "running");

// drawFoot shows what Save will do, or asks about unsaved changes (ed.asking).
function drawFoot() {
  const f = $("#ini-foot");
  f.classList.toggle("ask-save", !!ed?.asking);
  if (!ed || ed.loading || ed.failed) {
    f.innerHTML = `<div class="grow"></div><button class="btn" id="ini-done">Close</button>`;
    $("#ini-done").onclick = () => closeINI(true);
    return;
  }
  const ch = iniChanges();
  if (ed.asking) {
    f.innerHTML = `<div class="grow"><b>${ch.count} unsaved change${ch.count === 1 ? "" : "s"}.</b> Save before you ${ed.asking.what}?</div>
      <button class="btn ghost" id="ini-keep">Keep editing</button>
      <button class="btn" id="ini-discard">Discard</button>
      <button class="btn primary" id="ini-save" ${ed.bad.size ? "disabled" : ""}>Save</button>`;
    $("#ini-keep").onclick = () => { ed.asking = null; drawFoot(); };
    $("#ini-discard").onclick = () => { const then = ed.asking.then; ed.asking = null; then(); };
    $("#ini-save").onclick = async () => { const then = ed.asking.then; ed.asking = null; if (await saveINI()) then(); };
    $("#ini-save").focus();
    return;
  }
  let status;
  if (ed.saving) status = "Saving…";
  else if (ed.conflict) status = `<span class="error-text">php.ini was changed in another editor since you opened it here.</span> <button class="link" id="ini-reload">Load the new file</button> (your changes here are lost).`;
  else if (ed.error) status = `<span class="error-text">${esc(ed.error)}</span>`;
  else if (ed.bad.size) status = `<span class="error-text">Fix the highlighted value${ed.bad.size === 1 ? "" : "s"} to save.</span>`;
  else if (ch.count) status = `<b>${ch.count} unsaved change${ch.count === 1 ? "" : "s"}.</b> ` + (phpRunning()
    ? `Saving restarts PHP ${esc(ed.php)}, which takes a moment.`
    : S && S.running ? `PHP ${esc(ed.php)} isn't running; saving tries to start it.` : "The servers are stopped: they use the changes when they start.");
  else if (ed.saved) status = ed.saved;
  else status = "Nothing is written until you press Save.";
  f.innerHTML = `<div class="grow">${status}</div>
    <button class="btn" id="ini-done">${ch.count ? "Cancel" : "Close"}</button>
    <button class="btn primary" id="ini-save" ${!ch.count || ed.bad.size || ed.saving ? "disabled" : ""}>Save</button>`;
  $("#ini-done").onclick = () => closeINI();
  $("#ini-save").onclick = () => saveINI();
  $("#ini-reload") && ($("#ini-reload").onclick = () => loadINI());
}

async function saveINI() {
  const ch = iniChanges();
  if (!ch.count || ed.bad.size) return !ch.count;
  ed.saving = true; ed.error = ""; ed.conflict = false;
  drawFoot();
  const php = ed.php;
  try {
    const res = await api("/api/phpini", { php, rev: ed.data.rev, set: ch.set, unset: ch.unset, extensions: ch.extensions });
    await loadINI();
    if (!ed || ed.php !== php) return true; // closed meanwhile; the save itself went through
    ed.saved = res.error ? `<span class="error-text">Saved, but PHP ${esc(php)} didn't restart: ${esc(res.error.split("\n")[0])}</span> See the Logs page.`
      : res.running ? `Saved. ${(res.actions || []).some((a) => a.includes("php-" + php)) ? `PHP ${esc(php)} restarted with the new settings.` : "Applied."}`
      : "Saved. The servers use it when they start, and php in terminals uses it now.";
    drawFoot();
    poll(true);
    return true;
  } catch (e) {
    if (!ed || ed.php !== php) return false;
    ed.saving = false;
    if (e.status === 409) ed.conflict = true; else ed.error = e.message;
    drawFoot();
    return false;
  }
}

// closeINI closes the editor, asking first when there are unsaved changes.
function closeINI(force) {
  if (ed?.saving) return; // the save's answer is on its way; it would land on a closed editor
  if (!force && ed?.data && iniChanges().count) {
    ed.asking = { what: "close", then: () => closeINI(true) };
    drawFoot();
    return;
  }
  ed = null;
  $("#ini").close();
}

function openINIText() {
  const php = ed.php;
  const go = () => { closeINI(true); open("phpini", php); };
  if (ed?.data && iniChanges().count) {
    ed.asking = { what: "open the file as text", then: go };
    drawFoot();
    return;
  }
  go();
}

$("#ini-close").innerHTML = icon("cross");
$("#ini-text").innerHTML = icon("open") + "Open as text";
$("#ini-text").title = "Open this php.ini in your text editor, comments and all";
$("#ini-close").onclick = () => closeINI();
$("#ini-text").onclick = openINIText;
$("#ini").addEventListener("cancel", (e) => { e.preventDefault(); if (ed?.asking) { ed.asking = null; drawFoot(); } else closeINI(); });
$("#ini-search").oninput = filterINI;
// Back from editing the file as text: show what it says now, unless there are edits here to keep.
window.addEventListener("focus", () => { if (ed?.data && !ed.saving && !iniChanges().count && $("#ini").open) loadINI(true); });

// ---- logs --------------------------------------------------------------------------

let logTimer, logFiles = [], logName = localStorageGet("log") || "devstack.log";

// showLog opens the Logs page on one file.
function showLog(name) {
  logName = name;
  localStorageSet("log", name);
  go("logs");
}

async function renderLogs() {
  if (drawn.logsActive) return;
  drawn.logsActive = true;
  await refreshLog();
}

async function refreshLog() {
  clearTimeout(logTimer);
  if (page !== "logs") { drawn.logsActive = false; return; }
  try {
    const asked = logName;
    const r = await api("/api/logs?name=" + encodeURIComponent(asked));
    if (asked !== logName || page !== "logs") return;
    logFiles = r.files;
    if (r.files.length && !r.files.includes(logName)) {
      // That log isn't written yet (a server that never logged an error): show the
      // server's main log, or devstack's own.
      const base = logName.replace(/-error\.log$/, ".log");
      logName = r.files.includes(base) ? base : r.files.includes("devstack.log") ? "devstack.log" : r.files[0];
      return refreshLog();
    }
    drawLogFiles();
    const pre = $("#log-text");
    const atBottom = pre.scrollTop + pre.clientHeight >= pre.scrollHeight - 30;
    if (pre.dataset.name !== logName || pre.textContent !== (r.text || "(empty)")) {
      const switched = pre.dataset.name !== logName;
      pre.dataset.name = logName;
      pre.textContent = r.text || "(empty)";
      if (atBottom || switched) pre.scrollTop = pre.scrollHeight;
    }
  } catch (e) { toast(e.message, true); }
  if ($("#log-follow").checked) logTimer = setTimeout(refreshLog, 2000);
}

function drawLogFiles() {
  draw("logfiles", [logFiles, logName], () => {
    const item = (f, text) => `<button data-file="${esc(f)}" aria-current="${f === logName}" title="${esc(f)}">${esc(text)}</button>`;
    const own = logFiles.filter((f) => !f.startsWith("sites/"));
    const sites = logFiles.filter((f) => f.startsWith("sites/"));
    $("#log-files").innerHTML = `<h3>devstack and servers</h3>${own.map((f) => item(f, f)).join("")}` +
      (sites.length ? `<h3>Site access logs</h3>${sites.map((f) => item(f, f.replace(/^sites\//, "").replace(/\.access\.log$/, ""))).join("")}` : "");
    $("#log-files").querySelectorAll("[data-file]").forEach((b) => (b.onclick = () => {
      logName = b.dataset.file;
      localStorageSet("log", logName);
      drawn.logsActive = false;
      renderLogs();
    }));
  });
}

// ---- settings ------------------------------------------------------------------------

function renderSettings() {
  ensureServerData();
  const ng = services().find((g) => g.name === "nginx");
  draw("settings", [S.settings, S.root, S.version, S.dns, S.https, ng, S.running, packages, progressKey(installJob()), jobBusy()], () => {
    const sw = (id, on, labelText) => `<label class="switch"><input type="checkbox" id="${id}" ${on ? "checked" : ""} aria-label="${esc(labelText)}"><span></span></label>`;
    const row = (title, desc, action) => `<div class="rowi"><div class="grow"><div class="set-title">${title}</div><div class="set-desc">${desc}</div></div>${action}</div>`;
    $("#settings").innerHTML = `
      ${S.dns || S.https ? `<div class="section"><h2>Addresses</h2><div class="rows">${S.dns ? dnsRow(S.dns) : ""}${S.https ? httpsRow(S.https) : ""}</div></div>` : ""}
      <div class="section"><h2>Startup</h2><div class="rows">
        ${row("Start servers when devstack opens", "Your sites are up as soon as the app is.", sw("set-start", S.settings.start_servers, "Start servers when devstack opens"))}
        ${row("Open devstack when you sign in", "Starts in the tray, without a window.", sw("set-autostart", S.settings.autostart, "Open devstack when you sign in"))}
      </div></div>
      <div class="section"><h2>Privacy</h2><div class="rows">
        ${row("Ask Windows before showing the MySQL password", "Windows Hello, or your Windows password, before the password is shown or copied. Then it stays visible for one minute.", sw("set-pwcheck", S.settings.password_check, "Ask Windows before showing the MySQL password"))}
      </div></div>
      <div class="section"><h2>Updates</h2><div class="rows">
        <div class="rowi" id="update-card"></div>
        ${row("Check for updates automatically", "Once a day, from devstack.holostack.in. Updates install only when you choose.", sw("set-updates", S.settings.check_updates, "Check for updates automatically"))}
      </div></div>
      <div class="section"><h2>Stack</h2><div class="rows">
        ${row("Stack folder", `<span class="mono">${esc(S.root)}</span>`, `<button class="btn small" id="open-root">${icon("folder")}Open</button>`)}
        ${row("stack.toml", "Ports, PHP extensions and php.ini settings for every version. Reload on the Sites page after saving.", `<button class="btn small" id="edit-config">${icon("open")}Edit</button>`)}
        ${ng ? `<div class="rowi"><i class="mark ${mark(ng)}"></i><div class="grow"><div class="set-title">nginx ${esc(ng.version)}</div>
          <div class="set-desc">The web server in front of every site. ${esc(stateWord(ng))}${S.running && ng.ports ? ` on ${esc(portText(ng.ports))}` : ""}.</div></div>
          <button class="btn small ghost" data-log="nginx-error.log">${icon("logs")}Log</button>
          ${S.running ? `<button class="btn small ghost" data-restart="nginx">${icon("reload")}Restart</button>` : ""}</div>` : ""}
      </div>
      <p style="margin:10px 0 6px" class="muted">nginx versions</p>${versionRows("nginx")}</div>
      <div class="section"><h2>About</h2><div class="rows">
        ${row(`devstack ${esc(S.version)}`, `Open source. Runs unmodified builds of nginx (<button class="link" data-link="https://nginx.org/LICENSE">BSD</button>),
          PHP (<button class="link" data-link="https://www.php.net/license/">PHP License</button>),
          MySQL (<button class="link" data-link="https://www.mysql.com/about/legal/">GPLv2</button>),
          phpMyAdmin (<button class="link" data-link="https://www.phpmyadmin.net/license/">GPLv2</button>) and
          Mailpit (<button class="link" data-link="https://github.com/axllent/mailpit/blob/develop/LICENSE">MIT</button>), downloaded from their publishers.`,
          `<button class="btn small" data-link="https://github.com/harsha2805/devstack">${icon("open")}Source</button>`)}
        ${row("Support devstack", "devstack is free and open source. Sponsoring on GitHub keeps it going.",
          `<button class="btn small" data-link="https://github.com/sponsors/harsha2805">${icon("heart")}Sponsor</button>`)}
      </div></div>`;
    $("#open-root").onclick = () => open("folder", S.root);
    $("#edit-config").onclick = () => open("config", "");
    $("#set-start").onchange = (e) => act("/api/settings", { start_servers: e.target.checked }).catch(() => {});
    $("#set-autostart").onchange = (e) => act("/api/settings", { autostart: e.target.checked }).catch(() => {});
    $("#set-updates").onchange = (e) => act("/api/settings", { check_updates: e.target.checked }).catch(() => {});
    $("#set-pwcheck").onchange = (e) => {
      const on = e.target.checked;
      act("/api/settings", { password_check: on }, on ? "devstack asks Windows before showing the password" : "")
        .then(() => loadServerData())
        .catch(() => (e.target.checked = !on));
    };
    $("#set-dns") && ($("#set-dns").onchange = (e) => act("/api/dns", { on: e.target.checked }).catch(() => {}));
    $("#dns-repair") && ($("#dns-repair").onclick = () => act("/api/dns", { on: true }).catch(() => {}));
    $("#set-https") && ($("#set-https").onchange = (e) => act("/api/https", { on: e.target.checked }).catch(() => {}));
    $("#https-repair") && ($("#https-repair").onclick = () => act("/api/https", { on: true }).catch(() => {}));
    const el = $("#settings");
    el.querySelectorAll("[data-link]").forEach((b) => (b.onclick = () => open("url", b.dataset.link)));
    el.querySelectorAll("[data-log]").forEach((b) => (b.onclick = () => showLog(b.dataset.log)));
    el.querySelectorAll("[data-restart]").forEach((b) => (b.onclick = () => act("/api/restart", { unit: b.dataset.restart }, "Restarted nginx").catch(() => {})));
    wireVersions(el);
    drawn.update = null;
  });
  renderUpdate();
}

// dnsRow is the Settings row for <site>.<tld> addresses.
function dnsRow(d) {
  const sw = `<label class="switch"><input type="checkbox" id="set-dns" ${d.on ? "checked" : ""} aria-label=".${esc(d.tld)} addresses"><span></span></label>`;
  let desc;
  if (d.on && !d.rule) {
    desc = `<span class="error-text">The Windows rule that sends <span class="mono">*.${esc(d.tld)}</span> lookups to devstack is missing, so these addresses don't resolve.</span>`;
  } else if (d.on) {
    desc = `Sites answer at <span class="mono">name.${esc(d.tld)}</span> in every program while the servers run. Only <span class="mono">*.${esc(d.tld)}</span> lookups go to devstack; entries in the hosts file still win.`;
  } else {
    desc = `Sites answer at <span class="mono">name.localhost</span> (browsers and curl). Turn this on so <span class="mono">name.${esc(d.tld)}</span> works in PHP, Postman and terminals too. Windows asks for administrator approval once.`;
  }
  return `<div class="rowi"><div class="grow"><div class="set-title">.${esc(d.tld)} addresses</div><div class="set-desc">${desc}</div></div>
    ${d.on && !d.rule ? `<button class="btn small primary" id="dns-repair">Repair</button>` : sw}</div>`;
}

// httpsRow is the Settings row for trusted HTTPS.
function httpsRow(h) {
  const sw = `<label class="switch"><input type="checkbox" id="set-https" ${h.on ? "checked" : ""} aria-label="HTTPS"><span></span></label>`;
  const port = h.port === 443 ? "" : ` on port ${h.port}`;
  let desc;
  if (h.on && !h.trusted) {
    desc = `<span class="error-text">Windows no longer trusts the devstack certificate, so browsers will warn.</span>`;
  } else if (h.on) {
    desc = `Every site also answers at <span class="mono">https://</span>${esc(port)} with a certificate Windows trusts (Edge, Chrome, curl, PowerShell).
      PHP, Node and Python keep their own trust lists: point them at <span class="mono">${esc(h.ca)}</span> if they call a site over HTTPS.`;
  } else {
    desc = `Serve every site over <span class="mono">https://</span> too, with a certificate browsers trust. Windows asks you once to trust devstack's
      local certificate authority, which can only sign local names, never real websites. No administrator rights needed.`;
  }
  return `<div class="rowi"><div class="grow"><div class="set-title">HTTPS</div><div class="set-desc">${desc}</div></div>
    ${h.on && !h.trusted ? `<button class="btn small primary" id="https-repair">Trust again</button>` : sw}</div>`;
}

// renderUpdate draws the update row: up to date, available, downloading, or failed.
function renderUpdate() {
  const u = S.update;
  const job = S.job && S.job.title === "Updating devstack" ? S.job : null;
  draw("update", [u, progressKey(job)], () => {
    const el = $("#update-card");
    if (!el) return;
    const checked = u.checked ? new Date(u.checked).toLocaleString([], { dateStyle: "medium", timeStyle: "short" }) : "";
    let text, action = "";
    if (!u.release) {
      text = `<div class="set-title">Development build</div><div class="set-desc">Builds without a release version don't update.</div>`;
    } else if (job && !job.finished) {
      const st = job.steps[job.steps.length - 1] || { label: "Preparing", done: 0, total: 0 };
      // Steps without a size (stopping servers, starting the installer, a download
      // the server didn't size) get a sliding loader instead of a fill.
      const known = st.total > 0;
      const pct = known ? Math.min(100, Math.round((st.done / st.total) * 100)) : 0;
      const bar = known
        ? `<div class="bar-track update-bar">${barFill(st)}</div>`
        : `<div class="bar-track update-bar indeterminate"><div class="bar-fill" style="animation-delay:-${(Date.now() % 1400) / 1000}s"></div></div>`;
      const desc = known ? `${size(st.done || 0)} of ${size(st.total)} · ${pct}%`
        : st.done > 0 ? `${size(st.done)} downloaded`
        : "devstack closes and reopens itself once the installer starts.";
      text = `<div class="set-title">${esc(st.label)}…</div>${bar}<div class="set-desc num">${job.steps.length ? pctText(st, "update", desc) : desc}</div>`;
    } else if (u.version) {
      text = `<div class="set-title">devstack ${esc(u.version)} is available</div>
        ${u.notes ? `<div class="set-desc notes">${esc(u.notes)}</div>` : ""}
        ${job && job.error ? `<div class="set-desc error-text">${esc(job.error)}</div>` : ""}
        <div class="set-desc">${u.size ? `<span class="num">${size(u.size)}</span> download. ` : ""}${u.installed ? "Running servers stop for the update and start again afterwards." : "This copy isn't installed, so the download page opens."}</div>`;
      action = `<button class="btn small primary" id="update-now">${u.installed ? "Update now" : "Download"}${u.size ? ` <span class="num">(${size(u.size)})</span>` : ""}</button>`;
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
    const seg = (name, opts, labelText) => `<div class="choice" role="radiogroup" aria-label="${labelText}">${opts.map((x) =>
      `<button type="button" role="radio" data-${name}="${esc(x.value)}" aria-checked="${choice[name] === x.value}" tabindex="${choice[name] === x.value ? 0 : -1}">${esc(x.text)}</button>`).join("")}</div>`;
    const phpOpts = o.php.map((x) => ({ value: x.value, text: x.value }));
    const mysqlOpts = o.mysql.map((x) => ({ value: x.value, text: x.value + (x.label ? " " + x.label : "") }))
      .concat([{ value: "off", text: "None" }]);
    $("#setup-card").innerHTML = `
      <h1>Set up devstack</h1>
      <p class="lead">nginx, PHP and MySQL for every project folder, from one place. Pick where it lives and which versions to run.</p>
      <div class="field"><label for="root">Folder for your sites and servers</label>
        <input type="text" id="root" value="${esc(choice.root)}" spellcheck="false">
        <div class="hint" id="root-hint">Projects go in <span class="mono" id="root-www"></span>. Use a short path without spaces.</div></div>
      <div class="field"><span class="label">PHP</span>${seg("php", phpOpts, "PHP version")}</div>
      <div class="field"><span class="label">MySQL</span>${seg("mysql", mysqlOpts, "MySQL version")}</div>
      <div class="plan" id="plan"></div>
      ${o.vc_runtime ? "" : `<div class="notice warn" style="margin-bottom:12px"><div class="grow">PHP and MySQL need Microsoft's Visual C++ runtime, which isn't installed.
        <button class="link" id="vc">Get it from Microsoft</button>, then continue.</div></div>`}
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
      setupRail(null);
      $("#plan").textContent = `Downloads ${names.join(", ").replace(/, ([^,]*)$/, " and $1")} (${size(total)}) from their official sites, checks each file, then starts them.`;
      document.querySelectorAll("[data-php]").forEach((b) => { const on = b.dataset.php === choice.php; b.setAttribute("aria-checked", on); b.tabIndex = on ? 0 : -1; });
      document.querySelectorAll("[data-mysql]").forEach((b) => { const on = b.dataset.mysql === choice.mysql; b.setAttribute("aria-checked", on); b.tabIndex = on ? 0 : -1; });
    };
    document.querySelectorAll("[data-php]").forEach((b) => (b.onclick = () => { choice.php = b.dataset.php; update(); }));
    document.querySelectorAll("[data-mysql]").forEach((b) => (b.onclick = () => { choice.mysql = b.dataset.mysql; update(); }));
    // A radio group is one tab stop; the arrow keys choose within it.
    document.querySelectorAll('#setup-card [role="radiogroup"]').forEach((g) => (g.onkeydown = (e) => {
      const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
      if (!step) return;
      e.preventDefault();
      const items = [...g.querySelectorAll('[role="radio"]')];
      const next = items[(items.indexOf(document.activeElement) + step + items.length) % items.length];
      next.click();
      next.focus();
    }));
    $("#root").oninput = update;
    $("#vc") && ($("#vc").onclick = () => open("url", S.links.vc_redist));
    $("#go").onclick = () => act("/api/setup", { root: choice.root, php: choice.php, mysql: choice.mysql }).catch(() => {});
    update();
  });
}

// setupRail shows the servers setup will add as empty slots, filling each as it is set up.
function setupRail(job) {
  const o = S.setup || {};
  const php = (o.php || []).find((x) => x.value === choice.php), my = (o.mysql || []).find((x) => x.value === choice.mysql);
  const items = [["nginx", o.nginx && o.nginx.version], ["PHP", php && php.version], ["MySQL", my && my.version]].filter(([, v]) => v);
  const stateOf = (name) => {
    if (!job) return "";
    const steps = job.steps.filter((s) => s.label.toLowerCase().includes(name.toLowerCase()));
    if (steps.some((s) => s.state === "failed")) return "bad";
    if (steps.some((s) => s.state === "running")) return "busy";
    return steps.length && steps.every((s) => s.state === "done") ? "on" : "";
  };
  $("#setup-word").textContent = job ? (job.finished ? (job.error ? "Setup stopped" : "Ready") : "Setting up…") : "Not set up";
  $("#setup-slots").innerHTML = items.map(([n, v]) =>
    `<div class="slot ${stateOf(n)}"><i class="mark"></i><span class="name">${esc(n)}</span><em>${esc(short(v))}</em></div>`).join("");
}

function drawSetupProgress(job) {
  setupRail(job);
  draw("setup", ["job", progressKey(job)], () => {
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
        return `<div class="step ${esc(s.state)}">${stepIcon(s)}<span>${esc(s.label)}</span>
          <span class="amount num">${!s.total ? "" : s.state === "pending" ? size(s.total) : pctText(s, "slash", `${size(s.done || 0)} / ${size(s.total)}`)}</span>
          ${s.total ? `<div class="bar-track">${barFill(s)}</div>` : ""}</div>`;
      }).join("")}</div>
      ${job.error ? `<div class="notice error" style="white-space:pre-wrap;margin-bottom:12px">${esc(job.error)}</div>` : ""}
      ${job.error && !startFailed ? `<button class="btn primary wide" id="retry">Back</button>` : ""}
      ${job.finished && (startFailed || !job.error) ? `<button class="btn primary wide" id="done">Open dashboard</button>` : ""}`;
    $("#retry") && ($("#retry").onclick = () => api("/api/job/dismiss", {}).then(() => { drawn.setup = null; poll(true); }));
    $("#done") && ($("#done").onclick = () => api("/api/job/dismiss", {}).then(() => poll(true)));
  });
}

// ---- wiring -------------------------------------------------------------------

const menu = $("#add-menu"), addBtn = $("#add-site");
// closeMenu hides the Add site menu. Focus inside it goes back to its button,
// rather than to the page when the focused item disappears.
function closeMenu(refocus = true) {
  if (refocus && !menu.hidden && menu.contains(document.activeElement)) addBtn.focus();
  menu.hidden = true;
  addBtn.setAttribute("aria-expanded", "false");
}
addBtn.innerHTML = icon("plus") + "Add site" + icon("chevron");
$("#menu-www").innerHTML = `${icon("folder")}<span>Open the www folder<small>Every folder you put in it is a site</small></span>`;
$("#menu-folder").innerHTML = `${icon("plus")}<span>Add a folder from elsewhere…<small>Serve a project without moving it</small></span>`;
addBtn.onclick = (e) => {
  e.stopPropagation();
  const opening = menu.hidden;
  menu.hidden = !opening;
  addBtn.setAttribute("aria-expanded", String(opening));
  if (opening) $("#menu-www").focus();
};
$("#menu-www").onclick = () => { closeMenu(); open("folder", S.www); };
$("#menu-folder").onclick = pickProject;
menu.onkeydown = (e) => {
  const items = [...menu.querySelectorAll("button")], i = items.indexOf(document.activeElement);
  if (e.key === "ArrowDown") { e.preventDefault(); items[(i + 1) % items.length].focus(); }
  if (e.key === "ArrowUp") { e.preventDefault(); items[(i - 1 + items.length) % items.length].focus(); }
  if (e.key === "Home") { e.preventDefault(); items[0].focus(); }
  if (e.key === "End") { e.preventDefault(); items[items.length - 1].focus(); }
  if (e.key === "Tab") closeMenu(false); // Tab moves on, past the button
};
document.addEventListener("click", (e) => { if (!menu.hidden && !e.target.closest(".menu-wrap")) closeMenu(); });
document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape" || $("#ini").open) return;
  if (!menu.hidden) { closeMenu(); addBtn.focus(); return; }
  if (selected && page === "sites") select(selected);
});

$("#reload").innerHTML = icon("reload");
$("#reload").onclick = () => act("/api/reload", {}, "Reloaded").catch(() => {});
$("#find").oninput = (e) => { query = e.target.value; renderSites(); };
document.querySelectorAll(".nav [data-page]").forEach((b) => (b.onclick = () => go(b.dataset.page)));
$("#log-follow").onchange = () => { drawn.logsActive = false; renderLogs(); };
$("#log-folder").innerHTML = icon("folder") + "Open folder";
$("#log-folder").onclick = () => S && open("folder", S.root.replace(/[\\/]+$/, "") + "\\logs");

poll();
