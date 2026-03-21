package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
)

// ExecAction executes the action (respond, command, or http) and writes output to w.
func ExecAction(env *cel.Env, action *Action, event any, evalCtx *EvalContext, w io.Writer) error {
	if action.Respond != nil {
		return execRespond(env, action.Respond, event, evalCtx, w)
	}
	if action.HTTP != nil {
		return execHTTP(env, action.HTTP, event, evalCtx, w)
	}
	return execCommand(env, action, event, evalCtx, w)
}

// interpolateRespond normalises and interpolates a respond value, returning the result.
func interpolateRespond(env *cel.Env, respond any, event any, evalCtx *EvalContext) (any, error) {
	normalized, err := normalizeToJSON(respond)
	if err != nil {
		return nil, fmt.Errorf("normalize respond: %w", err)
	}
	interpolated, err := InterpolateValue(env, normalized, event, evalCtx)
	if err != nil {
		return nil, fmt.Errorf("interpolate respond: %w", err)
	}
	return interpolated, nil
}

func execRespond(env *cel.Env, respond any, event any, evalCtx *EvalContext, w io.Writer) error {
	interpolated, err := interpolateRespond(env, respond, event, evalCtx)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(interpolated)
}

// interpolateCommand interpolates command and optional stdin strings.
func interpolateCommand(env *cel.Env, action *Action, event any, evalCtx *EvalContext) (cmd string, stdin string, err error) {
	cmd, err = Interpolate(env, action.Command, event, evalCtx)
	if err != nil {
		return "", "", fmt.Errorf("interpolate command: %w", err)
	}
	if action.Stdin != "" {
		stdin, err = Interpolate(env, action.Stdin, event, evalCtx)
		if err != nil {
			return "", "", fmt.Errorf("interpolate stdin: %w", err)
		}
	}
	return cmd, stdin, nil
}

func execCommand(env *cel.Env, action *Action, event any, evalCtx *EvalContext, w io.Writer) error {
	interpolated, stdinStr, err := interpolateCommand(env, action, event, evalCtx)
	if err != nil {
		return err
	}
	cmd := exec.Command("sh", "-c", interpolated)
	cmd.Stdout = w
	cmd.Stderr = w
	if stdinStr != "" {
		cmd.Stdin = strings.NewReader(stdinStr)
	}
	return cmd.Run()
}

// DryRunAction previews the action without executing it.
func DryRunAction(env *cel.Env, action *Action, eventName string, event any, evalCtx *EvalContext, w io.Writer) error {
	if action.Respond != nil {
		interpolated, err := interpolateRespond(env, action.Respond, event, evalCtx)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(interpolated, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal respond: %w", err)
		}
		fmt.Fprintf(w, "[dry-run] respond: %s\n", data)
		if m, ok := interpolated.(map[string]any); ok {
			for _, warning := range ValidateRespondOutput(eventName, m) {
				fmt.Fprintf(w, "[dry-run] WARN: %s\n", warning)
			}
		}
		return nil
	}

	if action.HTTP != nil {
		urlStr, method, headers, err := interpolateHTTP(env, action.HTTP, event, evalCtx)
		if err != nil {
			return err
		}
		timeout := action.HTTP.Timeout
		if timeout == "" {
			timeout = "10s"
		}
		fmt.Fprintf(w, "[dry-run] http: %s %s (timeout: %s)\n", method, urlStr, timeout)
		headerKeys := make([]string, 0, len(headers))
		for key := range headers {
			headerKeys = append(headerKeys, key)
		}
		sort.Strings(headerKeys)
		for _, key := range headerKeys {
			fmt.Fprintf(w, "[dry-run] header: %s: %s\n", key, headers[key])
		}
		return nil
	}

	interpolated, stdinStr, err := interpolateCommand(env, action, event, evalCtx)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "[dry-run] command: %s\n", interpolated)
	if stdinStr != "" {
		fmt.Fprintf(w, "[dry-run] stdin: %s\n", stdinStr)
	}
	return nil
}

func execHTTP(env *cel.Env, httpAction *HTTPAction, event any, evalCtx *EvalContext, w io.Writer) error {
	// Interpolate URL
	urlStr, err := Interpolate(env, httpAction.URL, event, evalCtx)
	if err != nil {
		return fmt.Errorf("interpolate http url: %w", err)
	}

	// Determine method
	method := httpAction.Method
	if method == "" {
		method = "POST"
	}

	// Build request body from event JSON
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event for http: %w", err)
	}

	req, err := http.NewRequest(method, urlStr, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Interpolate and set headers
	for key, val := range httpAction.Headers {
		interpolatedVal, err := Interpolate(env, val, event, evalCtx)
		if err != nil {
			return fmt.Errorf("interpolate http header %q: %w", key, err)
		}
		req.Header.Set(key, interpolatedVal)
	}

	// Determine timeout
	timeout := 10 * time.Second
	if httpAction.Timeout != "" {
		timeout, err = time.ParseDuration(httpAction.Timeout)
		if err != nil {
			return fmt.Errorf("parse http timeout: %w", err)
		}
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("read http response: %w", err)
	}
	return nil
}

// interpolateHTTP interpolates HTTP action fields for dry-run preview.
func interpolateHTTP(env *cel.Env, httpAction *HTTPAction, event any, evalCtx *EvalContext) (urlStr, method string, headers map[string]string, err error) {
	urlStr, err = Interpolate(env, httpAction.URL, event, evalCtx)
	if err != nil {
		return "", "", nil, fmt.Errorf("interpolate http url: %w", err)
	}
	method = httpAction.Method
	if method == "" {
		method = "POST"
	}
	headers = make(map[string]string)
	for key, val := range httpAction.Headers {
		headers[key], err = Interpolate(env, val, event, evalCtx)
		if err != nil {
			return "", "", nil, fmt.Errorf("interpolate http header %q: %w", key, err)
		}
	}
	return urlStr, method, headers, nil
}

// normalizeToJSON round-trips through JSON to get consistent types (e.g. map[string]any).
func normalizeToJSON(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
