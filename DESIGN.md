---
name: devstack
description: A shadow board for a local PHP stack. Every server and site has a slot, and you read its state from the slot's shape.
colors:
  rail: "#24272B"
  rail-raised: "#32363C"
  rail-hover: "#2D3035"
  rail-line: "#6E757E"
  rail-ink: "#ECEDEF"
  rail-mute: "#B4BAC2"
  rail-on: "#FFFFFF"
  ground: "#F7F7F6"
  card: "#FFFFFF"
  subtle: "#EFEFED"
  hover: "#E8E8E5"
  rule: "#E2E3E0"
  rule-strong: "#C4C6C2"
  ink: "#1C1E21"
  ink-2: "#474B51"
  ink-3: "#5C6168"
  safety-orange: "#FF7A1A"
  safety-orange-hover: "#F26A06"
  on-orange: "#1A1A1A"
  running-green: "#2E7D32"
  running-green-soft: "#E5F1E6"
  running-green-rail: "#7FD08A"
  failing-crimson: "#B3261E"
  failing-crimson-soft: "#FBEAE8"
  failing-crimson-line: "#F2A59D"
  failing-crimson-rail: "#FF9A90"
  starting-amber: "#8A5A00"
  starting-amber-soft: "#FBF0D9"
  starting-amber-rail: "#F5C35C"
  failing-slot-ink: "#FFE3DF"
  failing-slot-hatch-a: "#4D2622"
  failing-slot-hatch-b: "#3F201D"
  rail-dark: "#141517"
  rail-raised-dark: "#25282C"
  rail-hover-dark: "#1E2023"
  rail-line-dark: "#5E656D"
  rail-ink-dark: "#E6E8EB"
  rail-mute-dark: "#A8AEB6"
  ground-dark: "#1B1C1E"
  card-dark: "#232528"
  subtle-dark: "#2A2C30"
  hover-dark: "#31343A"
  rule-dark: "#34373B"
  rule-strong-dark: "#4A4E54"
  ink-dark: "#ECEDEF"
  ink-2-dark: "#C5C9CE"
  ink-3-dark: "#A7ACB3"
  safety-orange-dark: "#FF8A33"
  safety-orange-hover-dark: "#FF9D52"
  running-green-dark: "#74C47A"
  running-green-soft-dark: "#213328"
  failing-crimson-dark: "#F2877D"
  failing-crimson-soft-dark: "#3D2421"
  failing-crimson-line-dark: "#7A3A33"
  starting-amber-dark: "#E9BA5C"
  starting-amber-soft-dark: "#3A3020"
typography:
  display:
    fontFamily: "Segoe UI Variable Display, Segoe UI Variable Text, Segoe UI, system-ui, sans-serif"
    fontSize: "22px"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.01em"
  headline:
    fontFamily: "Segoe UI Variable Display, Segoe UI Variable Text, Segoe UI, system-ui, sans-serif"
    fontSize: "20px"
    fontWeight: 600
    lineHeight: 1.2
  title:
    fontFamily: "Segoe UI Variable Display, Segoe UI Variable Text, Segoe UI, system-ui, sans-serif"
    fontSize: "16px"
    fontWeight: 600
  title-small:
    fontFamily: "Segoe UI Variable Display, Segoe UI Variable Text, Segoe UI, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 600
  body:
    fontFamily: "Segoe UI Variable Text, Segoe UI, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.45
  body-small:
    fontFamily: "Segoe UI Variable Text, Segoe UI, system-ui, sans-serif"
    fontSize: "12.5px"
    fontWeight: 400
  label:
    fontFamily: "Segoe UI Variable Text, Segoe UI, system-ui, sans-serif"
    fontSize: "12px"
    fontWeight: 600
  mono:
    fontFamily: "Cascadia Mono, Consolas, ui-monospace, monospace"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.55
    fontFeature: "tnum"
rounded:
  hatch-mark: "2px"
  control: "4px"
  slot: "5px"
  menu: "6px"
  sheet: "8px"
  switch: "10px"
  mark: "50%"
spacing:
  hair: "2px"
  xxs: "4px"
  xs: "6px"
  sm: "8px"
  md: "12px"
  lg: "14px"
  xl: "16px"
  gutter: "20px"
