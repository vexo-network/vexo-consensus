# AWS Minimal Validator Hosts

This Terraform example creates a small set of isolated EC2 instances for Vexo
validator drills. It intentionally does not generate validator keys, BLS keys,
remote signer tokens, genesis files, or release evidence.

Use it as an infrastructure skeleton only:

1. Build and sign a `vexod` release artifact.
2. Generate validator homes on a trusted machine.
3. Copy each validator home to a separate host with your secret-management process.
4. Start `vexod` with systemd, Docker, or Kubernetes.
5. Collect metrics, logs, pprof samples, long-run evidence, chaos evidence, signer evidence, and snapshot/replay evidence.

Do not store validator keys in Terraform variables, state, user data, or cloud-init.

