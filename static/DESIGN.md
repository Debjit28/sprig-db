# Design Spec: Infrastructure Dashboard

## Design Principles
- **Vibe:** Serious infrastructure tool (Grafana, Vercel, Linear).
- **Aesthetics:** Minimalist, flat, high data density, no glassmorphism or gradients.
- **Accessibility:** High contrast, clear focus states, semantic HTML.

## CSS Variables (Tokens)

### Colors (Light Mode Default, Dark Mode via `@media (prefers-color-scheme: dark)`)
- `--bg-canvas`: Page background. (Light: `#FAFAFA`, Dark: `#0E0E11`)
- `--bg-surface`: Card/table background. (Light: `#FFFFFF`, Dark: `#16161A`)
- `--bg-surface-hover`: Row hover. (Light: `#F4F4F5`, Dark: `#1F1F24`)
- `--border-color`: 1px borders everywhere. (Light: `#E4E4E7`, Dark: `#27272A`)
- `--text-main`: Primary text. (Light: `#09090B`, Dark: `#FAFAFA`)
- `--text-muted`: Secondary text, metadata. (Light: `#71717A`, Dark: `#A1A1AA`)
- `--accent`: Primary action color. (Light: `#000000`, Dark: `#FFFFFF`)
- `--accent-fg`: Text on accent background. (Light: `#FFFFFF`, Dark: `#000000`)
- `--danger`: Destructive actions. (Light: `#DC2626`, Dark: `#EF4444`)
- `--danger-bg`: Destructive background. (Light: `#FEF2F2`, Dark: `#7F1D1D`)
- `--success`: Status indicator. (Light: `#16A34A`, Dark: `#22C55E`)

### Typography
- `--font-sans`: `Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif`
- `--font-mono`: `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace`

### Sizing & Shapes
- `--radius`: `6px` maximum.
- `--shadow`: `0 1px 2px rgba(0,0,0,0.05)` (subtle only for light mode popups/cards, otherwise flat).

## Layout & Components
- **Sidebar**: Compact, 240px wide, solid border right.
- **Tables**: Sticky header, compact padding (8px 12px), monospace data cells.
- **Buttons**: Flat, 1px border, 6px radius, no glow.
- **Icons**: Inline SVGs (Lucide style, 16px, stroke 1.5), replacing emojis.
- **Forms**: Clear 1px borders, subtle focus ring (2px solid `--accent`).

## Icons (Lucide)
Will replace all emoji with inline SVGs for:
- Database / Collections
- Settings
- Logs
- Plus / Create
- Trash / Delete
- User / Admin
- Chevron / Arrows
