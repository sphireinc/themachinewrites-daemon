package observations

import (
	"encoding/json"
	"errors"
	"strings"
)

func EncodePromptEnvelope(instructions string, bundle Bundle) (string, error) {
	if strings.TrimSpace(instructions) == "" {
		return "", errors.New("trusted instructions are required")
	}
	if bundle.SourceMaterialPolicy != UntrustedSourceMaterialPolicy {
		return "", errors.New("source bundle policy must mark content as untrusted data")
	}
	for _, item := range bundle.Items {
		if item.Trust != "untrusted_external_data" {
			return "", errors.New("every bundle item must be marked untrusted_external_data")
		}
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return "", err
	}
	return "TRUSTED_INSTRUCTIONS:\n" + strings.TrimSpace(instructions) +
		"\n\nUNTRUSTED_SOURCE_DATA:\n" + string(encoded) + "\n", nil
}
