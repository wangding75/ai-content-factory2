# ACF desktop workspace

- Keep the existing ACF desktop application frame unchanged across all screens.
- Fixed top application bar with product logo/name on the left, current project selector, and right-side actions: 版本历史、提交审核、生成正文.
- Fixed breadcrumb row below the top bar.
- Three-column workspace: left chapter directory, central 正文编辑器, right contextual information panel.
- Fixed bottom status bar showing 字数、当前版本、保存状态、最后保存时间.
- Use Chinese UI text only. Do not introduce English navigation labels.
- Primary color is restrained blue. White panels, light gray borders, minimal shadows, enterprise SaaS density.
- Preserve consistent widths, spacing, tab layout, card styles, button positions and typography across screens.
- Do not redesign the global shell between screens. Only change local state or right-side content required by the workflow.
- Real content generation creates a candidate version first; never imply automatic overwrite of the current version.
- Async task states appear as a full-width status bar below breadcrumbs and above the three-column workspace.
- Error messages must be actionable and must not expose tokens, API keys, stack traces, or infrastructure addresses.