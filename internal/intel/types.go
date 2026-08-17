// Package intel implements Cerebro Intel: autonomous OSINT-style
// reconnaissance and general information retrieval over public endpoints
// (zero API keys), routed through the DoH transport so ISP-level blocks are
// bypassed. It probes 90+ platforms for a username, aggregates person/entity
// snippets from Wikipedia and DuckDuckGo, and streams findings live.
package intel

// Status is a finding's verification state.
type Status string

const (
	StatusFound      Status = "FOUND"
	StatusUnverified Status = "UNVERIFIED"
	StatusNotFound   Status = "NOT FOUND"
)

// Finding is one platform/recon result.
type Finding struct {
	Platform string
	URL      string
	Status   Status
	Detail   string
}

// Report aggregates the whole recon run: the target, its kind, a synthesized
// bio, reference links and every finding.
type Report struct {
	Target   string
	Kind     string
	Bio      string
	Links    []string
	Findings []Finding
}

// Kind values for Report.Kind.
const (
	KindUsername = "username"
	KindPerson   = "person"
	KindTopic    = "topic"
)