components:
  button:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 12px"
    height: "30px"
  button-hover:
    backgroundColor: "{colors.hover}"
  button-primary:
    backgroundColor: "{colors.safety-orange}"
    textColor: "{colors.on-orange}"
    rounded: "{rounded.control}"
    padding: "0 12px"
    height: "30px"
  button-primary-hover:
    backgroundColor: "{colors.safety-orange-hover}"
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.ink-2}"
    rounded: "{rounded.control}"
    height: "30px"
  button-small:
    padding: "0 9px"
    height: "26px"
  input:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: "0 8px"
    height: "30px"
  rail-slot:
    backgroundColor: "transparent"
    textColor: "{colors.rail-ink}"
    rounded: "{rounded.slot}"
    padding: "0 10px"
    height: "34px"
  rail-slot-running:
    backgroundColor: "{colors.rail-raised}"
    textColor: "{colors.rail-ink}"
  rail-slot-failing:
    backgroundColor: "{colors.failing-slot-hatch-a}"
    textColor: "{colors.failing-slot-ink}"
  nav-item:
    backgroundColor: "transparent"
    textColor: "{colors.rail-ink}"
    rounded: "{rounded.slot}"
    padding: "0 10px"
    height: "34px"
  nav-item-current:
    backgroundColor: "{colors.rail-on}"
    textColor: "{colors.ink}"
  site-row:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    rounded: "{rounded.slot}"
    padding: "6px 8px 6px 14px"
    height: "50px"
  segmented-pressed:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.card}"
    typography: "{typography.mono}"
  alert-failing:
    backgroundColor: "{colors.failing-crimson-soft}"
    textColor: "{colors.ink}"
    rounded: "{rounded.slot}"
    padding: "9px 10px 9px 12px"
  toast:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.card}"
    rounded: "{rounded.slot}"
    padding: "9px 14px"
  sheet:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    rounded: "{rounded.sheet}"
---

# Design System: devstack

## Overview

**Creative North Star: "The Shadow Board"**

devstack is a workshop tool board. Every server and every site has a slot on the board, and you read its state from the slot's shape: a filled slot is running, an empty outline is stopped, a hatched slot is failing, a dashed outline is a missing project. A two-second glance answers "is everything there?" before any color is read. Color confirms the shape. It never carries state alone.

The window is a native Windows app shown in WebView2: the standard title bar, Segoe UI Variable, standard control sizes (30px fields and buttons), and light and dark taken from Windows. A graphite rail runs the full height in both modes, carrying the overall status, one slot per server, and the page nav. The main pane is a near-white (or near-black) ground with white cards, ruled in 1.5px lines. It is dense and quiet, sized for fewer than ten sites, with addresses, ports, versions and paths set in Cascadia Mono. Safety orange appears only where there is something to press first.

The user rejected ornament: no stamps, postmarks or decorative marks. The direction also rejects the category default of a grey screen with one accent color and status pills. This app is the visual authority for devstack. The marketing site in `website/` still uses the older warm-neutral and terracotta look, and it should be brought in line with this system rather than the other way round.

**Key Characteristics:**
- State is carried by shape (filled, outline, hatched, dashed), and color confirms it.
- A graphite rail runs the full height and stays dark in both themes.
- Safety orange is kept for primary pressable actions, with dark text on it.
- Cascadia Mono, with tabular numerals, sets every machine value.
- Surfaces are flat and ruled. Shadow appears only on things that float.
- Native Windows: Segoe UI Variable, standard control heights, theme follows the system.

## Colors

The palette is Graphite + Safety Orange: graphite neutrals, one loud action color, and three state hues kept apart from it.

### Primary
- **Safety Orange** (`safety-orange`, dark `safety-orange-dark`): fills primary buttons (Add site, Open, Start, and Restart on a failing server), the progress bar fill, the "changed" dot on a php.ini row, the update dot in the nav, and the running-step icon during setup. Text on it is always **Near-Black** (`on-orange`). Hover deepens it in light mode (`safety-orange-hover`) and lightens it in dark mode.

