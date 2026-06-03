package releasegate

type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type Document struct {
	SchemaVersion string   `json:"schema_version"`
	OK            bool     `json:"ok"`
	Version       string   `json:"version"`
	Checks        []Check  `json:"checks"`
	NextActions   []string `json:"next_actions,omitempty"`
}

type PackCheck struct {
	Name    string
	OK      bool
	Message string
}

type Pack struct {
	OK     bool
	Checks []PackCheck
}

type Evidence struct {
	Chaos                string
	KMS                  string
	Snapshot             string
	ExternalAudit        string
	BLSAudit             string
	AllowExternalPending bool
	Exists               func(string) bool
}

func Build(version string, pack Pack, evidence Evidence) Document {
	if evidence.Exists == nil {
		evidence.Exists = func(string) bool { return false }
	}
	document := Document{
		SchemaVersion: "v1",
		OK:            true,
		Version:       version,
	}
	document.addCheck("release_pack", pack.OK, "release pack must include manifest, checksums, SBOM, signature when required, and core RC evidence")
	for _, check := range pack.Checks {
		document.addCheck("pack_"+check.Name, check.OK, check.Message)
	}
	document.addFileCheck("chaos_evidence", evidence.Chaos, "chaos test evidence must exist", evidence.Exists)
	document.addFileCheck("kms_signer_evidence", evidence.KMS, "KMS/remote signer policy and double-sign guard evidence must exist", evidence.Exists)
	document.addFileCheck("snapshot_replay_evidence", evidence.Snapshot, "snapshot restore and replay consistency evidence must exist", evidence.Exists)
	document.addExternalCheck("external_security_audit", evidence.ExternalAudit, evidence.AllowExternalPending, "external audit disposition must exist before public production release", evidence.Exists)
	document.addExternalCheck("bls_adapter_audit", evidence.BLSAudit, evidence.AllowExternalPending, "audited BLS adapter and dependency audit evidence must exist when BLS is enabled", evidence.Exists)
	if !document.OK {
		document.NextActions = []string{
			"collect missing evidence artifacts and rerun release gate",
			"do not publish production release artifacts until all required checks pass",
			"use --allow-external-pending only for private release candidates, never public mainnet launch",
		}
	}
	return document
}

func (document *Document) addCheck(name string, ok bool, message string) {
	if !ok {
		document.OK = false
	}
	document.Checks = append(document.Checks, Check{Name: name, OK: ok, Message: message})
}

func (document *Document) addFileCheck(name string, path string, message string, exists func(string) bool) {
	document.addCheck(name, path != "" && exists(path), message)
}

func (document *Document) addExternalCheck(name string, path string, allowPending bool, message string, exists func(string) bool) {
	if path != "" && exists(path) {
		document.addCheck(name, true, message)
		return
	}
	document.addCheck(name, allowPending, message)
}
