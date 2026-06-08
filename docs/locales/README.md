# Documentation Locales

English (`en`) is the canonical technical documentation set. The locale directories keep the same document tree so translations can be reviewed file-by-file without changing links or release evidence paths.

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

Localization policy:

- Keep `en/` as the normative source for protocol, security, release, and SDK behavior.
- Keep every locale directory structurally identical to `en/`.
- Add new documents to `docs/` first, mirror them under every locale directory, and keep `en/` as the canonical review target.
- Translation updates should preserve commands, JSON field names, RPC method names, config keys, and code identifiers exactly.
- Security-sensitive wording should be reviewed against `en/` before release.