### Secondary
- **Running Green** (`running-green`): running state only. It is the filled mark, the "Running" state word, and the done step. On the rail it uses the lighter `running-green-rail`.
- **Failing Crimson** (`failing-crimson`): failing state, errors, and danger text. The soft fill (`failing-crimson-soft`) and line (`failing-crimson-line`) build the failing banner, and `failing-crimson-rail` is the rail version.
- **Starting Amber** (`starting-amber`): starting/busy state and warnings. The soft fill (`starting-amber-soft`) is used for warning notices and the unsaved-changes sheet footer, and `starting-amber-rail` is the rail version.

### Neutral
- **Graphite Rail** (`rail`, `rail-raised`, `rail-hover`, `rail-line`): the sidebar body, filled running slot, hover, and slot outline.
- **Rail Ink** (`rail-ink`, `rail-mute`, `rail-on`): rail text, ports and secondary rail text, and the white of the current nav item and the status word.
- **Board Ground** (`ground`): the main pane background.
- **Card White** (`card`): site rows, server cards, panel, menus, sheet, fields.
- **Subtle / Hover** (`subtle`, `hover`): quiet wells (error sample, plan summary, sheet footer) and the hover fill.
- **Rules** (`rule`, `rule-strong`): 1.5px slot outlines and dividers, plus stronger control borders and dashed empties.
- **Ink ramp** (`ink`, `ink-2`, `ink-3`): text, secondary text, and muted labels. `ink` is also the selection color.
- **Failing slot on the rail** (`failing-slot-ink`, `failing-slot-hatch-a`, `failing-slot-hatch-b`): fixed literals for the hatched failing slot. The rail is dark in both themes, so these do not switch.

Dark mode redefines the main-pane, rail and state tokens with the `-dark` values. `rail-on`, `on-orange` and the `-rail` state tints stay the same, because the rail is dark in both themes.

### Named Rules
**The Press-First Rule.** Safety orange fills only what the user should press first, or what is in progress or waiting on them (a progress fill, a changed-setting dot, an update dot). It is never used for text on the ground, for selection, for borders, or for decoration. Text on orange is always `on-orange`.

**The Ink Selects Rule.** Selection and "on" are shown in ink, not orange: the pressed segment, the selected log file, the checked switch, and the selected site's 1.5px ink outline.

**The State Hues Are Not Brand Rule.** Green, crimson and amber appear only when they report a state. They are never used for emphasis or category.

## Typography

**Display Font:** Segoe UI Variable Display (with Segoe UI Variable Text, Segoe UI, system-ui)
**Body Font:** Segoe UI Variable Text (with Segoe UI, system-ui)
**Label/Mono Font:** Cascadia Mono (with Consolas, ui-monospace)

**Character:** These are the Windows platform faces, chosen so the window reads as a native app. Cascadia Mono sets everything a machine produced, which gives the board its stencilled labels.

### Hierarchy
- **Display** (600, 22px, 1.2, -0.01em): page headings (Sites, PHP, Database, Logs, Settings) and the setup heading. The count beside it is 13px mono in `ink-3`.
- **Headline** (600, 20px, 1.2): the site panel's site name. 18px is used for the php.ini sheet title.
- **Title** (600, 16px): server card names (PHP 8.4, MySQL 8.4).
- **Title small** (600, 15px): section headings and the rail status word. 14px is used for php.ini group headings.
- **Body** (400, 13px, 1.45): default text. Site names use 13.5px at 600.
- **Body small** (400, 12.5px): descriptions, hints, panel rows, and controls in slots. Prose is capped at 68 to 70ch.
- **Label** (600, 12 to 12.5px): button text, column headers, group labels and field labels. Labels are sentence case and never uppercase or tracked.
- **Mono** (400, 12px, 1.55 in log text): addresses, paths, ports, versions, PHP selects, segmented version buttons, kv values, and logs. Counts use tabular numerals.

### Named Rules
**The Mono Means Machine Rule.** Use Cascadia Mono for any value a machine produced or will parse (an address, port, version, path, ini key, count), and Segoe for anything a person reads as language. Don't use mono for decoration.

**The Two Weights Rule.** Use only 400 and 600. Hierarchy comes from size and ink step, not from more weights.

## Layout

The window is a two-column shell. The graphite rail is 216px wide (184px under 860px), with 14/12px padding and a 6px gap: status block, server slots, then the page nav after a 10px gap, a flexible spacer, and Settings pinned to the bottom. The work pane takes the rest. Page headers are padded 16px 20px 12px, scrolling content has 20px gutters, and page bodies cap at 860px wide with an 18px vertical gap.

