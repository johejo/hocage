package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
)

const (
	// maxHTTPResponseSize is the maximum size of an HTTP response body (1MB).
	maxHTTPResponseSize = 1 << 20
	// maxHTTPRedirects is the maximum number of HTTP redirects to follow.
	maxHTTPRedirects = 10
)

// ExecAction executes the action (respond, command, or http) and writes output
// to w; a command action's stderr goes to errW.
func ExecAction(env *cel.Env, action *Action, event any, evalCtx *EvalContext, w, errW io.Writer) error {
	if action.Respond != nil {
		return execRespond(env, action.Respond, event, evalCtx, w)
	}
	if action.HTTP != nil {
		return execHTTP(env, action.HTTP, event, evalCtx, w)
	}
	return execCommand(env, action, event, evalCtx, w, errW)
}

// commandExitError carries a command action's exit code so main can exit with
// the same code (Claude Code assigns meaning to hook exit codes, e.g. 2 =
// blocking error).
type commandExitError struct {
	code int
}

func (e *commandExitError) Error() string {
	return fmt.Sprintf("command exited with code %d", e.code)
}

// resolveRespond normalises a respond value and resolves its {cel: ...} nodes.
func resolveRespond(env *cel.Env, respond any, event any, evalCtx *EvalContext) (any, error) {
	normalized, err := normalizeToJSON(respond)
	if err != nil {
		return nil, fmt.Errorf("normalize respond: %w", err)
	}
	resolved, err := ResolveValue(env, normalized, event, evalCtx)
	if err != nil {
		return nil, fmt.Errorf("resolve respond: %w", err)
	}
	return resolved, nil
}

func execRespond(env *cel.Env, respond any, event any, evalCtx *EvalContext, w io.Writer) error {
	resolved, err := resolveRespond(env, respond, event, evalCtx)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resolved)
}

// resolveCommand resolves the command action into an argv slice (the string
// form runs via sh -c, the list form runs without a shell), the extra
// environment entries from env:, and the optional stdin. Event data reaches
// the shell only through environment variables, never the command text.
func resolveCommand(env *cel.Env, action *Action, event any, evalCtx *EvalContext) (argv []string, extraEnv []string, stdin string, err error) {
	switch cmd := action.Command.(type) {
	case string:
		argv = []string{"sh", "-c", cmd}
	case []any:
		argv = make([]string, len(cmd))
		for i, elem := range cmd {
			argv[i], err = ResolveStringSlot(env, elem, event, evalCtx)
			if err != nil {
				return nil, nil, "", fmt.Errorf("resolve command[%d]: %w", i, err)
			}
		}
	default:
		return nil, nil, "", fmt.Errorf("command must be a string or a list, got %T", cmd)
	}
	for _, name := range slices.Sorted(maps.Keys(action.Env)) {
		val, err := evalExprString(env, action.Env[name], event, evalCtx)
		if err != nil {
			return nil, nil, "", fmt.Errorf("resolve env %s: %w", name, err)
		}
		extraEnv = append(extraEnv, name+"="+val)
	}
	if action.Stdin != nil {
		stdin, err = ResolveStringSlot(env, action.Stdin, event, evalCtx)
		if err != nil {
			return nil, nil, "", fmt.Errorf("resolve stdin: %w", err)
		}
	}
	return argv, extraEnv, stdin, nil
}

func execCommand(env *cel.Env, action *Action, event any, evalCtx *EvalContext, w, errW io.Writer) error {
	argv, extraEnv, stdinStr, err := resolveCommand(env, action, event, evalCtx)
	if err != nil {
		return err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	cmd.Stdout = w
	cmd.Stderr = errW
	if stdinStr != "" {
		cmd.Stdin = strings.NewReader(stdinStr)
	}
	err = cmd.Run()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() > 0 {
		return &commandExitError{code: exitErr.ExitCode()}
	}
	return err
}

// DryRunAction previews the action without executing it.
func DryRunAction(env *cel.Env, action *Action, event any, evalCtx *EvalContext, w io.Writer) error {
	if action.Respond != nil {
		resolved, err := resolveRespond(env, action.Respond, event, evalCtx)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(resolved, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal respond: %w", err)
		}
		fmt.Fprintf(w, "[dry-run] respond: %s\n", data)
		if m, ok := resolved.(map[string]any); ok {
			for _, warning := range ValidateRespondOutput(m) {
				fmt.Fprintf(w, "[dry-run] WARN: %s\n", warning)
			}
		}
		return nil
	}

	if action.HTTP != nil {
		urlStr, method, headers, err := resolveHTTP(env, action.HTTP, event, evalCtx)
		if err != nil {
			return err
		}
		if err := validateHTTPURL(urlStr); err != nil {
			return fmt.Errorf("http url validation: %w", err)
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

	argv, extraEnv, stdinStr, err := resolveCommand(env, action, event, evalCtx)
	if err != nil {
		return err
	}
	if cmdStr, ok := action.Command.(string); ok {
		fmt.Fprintf(w, "[dry-run] command: %s\n", cmdStr)
	} else {
		fmt.Fprintf(w, "[dry-run] command: %q\n", argv)
	}
	for _, kv := range extraEnv {
		fmt.Fprintf(w, "[dry-run] env: %s\n", kv)
	}
	if stdinStr != "" {
		fmt.Fprintf(w, "[dry-run] stdin: %s\n", stdinStr)
	}
	return nil
}

// validateHTTPURL checks that the URL scheme is http or https to prevent SSRF via file://, gopher://, etc.
func validateHTTPURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	switch u.Scheme {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("unsupported url scheme %q: only http and https are allowed", u.Scheme)
	}
}

func execHTTP(env *cel.Env, httpAction *HTTPAction, event any, evalCtx *EvalContext, w io.Writer) error {
	urlStr, method, headers, err := resolveHTTP(env, httpAction, event, evalCtx)
	if err != nil {
		return err
	}

	// Validate URL scheme to prevent SSRF
	if err := validateHTTPURL(urlStr); err != nil {
		return fmt.Errorf("http url validation: %w", err)
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

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	// Determine timeout
	timeout := 10 * time.Second
	if httpAction.Timeout != "" {
		timeout, err = time.ParseDuration(httpAction.Timeout)
		if err != nil {
			return fmt.Errorf("parse http timeout: %w", err)
		}
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxHTTPRedirects {
				return errors.New("too many redirects")
			}
			// Validate redirect URL scheme
			if err := validateHTTPURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect url validation: %w", err)
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	limitedBody := io.LimitReader(resp.Body, maxHTTPResponseSize)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(limitedBody)
		return fmt.Errorf("http request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if _, err := io.Copy(w, limitedBody); err != nil {
		return fmt.Errorf("read http response: %w", err)
	}
	return nil
}

// resolveHTTP resolves HTTP action fields (URL, method, headers) shared by
// execHTTP and DryRunAction's preview so the two paths can't drift.
func resolveHTTP(env *cel.Env, httpAction *HTTPAction, event any, evalCtx *EvalContext) (urlStr, method string, headers map[string]string, err error) {
	urlStr, err = ResolveStringSlot(env, httpAction.URL, event, evalCtx)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve http url: %w", err)
	}
	method = httpAction.Method
	if method == "" {
		method = "POST"
	}
	headers = make(map[string]string)
	for key, val := range httpAction.Headers {
		headers[key], err = ResolveStringSlot(env, val, event, evalCtx)
		if err != nil {
			return "", "", nil, fmt.Errorf("resolve http header %q: %w", key, err)
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
