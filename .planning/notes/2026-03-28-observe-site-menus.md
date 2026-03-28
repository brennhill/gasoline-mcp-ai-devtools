---
date: "2026-03-28 14:45"
promoted: false
---

observe(what: "site_menus") — smart menu discovery using 3-layer heuristic: (1) semantic elements first (nav/header/footer/ARIA), (2) axis alignment for div-soup menus (shared X or Y = menu group), (3) border proximity to confirm grouping vs coincidence. Returns {main, sidebar, footer, other}. Also update explore_page to return menus separately from page links with no overlap.
