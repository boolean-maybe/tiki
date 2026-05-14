package config

import "testing"

// TestResolveRole_AllValidRolesResolve guards the contract that every name
// in workflow.ValidRoles maps to a non-zero color through ColorConfig.
// workflow imports nothing from config, so the vocabulary lives there; this
// test enforces the binding from the config side.
func TestResolveRole_AllValidRolesResolve(t *testing.T) {
	cc := ColorsFromPalette(DarkPalette())
	for _, role := range []string{"muted", "accent", "highlight", "info", "text", "danger", "warn", "ok"} {
		t.Run(role, func(t *testing.T) {
			c, ok := cc.ResolveRole(role)
			if !ok {
				t.Fatalf("role %q not resolved", role)
			}
			if c.IsDefault() {
				t.Errorf("role %q resolved to default/transparent color", role)
			}
		})
	}
}

func TestResolveRole_UnknownRoleReturnsFalse(t *testing.T) {
	cc := ColorsFromPalette(DarkPalette())
	if _, ok := cc.ResolveRole("nosuchrole"); ok {
		t.Error("unknown role should not resolve")
	}
	if _, ok := cc.ResolveRole(""); ok {
		t.Error("empty role should not resolve")
	}
}

// TestResolveRole_AllPalettesProvideRoleColors guards that no theme leaves
// danger/warn/ok at the zero value. The closed role vocabulary is a
// contract: workflow YAMLs reference it, so a theme that forgot to set
// these would render uncolored text in compact UIs.
func TestResolveRole_AllPalettesProvideRoleColors(t *testing.T) {
	for name, info := range themeRegistry {
		t.Run(name, func(t *testing.T) {
			cc := ColorsFromPalette(info.Palette())
			for _, role := range []string{"danger", "warn", "ok"} {
				c, ok := cc.ResolveRole(role)
				if !ok || c.IsDefault() {
					t.Errorf("theme %q: role %q missing or default", name, role)
				}
			}
		})
	}
}
