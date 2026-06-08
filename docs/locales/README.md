# Documentation Locales

English (`en`) is the canonical technical documentation set. Every other locale mirrors the same document tree so contributors can review a localized page next to the exact English source without changing links, release evidence paths, or audit references.

Localized pages are written as companion documents: they explain the purpose of the canonical page, preserve interface names exactly, and give readers a checklist for safe use. Protocol rules, release gates, security assumptions, command semantics, config keys, RPC names, JSON fields, and code identifiers remain normative in English.

`make docs-check` enforces the locale tree. A documentation change fails if a locale is missing a Markdown path, adds a non-canonical path, omits its locale marker, or is accidentally copied byte-for-byte from the English source.

| Locale | Directory |
| --- | --- |
| English | `en/` |
| Korean | `ko/` |
| Chinese | `zh/` |
| Japanese | `ja/` |
| French | `fr/` |
| German | `de/` |
| Spanish | `es/` |
| Portuguese | `pt/` |
| Russian | `ru/` |
| Arabic | `ar/` |
| Hindi | `hi/` |
| Indonesian | `id/` |
| Vietnamese | `vi/` |

## Localization Policy

- Keep `en/` as the normative source for protocol, security, release, SDK, command, config, and RPC behavior.
- Keep every locale directory structurally identical to `en/`.
- Add new documents to `docs/` first, mirror them under every locale directory, and keep `en/` as the canonical review target.
- Preserve commands, JSON field names, RPC method names, config keys, package paths, and code identifiers exactly.
- Translate explanatory text, reader guidance, checklists, and failure-mode notes into the target locale.
- Review security-sensitive wording against `en/` before release.
- Run `make docs-check` before release documentation changes.

## What a Good Locale Page Must Include

Each localized page should make three things obvious:

1. **Purpose:** what the canonical document is for and who should read it.
2. **Safe-use checklist:** what a reader must verify before copying commands, changing config, or making release/security decisions.
3. **Canonical link:** the exact English document that remains authoritative.

Do not make localized pages vague shells. If a locale page only lists identifiers and does not explain how the document is used, improve it before release.

## Adding a New Locale

1. Add the locale code to `docs/locales/manifest.json`.
2. Create a directory under `docs/locales/<locale>/`.
3. Mirror every canonical Markdown path.
4. Add a locale marker in every page: `Locale: <locale>`.
5. Preserve interface identifiers exactly.
6. Run `make docs-check`.
