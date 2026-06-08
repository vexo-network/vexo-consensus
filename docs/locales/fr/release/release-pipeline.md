# Release Pipeline

> Locale: fr · Français
> Ce document est un guide traduit à partir de la documentation anglaise canonique. Les décisions de protocole, de sécurité et de publication restent normatives en anglais.

## Objectif

Ce document couvre le pipeline de release avec binaires signés, checksums et SBOM. Les commandes, champs JSON, noms RPC, config key et identifiants de code utilisés par l’implémentation et l’exploitation restent en anglais pour préserver la compatibilité.

## Périmètre essentiel

- Vérifiez les points suivants lors de la lecture. Les commandes, champs JSON, méthodes RPC, clés de configuration et identifiants de code restent en anglais pour préserver la compatibilité.
- Pour les formulations normatives détaillées, utilisez le document anglais.
- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/fr/release/release-pipeline.md`

## Identifiants à conserver

- `release gate`
- `ok`
- `status`
- `--allow-external-pending`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `CGO_ENABLED=0`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `make network-e2e`
- `RC_DRY_RUN=1`

## Sections anglaises

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Launch Runbook

## Notes opérationnelles

- `MUST`, `SHOULD`, `MAY`, les exemples de commande, les exemples JSON et les noms RPC conservent l’orthographe anglaise.
- Après modification de cette traduction, exécutez `make docs-check`.
- Si cette page contredit la source anglaise, utilisez la source anglaise et mettez à jour ce fichier locale dans le même changement.

## Source canonique

- [English canonical document](../../en/release/release-pipeline.md)
