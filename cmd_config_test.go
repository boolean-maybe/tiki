package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
)

func TestParseConfigResetArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		target    config.ResetTarget
		scope     config.ResetScope
		wantErr   error  // sentinel match (nil = no error)
		errSubstr string // substring match for non-sentinel errors
	}{
		{
			name:   "global all",
			args:   []string{"--global"},
			target: config.TargetAll,
			scope:  config.ScopeGlobal,
		},
		{
			name:   "local workflow",
			args:   []string{"workflow", "--local"},
			target: config.TargetWorkflow,
			scope:  config.ScopeLocal,
		},
		{
			name:   "current config",
			args:   []string{"--current", "config"},
			target: config.TargetConfig,
			scope:  config.ScopeCurrent,
		},
		{
			name:   "global new",
			args:   []string{"new", "--global"},
			target: config.TargetNew,
			scope:  config.ScopeGlobal,
		},
		{
			name:    "help flag",
			args:    []string{"--help"},
			wantErr: errHelpRequested,
		},
		{
			name:    "short help flag",
			args:    []string{"-h"},
			wantErr: errHelpRequested,
		},
		{
			name:      "missing scope",
			args:      []string{"config"},
			errSubstr: "scope required",
		},
		{
			name:      "unknown flag",
			args:      []string{"--verbose"},
			errSubstr: "unknown flag",
		},
		{
			name:      "unknown target",
			args:      []string{"themes", "--global"},
			errSubstr: "unknown target",
		},
		{
			name:      "multiple targets",
			args:      []string{"config", "workflow", "--global"},
			errSubstr: "multiple targets",
		},
		{
			name:      "duplicate scopes",
			args:      []string{"--global", "--local"},
			errSubstr: "only one scope allowed",
		},
		{
			name:      "no args",
			args:      nil,
			errSubstr: "scope required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, scope, err := parseConfigResetArgs(tt.args)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if tt.errSubstr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if msg := err.Error(); !strings.Contains(msg, tt.errSubstr) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, msg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if target != tt.target {
				t.Errorf("target = %q, want %q", target, tt.target)
			}
			if scope != tt.scope {
				t.Errorf("scope = %q, want %q", scope, tt.scope)
			}
		})
	}
}
