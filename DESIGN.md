---
name: Technical Precision
colors:
  surface: '#f7f9fb'
  surface-dim: '#d8dadc'
  surface-bright: '#f7f9fb'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#f2f4f6'
  surface-container: '#eceef0'
  surface-container-high: '#e6e8ea'
  surface-container-highest: '#e0e3e5'
  on-surface: '#191c1e'
  on-surface-variant: '#444653'
  inverse-surface: '#2d3133'
  inverse-on-surface: '#eff1f3'
  outline: '#757684'
  outline-variant: '#c4c5d5'
  surface-tint: '#3755c3'
  primary: '#00288e'
  on-primary: '#ffffff'
  primary-container: '#1e40af'
  on-primary-container: '#a8b8ff'
  inverse-primary: '#b8c4ff'
  secondary: '#515f74'
  on-secondary: '#ffffff'
  secondary-container: '#d5e3fc'
  on-secondary-container: '#57657a'
  tertiary: '#2d3449'
  on-tertiary: '#ffffff'
  tertiary-container: '#434b60'
  on-tertiary-container: '#b4bbd5'
  error: '#ba1a1a'
  on-error: '#ffffff'
  error-container: '#ffdad6'
  on-error-container: '#93000a'
  primary-fixed: '#dde1ff'
  primary-fixed-dim: '#b8c4ff'
  on-primary-fixed: '#001453'
  on-primary-fixed-variant: '#173bab'
  secondary-fixed: '#d5e3fc'
  secondary-fixed-dim: '#b9c7df'
  on-secondary-fixed: '#0d1c2e'
  on-secondary-fixed-variant: '#3a485b'
  tertiary-fixed: '#dae2fd'
  tertiary-fixed-dim: '#bec6e0'
  on-tertiary-fixed: '#131b2e'
  on-tertiary-fixed-variant: '#3f465c'
  background: '#f7f9fb'
  on-background: '#191c1e'
  surface-variant: '#e0e3e5'
typography:
  headline-xl:
    fontFamily: Geist
    fontSize: 36px
    fontWeight: '700'
    lineHeight: 44px
    letterSpacing: -0.02em
  headline-lg:
    fontFamily: Geist
    fontSize: 28px
    fontWeight: '600'
    lineHeight: 36px
    letterSpacing: -0.01em
  headline-md:
    fontFamily: Geist
    fontSize: 20px
    fontWeight: '600'
    lineHeight: 28px
  body-lg:
    fontFamily: Geist
    fontSize: 16px
    fontWeight: '400'
    lineHeight: 24px
  body-md:
    fontFamily: Geist
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
  body-sm:
    fontFamily: Geist
    fontSize: 12px
    fontWeight: '400'
    lineHeight: 16px
  label-md:
    fontFamily: Geist
    fontSize: 12px
    fontWeight: '600'
    lineHeight: 16px
    letterSpacing: 0.05em
  mono-label:
    fontFamily: JetBrains Mono
    fontSize: 11px
    fontWeight: '500'
    lineHeight: 14px
rounded:
  sm: 0.125rem
  DEFAULT: 0.25rem
  md: 0.375rem
  lg: 0.5rem
  xl: 0.75rem
  full: 9999px
spacing:
  unit: 4px
  gutter: 16px
  margin-mobile: 16px
  margin-desktop: 32px
  container-max: 1440px
---

## Brand & Style
The design system is built on the principles of engineering excellence and surgical precision. It targets professional environments where clarity, speed of cognition, and data density are paramount. The aesthetic is rooted in a refined **Minimalism** with a **Corporate/Modern** backbone, emphasizing high-functioning utility over decorative flair.

The UI should evoke a sense of calm authority—like a high-end laboratory or a blueprint. Visual noise is treated as a defect. Every element must justify its existence through function, utilizing thin lines, generous but calculated negative space, and a disciplined monochromatic foundation.

## Colors
The palette is anchored by **#F8FAFC** (Slate 50), providing a cool, crisp off-white canvas that reduces eye strain compared to pure white. 

- **Primary (Blueprint Blue):** Used for primary actions and indicative states. It represents the "active" layer of the interface.
- **Secondary (Slate):** Used for secondary UI elements, iconography, and less prominent text.
- **Surface & Backgrounds:** Utilizes a spectrum of cool grays (Slate 100-200) to create subtle differentiation between navigation, sidebars, and content areas.
- **Accents:** High-contrast Slate 900 is reserved for typography and critical structural lines to ensure maximum legibility.

## Typography
**Geist** is the primary typeface, chosen for its geometric rigor and exceptional legibility in technical contexts. For data-heavy tables or terminal outputs, a monospaced font like **JetBrains Mono** should be used.

Typography follows a strict hierarchy. Large headers use slight negative letter-spacing to feel more compact and structured. Labels and small metadata often use uppercase with increased letter-spacing to distinguish them from body copy. Line heights are kept tight (1.4x - 1.5x) to maintain the high-density layout required for technical tools.

## Layout & Spacing
The layout utilizes a **12-column fluid grid** for content areas, but navigation and sidebars are typically fixed-width to ensure a predictable workspace. 

Spacing is based on a **4px base unit**. For technical density, use "tight" spacing:
- **8px (xs):** Component internals (icon to text).
- **12px (sm):** Smaller padding for items like list entries.
- **16px (md):** Standard gutter and card padding.
- **24px (lg):** Section spacing.

Elements are aligned with "surgical" precision, meaning horizontal and vertical lines should be consistent across the viewport. Containers should favor alignment over decorative whitespace.

## Elevation & Depth
In this design system, depth is communicated through **tonal layering and low-contrast outlines** rather than heavy shadows.

- **Level 0 (Base):** #F8FAFC (Background).
- **Level 1 (Surface):** #FFFFFF (Cards, Content Areas) with a 1px border of #E2E8F0.
- **Level 2 (Popovers):** #FFFFFF with a subtle, ultra-soft shadow: `0px 4px 12px rgba(0, 0, 0, 0.05)`.

Separation between elements is primarily achieved via 1px lines (#E2E8F0 or #CBD5E1). Shadows are used sparingly, reserved only for elements that physically sit above the interface, like dropdown menus or modals.

## Shapes
The shape language is disciplined and professional. We use **Soft (0.25rem)** roundedness for standard components like buttons and input fields. This provides a modern touch while maintaining the "sharp" technical feel. Large containers like cards or panels may use **0.5rem (rounded-lg)** to provide a clear frame for content. Pills are used exclusively for status indicators and tags.

## Components

### Buttons
- **Primary:** Blueprint Blue background, white text, 4px corner radius. No gradient.
- **Secondary:** Transparent background, 1px Slate 200 border, Slate 900 text.
- **Tertiary/Ghost:** No border or background, Blueprint Blue text.

### Inputs & Form Fields
Fields use a 1px border (#E2E8F0) and a white background. On focus, the border changes to Blueprint Blue with a subtle 2px outer glow of the same color at 10% opacity. Labels are always positioned above the field in `body-sm` weight 600.

### Cards
Cards are flat, defined by a 1px border. No shadows are applied unless the card is interactive (on hover, a subtle shadow may appear). Headers within cards should have a subtle background tint (#F1F5F9) and a bottom border to separate them from the content.

### Data Tables
Tables are a core component. Use a 12px horizontal padding for cells. Zebra striping is discouraged; use 1px horizontal dividers instead. Headers are `label-md` with a subtle Slate 50 background.

### Chips & Tags
Tags are rectangular with a 2px radius or full-pill. They use a light tinted background (e.g., light blue for "active") with a high-contrast text color. Monospace fonts are preferred for status tags (e.g., version numbers or status codes).
