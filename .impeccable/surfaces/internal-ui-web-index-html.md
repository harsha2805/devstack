---
version: 1
slug: "internal-ui-web-index-html"
primary_target: "internal/ui/web/index.html"
related_targets: ["internal/ui/web/app.css","internal/ui/web/app.js"]
---

# devstack dashboard (Windows app window)

Scope: the whole dashboard in `internal/ui/web/` (shell, Sites + site panel, PHP, Database, Logs, Settings; setup screen and php.ini form restyled). Mode: Operate. Shown in a WebView2 window, default 1000×700, minimum 720×540, light and dark from Windows.

Audience and job: Windows PHP developers with under 10 sites; two visits, the two-second glance (running? open a site, folder, terminal) and the fix (PHP version, extensions, php.ini, logs, database login).

Decided with the user: nginx versions live in Settings → Stack; the site panel's Recent errors shows sample lines only in development builds and "Per-site errors are coming in a later version" in releases; no stamps, postmarks or other ornament; it must read as a native Windows app.

Open: per-site error feed (backend) not built yet.

## Direction contract

THESIS: Every service and site has its place on the board; state reads from the shape of its slot (filled running, empty outline stopped, hatched failing, dashed missing), so a glance answers "is everything there?". Refuses the category default of a grey screen with one accent and status pills.

OWN-WORLD: Shadow Board in Graphite + Safety Orange: graphite sidebar #24272B owning the full height, near-white ground #F7F7F6, white slots with 1.5px rules, safety orange #FF7A1A with dark text only on pressable primary actions, green #2E7D32 running, crimson #B3261E failing. Segoe UI Variable (Display for headings), Cascadia Mono for addresses, ports and versions in tabular columns. Native Windows title bar; standard controls.

STORY: The user opens the window, sees every service slot filled and the sites list, opens a site or its folder or terminal from the row or the site panel, and when something breaks sees the hatched slot, the banner with Restart, and the affected rows.

FIRST VIEWPORT: 212px graphite rail: Running + Stop, one outlined slot per service with port, then Sites / PHP / Database / Logs, Settings at the bottom. Main: "Sites" heading, Find, orange "Add site"; failing-service banner; site slots (state mark, name, address, PHP, AI guide switch); a missing project as a dashed slot. A 300px site panel opens on the right for the selected site.

FORM: Shadow Board (workshop tool board), position 5 of 7 on the grounded list, palette changed by the user to Graphite + Safety Orange. seed:67bebd52eb283ad6

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
