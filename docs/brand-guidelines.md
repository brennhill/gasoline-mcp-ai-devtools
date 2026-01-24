---
title: "Brand Guidelines"
description: "Gasoline brand identity — colors, typography, voice, and visual language for the fire-themed developer tool."
permalink: /brand/
toc: true
toc_sticky: true
---

## Brand Essence

**Gasoline fuels the fire.** The brand leans into the cooking-with-fire metaphor — raw, essential, high-energy. Gasoline isn't a polished consumer product; it's the fuel that makes autonomous coding agents burn brighter.

**Tagline:** Browser observability for AI coding agents

**Domain:** cookwithgasoline.com — a nod to the idiom "now we're cooking with gas" (meaning things are going well, working efficiently).

## Color Palette

### Primary (Fire Spectrum)

| Swatch | Name | Hex | Usage |
|--------|------|-----|-------|
| | Flame Outer | `#E67E22` | Primary accent, buttons, active states, borders |
| | Flame Mid | `#F39C12` | Links, hover states, inline code |
| | Flame Inner | `#F5C842` | Highlights, link hover, emphasis |

### Backgrounds (Dark Charcoal)

| Swatch | Name | Hex | Usage |
|--------|------|-----|-------|
| | Base | `#1A1F2E` | Page background |
| | Elevated | `#232A3B` | Cards, code blocks, TOC |
| | Surface | `#2A3244` | Table headers, nav highlights |
| | Code | `#151A26` | Code block backgrounds |

### Text & Borders

| Swatch | Name | Hex | Usage |
|--------|------|-----|-------|
| | Heading | `#FFFFFF` | H1-H6, bold text |
| | Body | `#E8E8E8` | Paragraph text |
| | Muted | `#9BA3B0` | Subtitles, captions, secondary text |
| | Border | `#3A4556` | Dividers, card borders, table lines |

### Icon Gradient (Image Generation & Marketing)

The flame icon uses a richer multi-stop gradient for visual impact at larger sizes. Use these values for image generation, marketing assets, social banners, and any context where the flame needs to feel vibrant and dimensional.

| Swatch | Name | Hex | Usage |
|--------|------|-----|-------|
| | Gradient Start | `#f97316` | Outer flame base (orange-500) |
| | Gradient Mid | `#fb923c` | Outer flame body (orange-400) |
| | Gradient End | `#fbbf24` | Flame tip / inner start (amber-400) |
| | Inner Tip | `#fef3c7` | Hottest center point (amber-100) |
| | Icon Background | `#1a1a1a` | Dark circle behind flame in icon |

**Gradient direction:** Bottom-to-top (`#f97316` at base → `#fef3c7` at tip).

**When to use which palette:**
- **Primary (Fire Spectrum)** — UI elements, text accents, borders, website components
- **Icon Gradient** — Image generation, marketing materials, hero graphics, social media, app store assets

### Logo Circle

| Swatch | Name | Hex | Usage |
|--------|------|-----|-------|
| | Circle | `#2C3E50` | Dark blue-grey backdrop behind flame |

## Logo

The logo is a stylized flame (teardrop shape) with a gradient from Flame Outer at the edges to Flame Inner at the center, set against a Circle-colored backdrop.

### Usage Rules

- Always use on dark backgrounds (`#1A1F2E` or darker)
- Minimum clear space: 1x the flame width on all sides
- Never rotate, stretch, or add effects
- The flame gradient is top-to-center (outer → inner)

### Favicon

The flame icon alone, without the circle backdrop, on transparent background.

## Mascot: Sparky

Sparky is the Gasoline brand character — a friendly anthropomorphic salamander with a fire-colored gradient. He is **not** a replacement for the logo flame; he is a separate illustrated asset used where there's room for expression and storytelling.

### Sparky vs. Logo Flame

| | Logo Flame | Sparky |
|---|-----------|--------|
| **What** | Abstract flame icon | Anthropomorphic salamander character |
| **Shape** | Sharp teardrop, symmetrical | Compact upright body, slightly chubby proportions |
| **Features** | Abstract, no face | Large expressive dark maroon eyes, short stubby limbs, accent markings |
| **Style** | Flat color or simple gradient | Modern vector with thick defined outlines |
| **Behavior** | Static, precise | Leans, stretches, reacts, emotes, holds props |
| **Usage** | Icons, nav, favicons, inline bullets | Illustrations, hero sections, empty states, step guides, error pages |
| **Minimum size** | 16px | 48px (needs room for expression) |

### Design Principles

