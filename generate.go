package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type hooksSettings struct {
	Hooks map[string][]hookMatcher `json:"hooks"`
}

type hookMatcher struct {
	Matcher string       `json:"matcher,omitempty"`
	Hooks   []hookEntry  `json:"hooks"`
}

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// Generate writes the Claude Code settings.json hooks section to w.
func Generate(cfg *Config, agcelCmd string, w io.Writer) error {
	// Group hooks by (event_name, matcher)
	type key struct {
		eventName string
		matcher   string
	}
	groups := make(map[key][]string)   // key -> list of hook names
	order := make([]key, 0)
	seen := make(map[key]bool)

	for name, hook := range cfg.Hooks {
		k := key{hook.EventName, hook.Matcher}
		if !seen[k] {
			order = append(order, k)
			seen[k] = true
		}
		groups[k] = append(groups[k], name)
	}

	settings := hooksSettings{
		Hooks: make(map[string][]hookMatcher),
	}

	for _, k := range order {
		names := groups[k]
		// Sort by priority (ascending), then by name for stable ordering
		sort.Slice(names, func(i, j int) bool {
			pi := cfg.Hooks[names[i]].Priority
			pj := cfg.Hooks[names[j]].Priority
			if pi != pj {
				return pi < pj
			}
			return names[i] < names[j]
		})
		var entries []hookEntry
		for _, name := range names {
			entries = append(entries, hookEntry{
				Type:    "command",
				Command: fmt.Sprintf("%s hooks run %s", agcelCmd, name),
			})
		}
		matcher := hookMatcher{
			Matcher: k.matcher,
			Hooks:   entries,
		}
		settings.Hooks[k.eventName] = append(settings.Hooks[k.eventName], matcher)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(settings)
}

// GenerateMerged generates hooks and merges them into existingJSON, preserving all other keys.
func GenerateMerged(cfg *Config, agcelCmd string, existingJSON []byte, w io.Writer) error {
	// Generate hooks into a buffer
	var buf strings.Builder
	if err := Generate(cfg, agcelCmd, &buf); err != nil {
		return err
	}

	// Parse generated hooks
	var generated map[string]json.RawMessage
	if err := json.Unmarshal([]byte(buf.String()), &generated); err != nil {
		return fmt.Errorf("unmarshal generated hooks: %w", err)
	}

	// Parse existing JSON
	var existing map[string]json.RawMessage
	if err := json.Unmarshal(existingJSON, &existing); err != nil {
		return fmt.Errorf("parse existing JSON: %w", err)
	}

	// Merge: overwrite only the hooks key
	existing["hooks"] = generated["hooks"]

	// json.MarshalIndent sorts map keys alphabetically
	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal merged JSON: %w", err)
	}
	out = append(out, '\n')

	_, err = w.Write(out)
	return err
}
