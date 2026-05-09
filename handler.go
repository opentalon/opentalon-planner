package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/opentalon/opentalon/pkg/plugin"
)

// Handler implements the opentalon plugin interface for pipeline confirmation decisions.
type Handler struct {
	mode            string
	skipPatterns    []*regexp.Regexp
	confirmPatterns []*regexp.Regexp
}

// Config is the JSON configuration passed via Configure.
type Config struct {
	Mode            string   `json:"mode"`             // "always" | "write_only" | "never"; default "write_only"
	SkipPatterns    []string `json:"skip_patterns"`    // regex patterns for actions that skip confirmation
	ConfirmPatterns []string `json:"confirm_patterns"` // regex patterns for actions that always confirm
}

// StepInfo is one step from the planner's pipeline result.
type StepInfo struct {
	Plugin string `json:"plugin"`
	Action string `json:"action"`
	Name   string `json:"name"`
}

// CheckResult is the structured response from check_confirmation.
type CheckResult struct {
	RequiresConfirmation bool     `json:"requires_confirmation"`
	Reason               string   `json:"reason"`
	WriteActions         []string `json:"write_actions,omitempty"`
}

// DefaultConfig returns sensible defaults for write-only confirmation.
func DefaultConfig() Config {
	return Config{
		Mode: "write_only",
		SkipPatterns: []string{
			"^list-", "^show-", "^search-", "^get-",
			"ask_knowledge",
		},
		ConfirmPatterns: []string{
			"^create-", "^update-", "^delete-",
			"^assign-", "^consume-", "^remove-",
			"^schedule-", "^return-", "^change-",
			"^complete-", "^add-to-", "^batch-",
		},
	}
}

// NewHandler creates a handler with default configuration.
func NewHandler() *Handler {
	cfg := DefaultConfig()
	h := &Handler{mode: cfg.Mode}
	h.compilePatterns(cfg)
	return h
}

func (h *Handler) compilePatterns(cfg Config) {
	h.skipPatterns = compileRegexList(cfg.SkipPatterns)
	h.confirmPatterns = compileRegexList(cfg.ConfirmPatterns)
}

func compileRegexList(patterns []string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			log.Printf("planner-plugin: invalid regex %q: %v (skipped)", p, err)
			continue
		}
		out = append(out, re)
	}
	return out
}

// Capabilities returns the plugin's self-description.
func (h *Handler) Capabilities() plugin.CapabilitiesMsg {
	return plugin.CapabilitiesMsg{
		Name:        "planner",
		Description: "Pipeline confirmation strategy plugin",
		Actions: []plugin.ActionMsg{
			{
				Name:        "check_confirmation",
				Description: "Decide whether a multi-step pipeline requires user confirmation before execution",
				Parameters: []plugin.ParameterMsg{
					{Name: "steps", Description: "JSON array of pipeline steps [{plugin, action, name}]", Type: "json", Required: true},
				},
				UserOnly: true, // not exposed to LLM — called internally by orchestrator
			},
		},
	}
}

// Configure applies operator-provided JSON configuration.
func (h *Handler) Configure(configJSON string) error {
	if configJSON == "" {
		return nil // keep defaults
	}
	var cfg Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("planner-plugin: invalid config: %w", err)
	}
	if cfg.Mode != "" {
		h.mode = cfg.Mode
	}
	if cfg.SkipPatterns != nil || cfg.ConfirmPatterns != nil {
		defaults := DefaultConfig()
		if cfg.SkipPatterns == nil {
			cfg.SkipPatterns = defaults.SkipPatterns
		}
		if cfg.ConfirmPatterns == nil {
			cfg.ConfirmPatterns = defaults.ConfirmPatterns
		}
		h.compilePatterns(cfg)
	}
	log.Printf("planner-plugin: configured mode=%s skip_patterns=%d confirm_patterns=%d",
		h.mode, len(h.skipPatterns), len(h.confirmPatterns))
	return nil
}

// Execute handles action requests.
func (h *Handler) Execute(req plugin.Request) plugin.Response {
	switch req.Action {
	case "check_confirmation":
		return h.checkConfirmation(req)
	default:
		return plugin.Response{
			CallID: req.ID,
			Error:  fmt.Sprintf("unknown action %q", req.Action),
		}
	}
}

func (h *Handler) checkConfirmation(req plugin.Request) plugin.Response {
	stepsJSON := req.Args["steps"]
	if stepsJSON == "" {
		return plugin.Response{CallID: req.ID, Error: "missing required argument: steps"}
	}

	var steps []StepInfo
	if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
		return plugin.Response{CallID: req.ID, Error: fmt.Sprintf("invalid steps JSON: %v", err)}
	}

	result := h.CheckConfirmation(steps)

	structured, _ := json.Marshal(result)
	return plugin.Response{
		CallID:            req.ID,
		Content:           result.Reason,
		StructuredContent: string(structured),
	}
}

// CheckConfirmation decides whether a pipeline requires user confirmation.
func (h *Handler) CheckConfirmation(steps []StepInfo) CheckResult {
	switch h.mode {
	case "never":
		return CheckResult{RequiresConfirmation: false, Reason: "confirmation disabled"}
	case "write_only":
		return h.checkWriteOnly(steps)
	default: // "always"
		return CheckResult{RequiresConfirmation: true, Reason: "confirmation mode: always"}
	}
}

func (h *Handler) checkWriteOnly(steps []StepInfo) CheckResult {
	var writeActions []string
	for _, step := range steps {
		action := extractActionName(step.Action)

		// Explicit confirm patterns take priority
		if h.matchesAny(action, h.confirmPatterns) {
			writeActions = append(writeActions, step.Action)
			continue
		}

		// Explicit skip patterns
		if h.matchesAny(action, h.skipPatterns) {
			continue
		}

		// Fallback: naming convention
		if isReadOnlyByConvention(action) {
			continue
		}

		// Unknown action → assume write (fail-safe)
		writeActions = append(writeActions, step.Action)
	}

	if len(writeActions) > 0 {
		return CheckResult{
			RequiresConfirmation: true,
			Reason:               "write operations: " + strings.Join(writeActions, ", "),
			WriteActions:         writeActions,
		}
	}
	return CheckResult{RequiresConfirmation: false, Reason: "all steps are read-only"}
}

func (h *Handler) matchesAny(action string, patterns []*regexp.Regexp) bool {
	for _, pat := range patterns {
		if pat.MatchString(action) {
			return true
		}
	}
	return false
}

// extractActionName strips the plugin prefix (e.g. "timly__list-items" → "list-items").
func extractActionName(action string) string {
	if idx := strings.LastIndex(action, "__"); idx >= 0 {
		return action[idx+2:]
	}
	return action
}

// isReadOnlyByConvention returns true for action names that are conventionally read-only.
func isReadOnlyByConvention(action string) bool {
	return strings.HasPrefix(action, "list-") ||
		strings.HasPrefix(action, "show-") ||
		strings.HasPrefix(action, "search-") ||
		strings.HasPrefix(action, "list_") ||
		strings.HasPrefix(action, "show_") ||
		strings.HasPrefix(action, "get-") ||
		strings.HasPrefix(action, "get_") ||
		action == "ask_knowledge"
}