The Sites page is a list of slots in a five-column grid: 10px state mark, a flexible name and address column, a 134px PHP select, a 56px AI-guide switch, and 88px of row tools. Rows are 6px apart. When the 304px site panel opens on the right, the tools column drops out, because the panel does the same jobs. Under 900px the panel overlays the list instead of pushing it. Under 860px the rail narrows, the status button stacks under the status word, and slot port numbers are hidden. Server names are always kept. The minimum window is 720×540.

The spacing rhythm is small and even: 2, 4, 6, 8, 10, 12, 14, 16, 18, 20px. Controls are 26px (small), 30px (default) or 34px (rail slots, nav items, wide buttons). The Logs page is a 210px file list joined to a text pane, shown as one bordered unit.

## Elevation & Depth

The board is flat. Slots, rows, cards and banners sit on the ground with borders and fills and no shadow. Depth comes from the rail's graphite against the light pane and from the step from ground to card. Shadow is reserved for things that float above the board.

### Shadow Vocabulary
- **Float** (`box-shadow: 0 10px 28px -12px rgb(20 22 26 / .28), 0 2px 6px -2px rgb(20 22 26 / .12)`; dark `0 12px 30px -12px rgb(0 0 0 / .6), 0 2px 6px -2px rgb(0 0 0 / .4)`): the Add site menu, the php.ini sheet, and the toast.
- **Overlay edge** (`box-shadow: -14px 0 28px -18px rgb(20 22 26 / .45)`): the site panel only when it overlays the list under 900px.
- **Selection ring** (`box-shadow: 0 0 0 1px` ink): this is not elevation. It thickens the selected site's outline.

### Named Rules
**The Flat Board Rule.** Nothing that sits on the board casts a shadow. If an element doesn't float over other content, it gets a border, not a shadow.

## Shapes

The corners are slightly soft and follow a strict hierarchy: 4px for controls (buttons, fields, segmented groups, wells), 5px for slots (rail slots, nav items, site rows, server cards, banners, toast), 6px for menus, and 8px for the sheet dialog. Pills are used only for the switch track (10px). State marks are 10px circles, except the failing mark, which is a 2px-radius square. Slot outlines are 1.5px. Hairline dividers inside cards are 1px.

### Named Rules
**The Slot Shape Rule.** State is drawn as shape: a filled circle or filled slot means running, an empty 1.5px outline means stopped, a 135° hatch inside a square-cornered outline means failing, a dashed outline means missing, and a pulsing ring means starting. Every new stateful thing must use one of these five shapes, so the meaning survives forced colors and color blindness.

## Components

### Buttons
Standard Windows buttons: plain, compact, with a confident primary.
- **Shape:** gently squared (4px).
- **Default:** card fill, 1px `rule-strong` border, ink text, 600 at 12.5px, 30px tall, 12px side padding, and a 6px gap to a 14px line icon.
- **Primary:** safety-orange fill and border with `on-orange` text. Use one per region (page header, banner, panel), for the action to press first there.
- **Ghost:** transparent with `ink-2` text. It fills with `hover` and turns ink on hover. Used for row tools and icon-only buttons (a square 30 or 26px).
- **Small:** 26px tall with 9px padding. **Danger:** crimson text on the default button.
- **Disabled:** 45% opacity, no hover change.
- **Focus:** 2px solid outline in the focus color (ink in light mode, near-white in dark mode, white on the rail), offset 2px.

### Rail Slots (signature)
One slot per server in the rail: 34px tall, 1.5px `rail-line` outline, 5px corners, a state mark, the server name, and its port right-aligned in 12px mono `rail-mute`. A running slot fills with `rail-raised` and drops the outline, and its mark is solid `running-green-rail`. A stopped slot is outline only, with an outlined mark. A starting slot has a pulsing amber mark. A failing slot is a crimson-outlined slot with a 5px/5px 135° maroon hatch, light pink text, and a square outlined mark. The status block above the slots names the overall state in 15px Display ("Running", "Needs attention"), tinted with the rail state hue when the stack is busy or failing.

