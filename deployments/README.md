# Vexo Deployment Assets

This directory contains operator-owned deployment templates for running, observing,
and release-validating Vexo networks.

These files are intentionally templates, not fake launch evidence. They help teams
bootstrap repeatable environments, but public release evidence must still come from
real binaries, real machines, real validator keys, real BLS/VRF audit inputs, and
recorded long-run/chaos outputs.

## Layout

- `docker/`: Docker Compose init/run templates for single-host and multi-host drills.
- `helm/vexo-consensus/`: Kubernetes chart for validator StatefulSet deployment.
- `monitoring/`: Prometheus, Alertmanager, and Grafana starter assets.
- `terraform/aws-minimal/`: Small AWS infrastructure template for isolated validator hosts.

## Recommended Flow

1. Build or pull a signed `vexod` image.
2. Generate validator homes with `vexod init validator` or the Docker init compose.
3. Store each node's split config files and encrypted key documents in a controlled secret path.
4. Deploy validators with Docker, Helm, or your own supervisor.
5. Scrape `/metrics/text`, poll `/v1/diagnostics`, and archive logs plus evidence JSON.
6. Run `make release-candidate` only after external evidence inputs are ready.

The GitHub `release-candidate` workflow intentionally targets a self-hosted
runner labeled `vexo-rc`, because real long-run/chaos validation can exceed
hosted-runner time limits and needs operator-controlled networking, cgo/BLS
toolchains, external fixture corpora, and artifact retention.

For Helm, create one Kubernetes Secret for split config files
(`config.json`, `module_config.json`, `network_config.json`,
`consensus_config.json`, `mempool_config.json`, `log_config.json`,
`genesis.json`) and another Secret for encrypted key documents
(`validator.key.json`, `node.key.json`). Do not put plaintext validator
private keys in `values.yaml`.

## Safety Boundaries

- Do not publish these templates as proof of production readiness.
- Do not embed validator private keys, API tokens, or remote signer tokens in Helm values or Terraform state.
- Do not expose RPC admin routes to the public internet without route-scoped auth and audit logging.
- Do not claim BLS/VRF production readiness unless the release evidence includes adapter audit metadata and key/proof-of-possession checks.
