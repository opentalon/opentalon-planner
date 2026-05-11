package main

import (
	"encoding/json"
	"testing"

	"github.com/opentalon/opentalon/pkg/plugin"
)

func TestCheckConfirmation_WriteOnly_ReadPipeline(t *testing.T) {
	h := NewHandler()
	h.mode = "write_only"

	result := h.CheckConfirmation([]StepInfo{
		{Plugin: "timly", Action: "timly__list-categories", Name: "List categories"},
		{Plugin: "timly", Action: "timly__show-item", Name: "Show item"},
	})
	if result.RequiresConfirmation {
		t.Errorf("read-only pipeline should not require confirmation, got reason: %s", result.Reason)
	}
}

func TestCheckConfirmation_WriteOnly_WritePipeline(t *testing.T) {
	h := NewHandler()
	h.mode = "write_only"

	result := h.CheckConfirmation([]StepInfo{
		{Plugin: "timly", Action: "timly__list-categories", Name: "List categories"},
		{Plugin: "timly", Action: "timly__create-item", Name: "Create item"},
	})
	if !result.RequiresConfirmation {
		t.Error("pipeline with create-item should require confirmation")
	}
	if len(result.WriteActions) != 1 || result.WriteActions[0] != "timly__create-item" {
		t.Errorf("write_actions = %v, want [timly__create-item]", result.WriteActions)
	}
	if result.ConfirmBeforeStep != 1 {
		t.Errorf("confirm_before_step = %d, want 1 (first write is step 1)", result.ConfirmBeforeStep)
	}
}

func TestCheckConfirmation_WriteOnly_ConfirmBeforeStep(t *testing.T) {
	h := NewHandler()
	h.mode = "write_only"

	// list → show → delete: confirm before step 2
	result := h.CheckConfirmation([]StepInfo{
		{Plugin: "timly", Action: "timly__list-items", Name: "Find item"},
		{Plugin: "timly", Action: "timly__show-item", Name: "Get details"},
		{Plugin: "timly", Action: "timly__delete-item", Name: "Delete item"},
	})
	if result.ConfirmBeforeStep != 2 {
		t.Errorf("confirm_before_step = %d, want 2", result.ConfirmBeforeStep)
	}

	// All read: confirm_before_step = -1
	result = h.CheckConfirmation([]StepInfo{
		{Plugin: "timly", Action: "timly__list-items", Name: "List"},
		{Plugin: "timly", Action: "timly__show-item", Name: "Show"},
	})
	if result.ConfirmBeforeStep != -1 {
		t.Errorf("all-read confirm_before_step = %d, want -1", result.ConfirmBeforeStep)
	}

	// Write at step 0: confirm before step 0 (confirm everything)
	result = h.CheckConfirmation([]StepInfo{
		{Plugin: "timly", Action: "timly__delete-item", Name: "Delete"},
	})
	if result.ConfirmBeforeStep != 0 {
		t.Errorf("first-step-write confirm_before_step = %d, want 0", result.ConfirmBeforeStep)
	}
}

func TestCheckConfirmation_WriteOnly_AllWriteActions(t *testing.T) {
	h := NewHandler()
	h.mode = "write_only"

	writeActions := []string{
		"create-item", "update-item", "delete-item",
		"assign-item", "consume-item", "remove-from-stock",
		"schedule-item", "return-item", "change-item-status",
		"complete-ticket", "add-to-stock", "batch-item-operation",
	}
	for _, action := range writeActions {
		result := h.CheckConfirmation([]StepInfo{
			{Plugin: "timly", Action: "timly__" + action, Name: action},
		})
		if !result.RequiresConfirmation {
			t.Errorf("action %q should require confirmation", action)
		}
	}
}

func TestCheckConfirmation_WriteOnly_AllReadActions(t *testing.T) {
	h := NewHandler()
	h.mode = "write_only"

	readActions := []string{
		"list-items", "show-item", "list-categories",
		"show-category", "list-containers", "show-container",
		"list-persons", "show-person", "list-org-units",
		"search-items", "get-status",
	}
	for _, action := range readActions {
		result := h.CheckConfirmation([]StepInfo{
			{Plugin: "timly", Action: "timly__" + action, Name: action},
		})
		if result.RequiresConfirmation {
			t.Errorf("action %q should NOT require confirmation, reason: %s", action, result.Reason)
		}
	}
}

func TestCheckConfirmation_ModeAlways(t *testing.T) {
	h := NewHandler()
	h.mode = "always"

	result := h.CheckConfirmation([]StepInfo{
		{Plugin: "timly", Action: "timly__list-items", Name: "List items"},
	})
	if !result.RequiresConfirmation {
		t.Error("mode=always should always require confirmation")
	}
}

