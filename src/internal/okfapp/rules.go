package okfapp

import (
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

func (s Service) store(space string) Store {
	return Store{Root: s.Root, Space: space, Bundle: filepath.Join(s.Root, "aidlc/spaces", space, "knowledge")}
}
func (s Service) Rules(space string) (string, string, error) {
	store := s.store(space)
	// A list read validates the caller's project/Space path before any Rule access.
	if err := store.ValidateRoot(); err != nil {
		return "", "", err
	}
	entry, err := okfmemory.Read(store.Bundle, "rules/entry")
	if err != nil {
		return "", "", err
	}
	if entry.String("type") != "Rule" {
		return "", "", invalid("entry must have type Rule")
	}
	entryRaw, err := okfmemory.ReadFile(store.Bundle, "rules/entry.md")
	if err != nil {
		return "", "", err
	}
	all := string(entryRaw)
	fingerprint := "rules/entry.md\x00" + all
	links := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`).FindAllStringSubmatch(entry.Body, -1)
	if len(links) == 0 {
		return "", "", invalid("Rule entry has no explicit links")
	}
	for _, link := range links {
		target := link[1]
		if strings.ContainsAny(target, "#?:\\") {
			return "", "", invalid("Rule link must name a local Concept")
		}
		if strings.HasPrefix(target, "/") {
			target = strings.TrimPrefix(target, "/")
		} else {
			target = path.Join("rules", target)
		}
		if !strings.HasSuffix(target, ".md") {
			return "", "", invalid("Rule link must name a Markdown file")
		}
		doc, err := okfmemory.Read(store.Bundle, strings.TrimSuffix(target, ".md"))
		if err != nil {
			return "", "", err
		}
		if doc.String("type") != "Rule" {
			return "", "", invalid("required Concept must have type Rule")
		}
		raw, err := okfmemory.ReadFile(store.Bundle, target)
		if err != nil {
			return "", "", err
		}
		all += "\n" + string(raw)
		fingerprint += target + "\x00" + string(raw)
	}
	if len(all) > 16*1024 {
		return "", "", invalid("required Rules exceed 16 KiB; reorganize without truncation")
	}
	return all, filestore.Hash([]byte(fingerprint)), nil
}
