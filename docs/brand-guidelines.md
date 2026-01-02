---
title: "Brand Guidelines"
description: "Gasoline brand identity — colors, typography, voice, and visual language for the fire-themed developer tool."
permalink: /brand/
toc: true
toc_sticky: true
---

## Brand Essence

**Gasoline fuels the fire.** The brand leans into the cooking-with-fire metaphor — raw, essential, high-energy. Gasoline isn't a polished consumer product; it's the fuel that makes AI assistants burn brighter.

**Tagline:** Fuel for the AI Fire

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
- **Tagline:** Fuel for the AI Fire
- **Action verbs:** "captures", "feeds", "fuels", "fires up"
- **Domain voice:** "Cook with Gasoline" — as in "now we're cooking"

### Do / Don't

| Do | Don't |
|----|-------|
| "Gasoline captures browser errors" | "Our tool helps you with errors" |
| "Zero overhead. Zero dependencies." | "We've worked hard to minimize overhead" |
| "Fire up your AI assistant" | "Start using Gasoline with your AI" |
| "Fuel your debugging workflow" | "Improve your development experience" |

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
