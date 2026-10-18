package engine

import (
	"context"
	"errors"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/prompts"
)

type CodexEngine struct {
	Exec CodexExec
}

func (c CodexEngine) Run(ctx context.Context, input Input) (Proposal, error) {
	if c.Exec.Runner == nil {
		return Proposal{}, errors.New("codex engine runner is not configured")
	}
	bundle, err := prompts.Load()
	if err != nil {
		return Proposal{}, err
	}
	if input.PromptVersion != bundle.Version {
		return Proposal{}, errors.New("prompt version does not match active editorial bundle")
	}
	output, err := c.Exec.Run(ctx, input)
	if err != nil {
		return Proposal{}, err
	}
	return ParseProposal(output.Stdout)
}
