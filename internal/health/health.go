package health

import "fmt"

const (
	StatusUnknown   = "unknown"
	StatusChecking  = "checking"
	StatusReachable = "reachable"
	StatusFailed    = "failed"
)

type TransportStatus struct {
	Name      string
	Status    string // unknown, checking, reachable, failed
	LatencyMS int
	Score     int
	Message   string
}

type Result struct {
	Name           string
	Reachable      bool
	LatencyMS      int
	LossPercent    int
	RecentFailures int
}

func Score(r Result) int {
	if !r.Reachable {
		return 0
	}
	score := 100
	score -= r.LatencyMS / 20
	score -= r.LossPercent * 2
	score -= r.RecentFailures * 10
	if score < 1 {
		return 1
	}
	return score
}

func SelectBest(results []Result) (Result, bool) {
	var best Result
	bestScore := 0
	for _, r := range results {
		s := Score(r)
		if s > bestScore {
			best = r
			bestScore = s
		}
	}
	return best, bestScore > 0
}

func (t TransportStatus) Label() string {
	switch t.Status {
	case StatusChecking:
		return fmt.Sprintf("%s: checking...", t.Name)
	case StatusReachable:
		return fmt.Sprintf("%s: %dms (score %d)", t.Name, t.LatencyMS, t.Score)
	case StatusFailed:
		return fmt.Sprintf("%s: failed (%s)", t.Name, t.Message)
	default:
		return fmt.Sprintf("%s: unknown", t.Name)
	}
}
