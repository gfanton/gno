package doctor

import "encoding/json"

type Severity int

const (
	Info Severity = iota
	Warning
	Critical
)

func (s Severity) String() string {
	switch s {
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Critical:
		return "critical"
	default:
		return "unknown"
	}
}

func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Detail   string   `json:"detail"`
	Source   string   `json:"source"`
}

type Check struct {
	ID  string
	Run func(ctx *Context) ([]Finding, error)
}

type Correlation struct {
	ID  string
	Run func(findings []Finding, ctx *Context) []Finding
}

type CheckError struct {
	CheckID string `json:"check_id"`
	Error   string `json:"error"`
}

type Report struct {
	Target   string          `json:"target"`
	Sources  map[string]bool `json:"sources"`
	Findings []Finding       `json:"findings"`
	Errors   []CheckError    `json:"errors,omitempty"`
	Healthy  bool            `json:"healthy"`
}
