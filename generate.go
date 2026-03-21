package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
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
		sort.Strings(names)
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