- **Salamander, not flame** — Sparky is a character with a body, limbs, and personality. He's fire-*colored*, not fire-shaped.
- **Thick outlines** — Clean vector style with bold, defined outlines. This is what makes him read well at illustration sizes.
- **Expressive through pose** — Personality comes from body language. Arms crossed = confident. Arms up = excited. Leaning = casual. No mouth unless the pose demands it (e.g., chef's kiss).
- **Same gradient** — Skin uses the Icon Gradient palette (`#f97316` → `#fb923c` → `#fbbf24` → `#fef3c7`). Deep orange on extremities/back, golden yellow on belly/tail tip.
- **Dark eyes** — Large, expressive, dark maroon. The primary point of emotional connection.
- **Accent markings** — Warm red-orange spots on cheeks and tail for visual interest.
- **Transparent backgrounds** — Rendered on transparent for maximum reusability. Backgrounds added in post-production.
- **Never replaces the logo** — The logo flame stays clean and abstract. Sparky lives beside it, not instead of it.

### When to Use Sparky

- Hero sections and marketing illustrations
- Step-by-step guides (one pose per step)
- Empty states and 404 pages
- Section headers with sufficient whitespace
- Social media and blog graphics
- Onboarding flows and success confirmations

### When NOT to Use Sparky

- Anywhere the plain logo flame currently appears (nav, favicon, inline icons)
- Dense UI with no breathing room
- Formal/enterprise contexts where the abstract logo is more appropriate
- At sizes below 48px — use the logo flame instead

## Typography

### Headings
- **Font:** System font stack (`-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif`)
- **Weight:** 700 (bold)
- **Color:** `#FFFFFF`

### Body
- **Font:** Same system stack
- **Weight:** 400 (regular)
- **Color:** `#E8E8E8`
- **Line height:** 1.6

### Code
- **Font:** `"SF Mono", "Fira Code", "JetBrains Mono", monospace`
- **Background:** `#151A26`
- **Inline code color:** `#F39C12` (Flame Mid)

## Voice & Tone

### Personality

| Trait | Expression |
|-------|-----------|
| **Direct** | No fluff. Say what it does, how to use it. |
| **Technical** | Developers are the audience. Use precise terms. |
| **Confident** | "Zero dependencies" not "we try to minimize dependencies" |
| **Fire metaphors** | Subtle, not forced. "Fuel", "ignite", "fire up" — not every sentence. |

### Naming Conventions

- **Product:** Gasoline (capital G, no "the")
- **Tagline:** Browser observability for AI coding agents
- **Action verbs:** "streams", "captures", "fuels", "fires up"
- **Domain voice:** "Cook with Gasoline" — as in "now we're cooking"

### Do / Don't

| Do | Don't |
|----|-------|
| "Gasoline streams browser errors to your agent" | "Our tool helps you with errors" |
| "Zero overhead. Zero dependencies." | "We've worked hard to minimize overhead" |
| "Fire up your coding agent" | "Start using Gasoline with your AI" |
| "Fuel your autonomous workflow" | "Improve your development experience" |

## UI Elements

### Buttons

- **Primary:** Flame Outer background, dark text (`#1A1F2E`), 24px border-radius (pill shape)
- **Hover:** Flame Mid background
- **Border buttons:** 1px Flame Outer border, transparent fill, Flame Outer text

### Cards / Containers

- Background: Elevated (`#232A3B`)
- Border: 1px solid Border (`#3A4556`)
- Border-radius: 6px
- No box shadows (flat design, depth via color)

### Flow Diagrams

- Use pill-shaped containers with subtle grey borders
- Connect with thin arrows (`→`)
- The final element (AI) gets the Flame Outer fill — it's what Gasoline fuels

```
[ Browser ] → [ Extension ] → [ Local Server ] → [ AI ]
  (grey)        (grey)           (grey)          (orange)
```

## Imagery

### Hero Backgrounds

- Dark gradient (`#1A1F2E` to `#151A26`)
- Optional: subtle ember/spark particles (never overwhelming)
- No stock photography

### Screenshots

- Always on dark backgrounds
- Browser chrome in dark theme
- Captured at 2x resolution

### Icons

- Minimal line style
- Flame Outer color on dark backgrounds
- Consistent 24px base size

## Metaphor Guide

The "cooking with gasoline" metaphor extends naturally:

| Concept | Metaphor |
|---------|----------|
| Browser data | Ingredients / raw fuel |
| Extension capturing | Collecting fuel |
| Server processing | The engine / burner |
| AI using the data | Cooking / the fire |
| MCP protocol | The pipeline / fuel line |
| Errors and logs | Sparks, signals |

Use sparingly. One fire metaphor per page is enough. The product name does most of the work.
