---
name: pinpoint webui contributor
description: >-
  Provides instructions and coding guidelines on how to contribute to the Pinpoint Web UI Angular
  project in the /pinpoint/webui directory. Any code change in this directory should adhere to this skill.
---

# Skia Buildbot Pinpoint Web UI Contributor Guidelines

Use this skill when making changes under `buildbot/pinpoint/webui`.

## Guidelines

1. **Separate Component Templates**: Always place component HTML templates in a standalone `.html` file. Do not inline HTML strings within the component's TypeScript (`.ts`) file using the `template` property.
2. **Separate Stylesheets**: If a component requires custom styling, always create a dedicated `.css` (or `.scss`) file. Never inline style declarations inside the component's TypeScript file using the `styles` property.
3. **Minimize Inline Styles**: Avoid using the inline HTML `style` attribute for complex styling. Simple, single-purpose styling adjustments that are unique to the element (e.g., adding a `margin-top` to a spacer or divider) are acceptable inline. However, if an element requires multiple style declarations or reusable rules, define them as a class in the component's stylesheet instead.
4. **Utilize Angular Material Theming**: Leverage the official Angular Material system, color palettes, utility variables (`var(--mat-...)`), and standard paddings/densities. Avoid introducing custom hardcoded colors or bespoke layout metrics. This ensures seamless light and dark mode support.
5. **Prefer Standard Angular Material Components**: Before building any custom UI element, check if a suitable component exists in the Angular Material library (e.g., `mat-table`, `mat-select`, `mat-dialog`). Reusing standard elements provides built-in accessibility (a11y) and keyboard navigation.
6. **Adhere to the Single Responsibility Principle**: Design each component to serve a single, clear purpose. If a component handles too many unrelated UI elements, complex operations, or distinct workflows, split it into smaller, specialized sub-components.
7. **Extract Complex Business Logic to Services**: Keep Angular components lean by delegating complex business logic, state management, and API data-fetching operations to dedicated Angular services. Components should primarily focus on managing the view presentation and user interactions.
8. **Promote Reusable Components**: Move common, highly reusable UI elements, pipes, and directives into shared or common directories. This prevents code duplication and enables seamless sharing across different feature components.
9. **Prefer Enums Over String Literals**: Avoid hardcoding raw string literals for states, types, categories, or configuration values throughout the codebase. Define and use strongly-typed TypeScript `enum`s or union types to improve readability, auto-completion, and refactoring safety.