func TestCheckConfirmation_ModeNever(t *testing.T) {
	h := NewHandler()
	h.mode = "never"

	result := h.CheckConfirmation([]StepInfo{
		{Plugin: "timly", Action: "timly__delete-item", Name: "Delete item"},
	})
	if result.RequiresConfirmation {
		t.Error("mode=never should never require confirmation")
	}
}

func TestCheckConfirmation_UnknownAction_FailSafe(t *testing.T) {
	h := NewHandler()
	h.mode = "write_only"

	result := h.CheckConfirmation([]StepInfo{
		{Plugin: "timly", Action: "timly__some-unknown-action", Name: "Unknown"},
	})
	if !result.RequiresConfirmation {
		t.Error("unknown action should require confirmation (fail-safe)")
	}
}

func TestCheckConfirmation_AskKnowledge_SkippedInWriteOnly(t *testing.T) {
	h := NewHandler()
	h.mode = "write_only"

	result := h.CheckConfirmation([]StepInfo{
		{Plugin: "weaviate", Action: "ask_knowledge", Name: "Search knowledge"},
		{Plugin: "timly", Action: "timly__list-items", Name: "List items"},
	})
	if result.RequiresConfirmation {
		t.Error("ask_knowledge + list-items should not require confirmation")
	}
}

func TestCheckConfirmation_MixedPipeline(t *testing.T) {
	h := NewHandler()
	h.mode = "write_only"

	result := h.CheckConfirmation([]StepInfo{
		{Plugin: "timly", Action: "timly__list-categories", Name: "List categories"},
		{Plugin: "timly", Action: "timly__list-org-units", Name: "List org-units"},
		{Plugin: "timly", Action: "timly__create-item", Name: "Create item"},
	})
	if !result.RequiresConfirmation {
		t.Error("mixed pipeline with create should require confirmation")
	}
	if len(result.WriteActions) != 1 {
		t.Errorf("expected 1 write action, got %d: %v", len(result.WriteActions), result.WriteActions)
	}
}

func TestConfigure_CustomPatterns(t *testing.T) {
	h := NewHandler()
	err := h.Configure(`{
		"mode": "write_only",
		"skip_patterns": ["^list-", "^show-", "^my-custom-read-"],
		"confirm_patterns": ["^create-", "^my-custom-write-"]
	}`)
	if err != nil {
		t.Fatal(err)
	}

	// Custom read pattern
	result := h.CheckConfirmation([]StepInfo{
		{Plugin: "x", Action: "my-custom-read-data", Name: "Custom read"},
	})
	if result.RequiresConfirmation {
		t.Error("custom skip pattern should not require confirmation")
	}

	// Custom write pattern
	result = h.CheckConfirmation([]StepInfo{
		{Plugin: "x", Action: "my-custom-write-data", Name: "Custom write"},
	})
	if !result.RequiresConfirmation {
		t.Error("custom confirm pattern should require confirmation")
	}
}

func TestConfigure_InvalidJSON(t *testing.T) {
	h := NewHandler()
	err := h.Configure("not-json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfigure_EmptyString(t *testing.T) {
	h := NewHandler()
	err := h.Configure("")
	if err != nil {
		t.Errorf("empty config should not error: %v", err)
	}
	if h.mode != "write_only" {
		t.Errorf("mode should be default write_only, got %q", h.mode)
	}
}

func TestExecute_CheckConfirmation(t *testing.T) {
	h := NewHandler()

	steps, _ := json.Marshal([]StepInfo{
		{Plugin: "timly", Action: "timly__list-items", Name: "List items"},
	})
	resp := h.Execute(plugin.Request{
		ID:     "req-1",
		Action: "check_confirmation",
		Args:   map[string]string{"steps": string(steps)},
	})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	var result CheckResult
	if err := json.Unmarshal([]byte(resp.StructuredContent), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.RequiresConfirmation {
		t.Error("list-items should not require confirmation")
	}
}

func TestExecute_UnknownAction(t *testing.T) {
	h := NewHandler()
	resp := h.Execute(plugin.Request{ID: "req-1", Action: "unknown"})
	if resp.Error == "" {
		t.Error("expected error for unknown action")
	}
}

func TestExecute_MissingSteps(t *testing.T) {
	h := NewHandler()
	resp := h.Execute(plugin.Request{ID: "req-1", Action: "check_confirmation", Args: map[string]string{}})
	if resp.Error == "" {
		t.Error("expected error for missing steps")
	}
}

func TestExtractActionName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"timly__list-items", "list-items"},
		{"timly__create-item", "create-item"},
		{"list-items", "list-items"},
		{"ask_knowledge", "ask_knowledge"},
		{"mcp__timly__show-item", "show-item"},
	}
	for _, c := range cases {
		got := extractActionName(c.in)
		if got != c.want {
			t.Errorf("extractActionName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
