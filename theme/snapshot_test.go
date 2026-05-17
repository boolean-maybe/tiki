package theme

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// snapshotEntry is one row in testdata/color_snapshot.json — a (theme, key) pair
// mapped to the hex color the OLD code path produced for that slot. Captured
// pre-refactor; the live test walks each entry and verifies the NEW theme
// package reproduces the same hex string for the mapped role.
type snapshotEntry struct {
	Theme string `json:"theme"`
	Key   string `json:"key"`
	Hex   string `json:"hex"`
}

// roleKeyMapping declares which Theme getter / ResolveByName key produces each
// old ColorConfig slot value. The snapshot test reads color_snapshot.json
// (captured from the OLD code) and walks this mapping in the NEW code to
// confirm every old slot has an equivalent new role producing identical output.
//
// Format: oldSlotKey → "method:Name" | "pair:Name.fg"
var roleKeyMapping = map[string]string{
	// Text-primary group
	"ContentTextColor":           "method:TextPrimary",
	"TikiListSelectionFg":        "method:TextPrimary",
	"TikiDetailTitleText":        "method:TextPrimary",
	"InputBoxLabelColor":         "method:TextPrimary",
	"InputBoxTextColor":          "method:TextPrimary",
	"InputFieldTextColor":        "method:TextPrimary",
	"HeaderKeyText":              "method:TextPrimary",
	"TikiDetailEditFocusText":    "method:TextPrimary",
	"TikiListStatusPendingColor": "method:TextPrimary",

	// Text-secondary
	"TikiBoxTitleColor":            "method:TextSecondary",
	"TikiDetailEditDimValueColor":  "method:TextSecondary",
	"HeaderActionPluginLabelColor": "method:TextSecondary",
	"TikiDetailTagForeground":      "method:TextSecondary",

	// Text-muted
	"TikiBoxLabelColor":           "method:TextMuted",
	"TikiBoxDescriptionColor":     "method:TextMuted",
	"TikiDetailEditDimTextColor":  "method:TextMuted",
	"TikiDetailEditDimLabelColor": "method:TextMuted",
	"TikiDetailPlaceholderColor":  "method:TextMuted",
	"HeaderInfoSeparator":         "method:TextMuted",
	"HeaderInfoDesc":              "method:TextMuted",
	"HeaderActionViewLabelColor":  "method:TextMuted",

	// Text-label
	"TikiDetailLabelText":     "method:TextLabel",
	"TikiListStatusDoneColor": "method:TextLabel",

	// Text-value
	"TikiDetailValueText": "method:TextValue",

	// Text-hint
	"CompletionHintColor": "method:TextHint",

	// Border
	"TikiBoxSelectedBorder":   "method:BorderFocus",
	"TikiBoxUnselectedBorder": "method:BorderIdle",

	// Surface
	"TikiBoxUnselectedBackground": "method:SurfaceTransparent",
	"InputBoxBackgroundColor":     "method:SurfaceTransparent",
	"InputFieldBackgroundColor":   "method:SurfaceTransparent",
	"TikiListSelectionBg":         "method:SurfaceSelection",
	"TikiDetailTagBackground":     "method:SurfaceSelection",
	"ContentBackgroundColor":      "method:SurfaceCanvas",

	// Accent / focus
	"TikiDetailEditFocusMarker":    "method:Highlight",
	"TikiDetailCommentAuthor":      "method:Highlight",
	"HeaderKeyBinding":             "method:Highlight",
	"HeaderActionGlobalKeyColor":   "method:Highlight",
	"StatuslineErrorFg":            "pair:StatuslineError.fg",
	"HeaderActionViewKeyColor":     "method:AccentAction",
	"TikiBoxTagValueColor":         "method:AccentTag",
	"HeaderActionGlobalLabelColor": "method:TextPrimary",

	// Status
	"DangerColor":                "method:StatusDanger",
	"WarnColor":                  "method:StatusWarn",
	"HeaderInfoLabel":            "method:StatusWarn",
	"HeaderActionPluginKeyColor": "method:StatusWarn",
	"OkColor":                    "method:StatusOk",
	"StatuslineInfoFg":           "pair:StatuslineInfo.fg",

	// Statusline
	"StatuslineBg":       "pair:StatuslineMain.bg",
	"StatuslineFg":       "pair:StatuslineMain.fg",
	"StatuslineAccentBg": "pair:StatuslineAccent.bg",
	"StatuslineAccentFg": "pair:StatuslineAccent.fg",
	"StatuslineInfoBg":   "pair:StatuslineInfo.bg",
	"StatuslineErrorBg":  "pair:StatuslineError.bg",
	"StatuslineFillBg":   "method:StatuslineFill",

	// Plugin-specific
	"DepsEditorBackground": "method:DepsEditorSurface",

	// Logo
	"LogoDotColor":    "method:LogoDot",
	"LogoShadeColor":  "method:LogoShade",
	"LogoBorderColor": "method:LogoBorder",
}

