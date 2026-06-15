# Documentation Locales

English (`en`) is the canonical technical documentation set. Every other locale mirrors the same document tree as a direct translation so contributors can read the same content in their own language without changing links, release evidence paths, or audit references.

Localized pages are translations of the canonical pages: they preserve interface names exactly and keep the same document structure, but the explanatory text is rewritten in the target language instead of being treated as a separate companion note. Protocol rules, release gates, security assumptions, command semantics, config keys, RPC names, JSON fields, and code identifiers remain normative in English.

## How To Read Localized Docs

Use the localized page when you want the explanation in your language. Open the English page next to it when you need the exact command, config key, JSON field, or release requirement that remains normative.

In practice:

- **Use the locale page** for the same content in the target language.
- **Use the English source** for commands, RPC methods, config keys, package paths, and release or security decisions.
- **Treat release evidence as separate from translation quality**; a translated page can still be a bad release claim if the underlying artifact is missing.

`make docs-check` enforces the locale tree. A documentation change fails if a locale is missing a Markdown path, adds a non-canonical path, omits its locale marker, or is accidentally copied byte-for-byte from the English source. `vexod release docs-quality --docs docs --json` exposes the same release-facing quality gate as a machine-readable artifact for release candidates.

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
- Run `go run ./cmd/vexod release docs-quality --docs docs --json` when a release candidate needs a docs quality artifact.

## What a Good Locale Page Must Include

Each localized page should be a faithful translation of the canonical page and make three things obvious:

1. **Purpose:** what the canonical document is for and who should read it.
2. **Safe-use checklist:** what a reader must verify before copying commands, changing config, or making release/security decisions.
3. **Canonical link:** the exact English document that remains authoritative.

Do not turn localized pages into vague summaries. If a locale page only lists identifiers and does not explain how the document is used, improve it before release.

For reader quality, every locale page should also include:

- the operational or implementation decision the page supports
- the most important safety boundary in plain language
- the commands, config keys, RPC methods, package paths, and JSON fields that must stay untranslated
- a short warning when code exists but external release evidence is still required
- enough context for a new contributor to know which English page to open next

Localized pages may be shorter than the English canonical page, but they should not become wrappers or summaries. A reader should leave the page knowing what matters, what can be copied, what must be changed, and what must be verified.

## Adding a New Locale

1. Add the locale code to `docs/locales/manifest.json`.
2. Create a directory under `docs/locales/<locale>/`.
3. Mirror every canonical Markdown path.
4. Add a locale marker in every page: `Locale: <locale>`.
5. Preserve interface identifiers exactly.
6. Run `make docs-check`.
