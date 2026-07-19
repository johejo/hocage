package main

import (
	"maps"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// transcriptLib provides helpers that flatten real Claude Code transcript
// entries. Transcript JSONL lines are heterogeneous: tool calls live inside
// assistant entries as message.content[] blocks of type "tool_use", their
// results arrive later as user entries carrying "tool_result" blocks plus a
// top-level "toolUseResult", and non-message lines (mode, file-history-snapshot,
// attachment, queue-operation, ...) are interleaved throughout. These functions
// centralize that shape so CEL expressions don't have to re-derive it.
type transcriptLib struct{}

func (l *transcriptLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("tool_calls",
			cel.Overload("tool_calls_list",
				[]*cel.Type{cel.ListType(cel.DynType)},
				cel.ListType(cel.DynType),
				cel.UnaryBinding(toolCallsImpl),
			),
		),
		cel.Function("user_messages",
			cel.Overload("user_messages_list",
				[]*cel.Type{cel.ListType(cel.DynType)},
				cel.ListType(cel.StringType),
				cel.UnaryBinding(userMessagesImpl),
			),
		),
	}
}

func (l *transcriptLib) ProgramOptions() []cel.ProgramOption {
	return nil
}

var anySliceType = reflect.TypeFor[[]any]()

// toolCallsImpl flattens tool_use blocks out of assistant entries, joining each
// with its result (matched by tool_use_id) when one exists in the transcript.
func toolCallsImpl(arg ref.Val) ref.Val {
	entries, err := arg.ConvertToNative(anySliceType)
	if err != nil {
		return types.NewErr("tool_calls: expected list, got %s", arg.Type())
	}

	// Results can precede their tool_use entry when the transcript is loaded
	// with order: reverse, so collect them in a first pass.
	results := map[string]map[string]any{}
	for _, e := range entries.([]any) {
		entry, ok := e.(map[string]any)
		if !ok {
			continue
		}
		for _, block := range messageContentBlocks(entry) {
			if block["type"] != "tool_result" {
				continue
			}
			id, ok := block["tool_use_id"].(string)
			if !ok {
				continue
			}
			result := map[string]any{}
			// toolUseResult holds the structured result (e.g. stdout/stderr
			// for Bash); the tool_result block holds what the model saw.
			if tur, ok := entry["toolUseResult"].(map[string]any); ok {
				maps.Copy(result, tur)
			}
			if content, ok := block["content"]; ok {
				result["content"] = content
			}
			isErr, _ := block["is_error"].(bool)
			result["is_error"] = isErr
			results[id] = result
		}
	}

	calls := []any{}
	for _, e := range entries.([]any) {
		entry, ok := e.(map[string]any)
		if !ok || entry["type"] != "assistant" {
			continue
		}
		for _, block := range messageContentBlocks(entry) {
			if block["type"] != "tool_use" {
				continue
			}
			call := map[string]any{
				"id":    block["id"],
				"name":  block["name"],
				"input": block["input"],
			}
			if call["input"] == nil {
				call["input"] = map[string]any{}
			}
			if id, ok := block["id"].(string); ok {
				if result, ok := results[id]; ok {
					call["result"] = result
				}
			}
			calls = append(calls, call)
		}
	}
	return types.DefaultTypeAdapter.NativeToValue(calls)
}

// userMessagesImpl extracts the text of real user messages: entries with
// type "user" whose message content is a string or contains text blocks.
// Meta entries (isMeta: true) and tool_result-only entries are skipped.
func userMessagesImpl(arg ref.Val) ref.Val {
	entries, err := arg.ConvertToNative(anySliceType)
	if err != nil {
		return types.NewErr("user_messages: expected list, got %s", arg.Type())
	}

	messages := []string{}
	for _, e := range entries.([]any) {
		entry, ok := e.(map[string]any)
		if !ok || entry["type"] != "user" {
			continue
		}
		if isMeta, _ := entry["isMeta"].(bool); isMeta {
			continue
		}
		message, ok := entry["message"].(map[string]any)
		if !ok {
			continue
		}
		switch content := message["content"].(type) {
		case string:
			if content != "" {
				messages = append(messages, content)
			}
		case []any:
			text := ""
			for _, b := range content {
				block, ok := b.(map[string]any)
				if !ok || block["type"] != "text" {
					continue
				}
				if s, ok := block["text"].(string); ok && s != "" {
					if text != "" {
						text += "\n"
					}
					text += s
				}
			}
			if text != "" {
				messages = append(messages, text)
			}
		}
	}
	return types.DefaultTypeAdapter.NativeToValue(messages)
}

// messageContentBlocks returns the message.content blocks of a transcript
// entry, or nil when the entry has no block-list content (non-message lines,
// plain-string user messages, ...).
func messageContentBlocks(entry map[string]any) []map[string]any {
	message, ok := entry["message"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := message["content"].([]any)
	if !ok {
		return nil
	}
	var blocks []map[string]any
	for _, c := range content {
		if block, ok := c.(map[string]any); ok {
			blocks = append(blocks, block)
		}
	}
	return blocks
}
