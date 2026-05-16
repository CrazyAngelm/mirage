package health

import "testing"

func TestScoreUnreachable(t *testing.T) {
	if Score(Result{Name: "x", Reachable: false}) != 0 {
		t.Fatal("unreachable score must be 0")
	}
}

func TestSelectBest(t *testing.T) {
	best, ok := SelectBest([]Result{
		{Name: "slow", Reachable: true, LatencyMS: 300},
		{Name: "fast", Reachable: true, LatencyMS: 20},
	})
	if !ok || best.Name != "fast" {
		t.Fatalf("best = %+v ok=%v", best, ok)
	}
}
