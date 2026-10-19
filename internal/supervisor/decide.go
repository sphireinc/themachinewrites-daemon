package supervisor

type DecisionInputs struct {
	NewCandidates     int
	DueFollowups      int
	IdleReflectionDue bool
}
type DecisionGate struct{}

func (DecisionGate) ShouldInvoke(input DecisionInputs) bool {
	return input.NewCandidates > 0 || input.DueFollowups > 0 || input.IdleReflectionDue
}
