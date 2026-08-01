---
name: taste
description: Build beautiful frontends with great styling, layout, color, typography, and component composition
---

# Taste — Frontend Styling Skill

You build beautiful, polished frontends. Focus on visual quality, consistency, and user experience.

## Principles

1. **Use the project's existing design system** — respect existing colors, fonts, spacing. Don't introduce new design languages.
2. **Consistent spacing** — use the project's spacing scale (Tailwind: `p-4`, `gap-3`, etc.). Never use arbitrary values.
3. **Color harmony** — use semantic color tokens (`bg-primary`, `text-muted-foreground`) not raw hex values.
4. **Typography hierarchy** — headings should be distinct from body text. Use the project's font-size scale.
5. **Whitespace** — good layouts breathe. Add padding and gaps generously.
6. **Responsive** — use Tailwind breakpoints (`sm:`, `md:`, `lg:`) for mobile-first layouts.
7. **Interactive states** — every clickable element needs `hover:`, `focus:`, `active:` styles.
8. **Loading states** — show spinners, skeletons, or placeholders during data fetch.
9. **Error states** — show clear error messages with recovery actions.
10. **Transitions** — use `transition-colors` or `transition-all` for smooth hover effects.

## Layout Patterns

- **Page layout**: Use a container with `max-w-7xl mx-auto px-4 sm:px-6 lg:px-8`
- **Sidebar layout**: Sidebar `w-64` + main content `flex-1`
- **Card grid**: `grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6`
- **Form layout**: `space-y-6` between sections, `space-y-4` between fields
- **Modal/Dialog**: Center with `fixed inset-0 flex items-center justify-center`, backdrop with `bg-black/50`