### Site Rows
White cards with a 1.5px `rule` border and 5px corners, at least 50px tall, laid out in the five-column grid. Hover strengthens the border. Selected rows get a 1.5px ink outline plus a 1px ink ring. Row tools sit at 55% opacity until the row is hovered or focused. A missing project is a transparent, dashed `rule-strong` row with a Remove action. The empty list is one dashed block with two actions.

### Inputs / Fields
- **Style:** card fill, 1px `rule-strong` border with an `ink-3` bottom edge (the Windows field underline), 4px corners, 30px tall, 13px text. Machine values (paths, ini values, names) use mono at 12.5px.
- **Focus:** the border turns ink and an inset 1px ink underline is added. There is no glow.
- **Error:** crimson border and crimson inset underline, with a crimson note below.

### Switch
36×20 pill with a 1px `ink-3` border and a 10px `ink-2` knob. When on, it fills with ink and the knob becomes card-colored, sliding 16px over .15s `cubic-bezier(.2,.8,.2,1)`.

### Segmented and Choice Groups
A row of buttons joined into one 4px-cornered group with 1px dividers, set in 600 mono. The pressed segment is ink with card-colored text.

### Navigation
Rail nav items are 34px, 600 at 13px, with a 16px `rail-mute` line icon and an optional mono count badge or 8px dot (crimson for a failing page, orange for an available update). The current page is a white block with ink text and an ink icon. Hover uses `rail-hover`.

### Banners and Notices
The failing banner sits above the page: `failing-crimson-soft` fill, 1px `failing-crimson-line` border, 5px corners, a hatched mark, the server name in bold crimson, the first error line in mono, and Log and Restart actions (Restart is primary). The warning variant uses the amber soft fill. Inline notices use the same fills without a border.

### Site Panel
A 304px card-colored column with a 1px left rule. The header is a 20px site name with its path in mono below. Below that: the address field with copy and a primary Open button, Folder and Terminal buttons, a PHP version segmented group, the AI guide switch, a database key-value grid (74px labels, mono values), and a subtle well for recent errors.

### Sheet, Menu, Toast
These are the only floating surfaces and the only ones that use the Float shadow. The php.ini sheet is an 8px dialog (at most 780×760) with a 45% graphite backdrop, a search bar on the ground color, ruled rows, and a subtle footer that turns amber when there are unsaved changes. The menu is 6px with 4px padding and two-line items. The toast is an ink block with card-colored text (crimson with white text for errors), centered on the work pane, fading over .18s.

### Motion
The motion is small and functional: the busy mark pulses (`breathe`, .8s ease-in-out, alternating opacity .3 to 1), the switch slides over .15s, the progress bar moves over .2s, an indeterminate bar slides over 1.4s, and the toast fades over .18s. `prefers-reduced-motion` turns all of it off, and the indeterminate bar becomes a static 60% fill.

## Do's and Don'ts

### Do:
- **Do** make state readable from shape first (filled, outline, 135° hatch, dashed, pulse) and add the state hue second, so it still works in High Contrast.
- **Do** keep safety orange for the one primary action per region and for progress or pending marks, always with `on-orange` text.
- **Do** show selection and "on" in ink (pressed segment, checked switch, selected row outline, selected log file).
- **Do** set addresses, ports, versions, paths, ini keys and counts in Cascadia Mono with tabular numerals.
- **Do** use 1.5px outlines for slots and rows, 1px for control borders and dividers, and the 4/5/6/8px corner hierarchy.
- **Do** keep the rail graphite in both themes, and use the `-rail` state tints on it.
- **Do** draw icons as inline 16-unit line SVGs (1.2 to 1.6 stroke, `currentColor`) at 14px, or 16px in the nav, and give icon-only buttons an aria-label.
- **Do** keep focus visible with the 2px outline, offset 2px.

### Don't:
- **Don't** add stamps, postmarks, badges or other ornament to the board.
- **Don't** report state with colored pills or with color alone. Use a mark shape plus a plain state word.
- **Don't** use orange for selection, links, headings, borders or emphasis, and never put white text on it.
- **Don't** give flat board elements (slots, rows, cards, banners) a shadow. Shadow means the element floats.
- **Don't** add font weights beyond 400 and 600, or uppercase and letter-spaced labels.
- **Don't** use Unicode glyphs or emoji as icons.
- **Don't** carry the Segoe platform voice to surfaces outside the Windows app. It is there because this window must read as native Windows.
