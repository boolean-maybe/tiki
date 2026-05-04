package taskdetail

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/util/gradient"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/rivo/tview"
)

// Base contains shared state and methods for task detail/edit views.
// Both TaskDetailView and TaskEditView embed this struct to share common functionality.
type Base struct {
	// Layout
	root    *tview.Flex
	content *tview.Flex

	// Dependencies
	taskStore    store.Store
	taskID       string
	imageManager *navtview.ImageManager
	mermaidOpts  *nav.MermaidOptions
	descView     tview.Primitive

	// Task data
	fallbackTiki   *tikipkg.Tiki
	taskController *controller.TaskController

	// Shared state
	fullscreen         bool
	focusSetter        func(tview.Primitive)
	onFullscreenChange func(bool)
}

// build initializes the root and content flex layouts
func (b *Base) build() {
	b.content = tview.NewFlex().SetDirection(tview.FlexRow)
	b.root = tview.NewFlex().SetDirection(tview.FlexRow)
	b.root.AddItem(b.content, 0, 1, true)
}

// GetTiki returns the tiki from the store or the fallback tiki
func (b *Base) GetTiki() *tikipkg.Tiki {
	tk := b.taskStore.GetTiki(b.taskID)
	if tk == nil && b.fallbackTiki != nil && b.fallbackTiki.ID == b.taskID {
		tk = b.fallbackTiki
	}
	return tk
}

// GetPrimitive returns the root tview primitive
func (b *Base) GetPrimitive() tview.Primitive {
	return b.root
}

// SetFallbackTiki sets a tiki to render when it does not yet exist in the store (draft mode)
func (b *Base) SetFallbackTiki(tk *tikipkg.Tiki) {
	b.fallbackTiki = tk
}

// SetTaskController sets the task controller for edit session management
func (b *Base) SetTaskController(tc *controller.TaskController) {
	b.taskController = tc
}

// SetFocusSetter sets the callback for requesting focus changes
func (b *Base) SetFocusSetter(setter func(tview.Primitive)) {
	b.focusSetter = setter
}

// SetFullscreenChangeHandler sets the callback for when fullscreen state changes
func (b *Base) SetFullscreenChangeHandler(handler func(isFullscreen bool)) {
	b.onFullscreenChange = handler
}

// IsFullscreen reports whether the view is currently in fullscreen mode
func (b *Base) IsFullscreen() bool {
	return b.fullscreen
}

// assembleMetadataBox builds the framed metadata box from pre-built title and column
// primitives. It computes blocked tasks once and passes them to both BuildSectionInputs
// and RenderBlocksColumn, avoiding a double-scan.
func (b *Base) assembleMetadataBox(
	tk *tikipkg.Tiki,
	colors *config.ColorConfig,
	titlePrimitive tview.Primitive,
	col1, col2, col3 *tview.Flex,
	titleHeight int,
) *tview.Frame {
	metadataContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	metadataContainer.AddItem(titlePrimitive, titleHeight, 0, titleHeight > 0)
	metadataContainer.AddItem(tview.NewBox(), 1, 0, false)

	blocked := findBlockedTikis(b.taskStore.GetAllTikis(), tk.ID)
	primitives := map[SectionID]tview.Primitive{
		SectionStatusGroup: col1,
		SectionPeopleGroup: col2,
		SectionDueGroup:    col3,
		SectionTags:        RenderTagsColumn(tk),
		SectionDependsOn:   RenderDependsOnColumn(tk, b.taskStore),
		SectionBlocks:      RenderBlocksColumn(blocked),
	}

	inputs := BuildSectionInputs(tk, len(blocked) > 0)
	metadataRow := newResponsiveMetadataRow(inputs, primitives)
	metadataContainer.AddItem(metadataRow, 0, 1, false)

	metadataBox := tview.NewFrame(metadataContainer).SetBorders(0, 0, 0, 0, 0, 0)
	metadataBox.SetBorder(true).SetTitle(
		fmt.Sprintf(" %s ", gradient.RenderAdaptiveGradientText(tk.ID, colors.TaskDetailIDColor, colors.FallbackTaskIDColor)),
	).SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())
	metadataBox.SetBorderPadding(1, 0, 2, 2)

	return metadataBox
}

// defaultString returns def if s is empty, otherwise s
func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// findBlockedTikis returns all tikis whose dependsOn list contains contextID.
func findBlockedTikis(allTikis []*tikipkg.Tiki, contextID string) []*tikipkg.Tiki {
	var result []*tikipkg.Tiki
	for _, tk := range allTikis {
		deps, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
		for _, dep := range deps {
			if strings.ToUpper(dep) == contextID {
				result = append(result, tk)
				break
			}
		}
	}
	return result
}
