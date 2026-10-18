package prompts

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

//go:embed assets/*.md
var promptFiles embed.FS

var orderedAssets = []string{
	"identity.md",
	"editorial.md",
	"source-trust.md",
	"citation.md",
}

type Bundle struct {
	Version string
	Text    string
}

func Load() (Bundle, error) {
	assets := make(map[string][]byte, len(orderedAssets))
	var sections []string
	for _, name := range orderedAssets {
		content, err := promptFiles.ReadFile("assets/" + name)
		if err != nil {
			return Bundle{}, fmt.Errorf("read prompt asset %s: %w", name, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return Bundle{}, fmt.Errorf("prompt asset %s is empty", name)
		}
		assets[name] = append([]byte(nil), content...)
		sections = append(sections, "# BEGIN "+name+"\n"+strings.TrimSpace(string(content))+"\n# END "+name)
	}
	version, err := versionAssets(assets)
	if err != nil {
		return Bundle{}, err
	}
	return Bundle{Version: version, Text: strings.Join(sections, "\n\n") + "\n"}, nil
}

func versionAssets(assets map[string][]byte) (string, error) {
	if len(assets) == 0 {
		return "", errors.New("no prompt assets provided")
	}
	names := make([]string, 0, len(assets))
	for name := range assets {
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		if _, err := fmt.Fprintf(digest, "%s\x00%d\x00", name, len(assets[name])); err != nil {
			return "", err
		}
		if _, err := digest.Write(assets[name]); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
}