func TestSnapshotMatchesOld(t *testing.T) {
	raw, err := os.ReadFile("testdata/color_snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []snapshotEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}

	byTheme := map[string][]snapshotEntry{}
	for _, e := range entries {
		byTheme[e.Theme] = append(byTheme[e.Theme], e)
	}

	themes := make([]string, 0, len(byTheme))
	for k := range byTheme {
		themes = append(themes, k)
	}
	sort.Strings(themes)

	for _, themeName := range themes {
		th := LoadByName(themeName)
		for _, e := range byTheme[themeName] {
			got, ok := lookupRoleHex(th, e.Key)
			if !ok {
				t.Errorf("[%s] no mapping for old key %q", themeName, e.Key)
				continue
			}
			if got != e.Hex {
				t.Errorf("[%s] %s: got %s, want %s", themeName, e.Key, got, e.Hex)
			}
		}
	}
}

func lookupRoleHex(th *Theme, key string) (string, bool) {
	// Caption pairs
	if matched, fgBg, idx, ok := parseCaption(key); ok {
		pair := th.PluginCaptions().At(idx)
		if matched == "CaptionFg" || fgBg == "fg" {
			return pair.Fg().Hex(), true
		}
		return pair.Bg().Hex(), true
	}
	// Aliases stored under "alias:<name>"
	if len(key) > 6 && key[:6] == "alias:" {
		r, ok := th.ResolveByName(key[6:])
		if !ok {
			return "", false
		}
		return r.Hex(), true
	}

	mapping, ok := roleKeyMapping[key]
	if !ok {
		return "", false
	}
	return resolveMapping(th, mapping)
}

func resolveMapping(th *Theme, mapping string) (string, bool) {
	idx := strings.Index(mapping, ":")
	if idx < 0 {
		return "", false
	}
	kind := mapping[:idx]
	rest := mapping[idx+1:]
	switch kind {
	case "method":
		return resolveByGetter(th, rest)
	case "pair":
		return resolvePairSide(th, rest)
	}
	return "", false
}

func resolveByGetter(th *Theme, name string) (string, bool) {
	switch name {
	case "TextPrimary":
		return th.TextPrimary().Hex(), true
	case "TextSecondary":
		return th.TextSecondary().Hex(), true
	case "TextMuted":
		return th.TextMuted().Hex(), true
	case "TextLabel":
		return th.TextLabel().Hex(), true
	case "TextValue":
		return th.TextValue().Hex(), true
	case "TextHint":
		return th.TextHint().Hex(), true
	case "BorderFocus":
		return th.BorderFocus().Hex(), true
	case "BorderIdle":
		return th.BorderIdle().Hex(), true
	case "SurfaceTransparent":
		return th.SurfaceTransparent().Hex(), true
	case "SurfaceSelection":
		return th.SurfaceSelection().Hex(), true
	case "SurfaceCanvas":
		return th.SurfaceCanvas().Hex(), true
	case "Highlight":
		return th.Highlight().Hex(), true
	case "AccentAction":
		return th.AccentAction().Hex(), true
	case "AccentTag":
		return th.AccentTag().Hex(), true
	case "StatusDanger":
		return th.StatusDanger().Hex(), true
	case "StatusWarn":
		return th.StatusWarn().Hex(), true
	case "StatusOk":
		return th.StatusOk().Hex(), true
	case "StatuslineFill":
		return th.StatuslineFill().Hex(), true
	case "DepsEditorSurface":
		return th.DepsEditorSurface().Hex(), true
	case "LogoDot":
		return th.LogoDot().Hex(), true
	case "LogoShade":
		return th.LogoShade().Hex(), true
	case "LogoBorder":
		return th.LogoBorder().Hex(), true
	}
	return "", false
}

func resolvePairSide(th *Theme, spec string) (string, bool) {
	dot := strings.Index(spec, ".")
	if dot < 0 {
		return "", false
	}
	pairName, side := spec[:dot], spec[dot+1:]
	var p PairRole
	switch pairName {
	case "StatuslineMain":
		p = th.StatuslineMain()
	case "StatuslineAccent":
		p = th.StatuslineAccent()
	case "StatuslineInfo":
		p = th.StatuslineInfo()
	case "StatuslineError":
		p = th.StatuslineError()
	default:
		return "", false
	}
	if side == "fg" {
		return p.Fg().Hex(), true
	}
	return p.Bg().Hex(), true
}

func parseCaption(key string) (prefix, fgBg string, idx int, ok bool) {
	if len(key) < len("CaptionFg[0]") {
		return "", "", 0, false
	}
	prefix = key[:9]
	if prefix != "CaptionFg" && prefix != "CaptionBg" {
		return "", "", 0, false
	}
	if key[9] != '[' || key[len(key)-1] != ']' {
		return "", "", 0, false
	}
	num := key[10 : len(key)-1]
	for _, ch := range num {
		if ch < '0' || ch > '9' {
			return "", "", 0, false
		}
		idx = idx*10 + int(ch-'0')
	}
	if prefix == "CaptionFg" {
		fgBg = "fg"
	} else {
		fgBg = "bg"
	}
	return prefix, fgBg, idx, true
}
