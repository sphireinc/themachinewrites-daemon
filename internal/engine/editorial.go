package engine

import (
	"context"
	"errors"
	"strings"
)

type Decision struct {
	Proposal         Proposal
	ContentCandidate *Proposal
	PublicWrite      bool
}

func Decide(ctx context.Context, engine Engine, input Input, publicRepository string) (Decision, error) {
	if engine == nil {
		return Decision{}, errors.New("editorial engine is required")
	}
	if strings.TrimSpace(publicRepository) == "" {
		return Decision{}, errors.New("public repository path is required")
	}
	proposal, err := engine.Run(ctx, input)
	if err != nil {
		return Decision{}, err
	}
	switch proposal.Action {
	case ActionDefer, ActionInvestigate:
		return Decision{Proposal: proposal}, nil
	case ActionPublish:
		candidate := proposal
		return Decision{Proposal: proposal, ContentCandidate: &candidate}, nil
	default:
		return Decision{}, errors.New("unsupported editorial action")
	}
}
