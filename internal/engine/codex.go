package engine

import (
	"context"
	"errors"
)

type CodexEngine struct {
	Exec CodexExec
}

func (c CodexEngine) Run(ctx context.Context, input Input) (Proposal, error) {
	if c.Exec.Runner == nil {
		return Proposal{}, errors.New("codex engine runner is not configured")
	}
	output, err := c.Exec.Run(ctx, input)
	if err != nil {
		return Proposal{}, err
	}
	return ParseProposal(output.Stdout)
}
