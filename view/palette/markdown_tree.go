package palette

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
)

// matchingFiles returns the set of file RelPaths whose path fuzzy-matches
// query. An empty query matches every file.
func matchingFiles(root *store.MarkdownDir, query string) map[string]bool {
	out := make(map[string]bool)
	var walk func(d *store.MarkdownDir)
	walk = func(d *store.MarkdownDir) {
		for _, f := range d.Files {
			if query == "" {
				out[f.RelPath] = true
				continue
			}
			if ok, _ := fuzzyMatch(query, f.RelPath); ok {
				out[f.RelPath] = true
			}
		}
		for _, sub := range d.Dirs {
			walk(sub)
		}
	}
	walk(root)
	return out
}

// expandedDirsForMatches returns the set of dir RelPaths that are ancestors of
// any matched file — these must be force-expanded while a filter is active.
func expandedDirsForMatches(root *store.MarkdownDir, matched map[string]bool) map[string]bool {
	out := make(map[string]bool)
	var walk func(d *store.MarkdownDir) bool
	walk = func(d *store.MarkdownDir) bool {
		has := false
		for _, f := range d.Files {
			if matched[f.RelPath] {
				has = true
			}
		}
		for _, sub := range d.Dirs {
			if walk(sub) {
				has = true
			}
		}
		if has && d.RelPath != "" {
			out[d.RelPath] = true
		}
		return has
	}
	walk(root)
	return out
}

// --- widget ---

type mdNodeRef struct {
	isDir   bool
	relPath string
	absPath string
}

// MarkdownTree is a modal overlay listing .md files under the cwd as a
// collapsible directory tree, filterable by fuzzy typing.
type MarkdownTree struct {
	root        *tview.Flex
	filterInput *tview.InputField
	tree        *tview.TreeView
	hintView    *tview.TextView
	cfg         *model.MarkdownTreeConfig

	scanned      *store.MarkdownDir
	manualExpand map[string]bool // saved expand state captured before first filter keystroke
	filtering    bool
	isDoc        func(absPath string) bool
}

// NewMarkdownTree constructs the overlay widget bound to cfg.
func NewMarkdownTree(cfg *model.MarkdownTreeConfig) *MarkdownTree {
	roles := theme.Roles()
	mt := &MarkdownTree{cfg: cfg, manualExpand: map[string]bool{}}

	mt.filterInput = tview.NewInputField()
	mt.filterInput.SetLabel(" ")
	mt.filterInput.SetFieldBackgroundColor(roles.SurfaceCanvas().TCell())
	mt.filterInput.SetFieldTextColor(roles.TextPrimary().TCell())
	mt.filterInput.SetLabelColor(roles.TextPrimary().TCell())
	mt.filterInput.SetPlaceholder("Type to filter")
	mt.filterInput.SetPlaceholderStyle(tcell.StyleDefault.
		Foreground(roles.TextMuted().TCell()).
		Background(roles.SurfaceCanvas().TCell()))
	mt.filterInput.SetBackgroundColor(roles.SurfaceCanvas().TCell())

	mt.tree = tview.NewTreeView()
	mt.tree.SetBackgroundColor(roles.SurfaceCanvas().TCell())
	mt.tree.SetTopLevel(1)                              // hide the synthetic root; render its children at the top
	mt.tree.SetBorderPadding(1, 1, 0, 0)                // blank line above first node and below last node (matches All Actions)
	mt.tree.SetGraphicsColor(roles.TextMuted().TCell()) // dim the ├─ └─ connecting lines

	mt.hintView = tview.NewTextView().SetDynamicColors(true)
	mt.hintView.SetBackgroundColor(roles.SurfaceCanvas().TCell())
	mutedHex := roles.TextMuted().Hex()
	mt.hintView.SetText(fmt.Sprintf(" [%s]↑↓ Move  → Expand  ← Collapse  ⏎ Open  Esc Cancel", mutedHex))

	mt.root = tview.NewFlex().SetDirection(tview.FlexRow)
	mt.root.SetBackgroundColor(roles.SurfaceCanvas().TCell())
	mt.root.SetBorder(true)
	mt.root.SetBorderColor(roles.BorderIdle().TCell())
	mt.root.AddItem(mt.filterInput, 1, 0, true)
	mt.root.AddItem(mt.tree, 0, 1, false)
	mt.root.AddItem(mt.hintView, 1, 0, false)

	mt.filterInput.SetInputCapture(mt.handleFilterInput)
	return mt
}

// GetPrimitive returns the overlay root primitive.
func (mt *MarkdownTree) GetPrimitive() tview.Primitive { return mt.root }

// GetFilterInput returns the filter input primitive (for focus wiring).
func (mt *MarkdownTree) GetFilterInput() tview.Primitive { return mt.filterInput }

// SetDocMarker installs a predicate that marks files backed by a tiki doc.
func (mt *MarkdownTree) SetDocMarker(fn func(string) bool) { mt.isDoc = fn }

// OnShow receives the freshly scanned tree and rebuilds the view.
func (mt *MarkdownTree) OnShow(root *store.MarkdownDir) {
	mt.scanned = root
	mt.filtering = false
	mt.manualExpand = map[string]bool{}
	mt.filterInput.SetText("")
	mt.rebuild("")
}

// SetChangedFunc wires the filter input's change handler.
func (mt *MarkdownTree) SetChangedFunc() {
	mt.filterInput.SetChangedFunc(func(text string) {
		mt.onFilterChanged(text)
	})
}

func (mt *MarkdownTree) onFilterChanged(text string) {
	if text != "" && !mt.filtering {
		mt.captureManualExpand()
		mt.filtering = true
	}
	if text == "" && mt.filtering {
		mt.filtering = false
	}
	mt.rebuild(text)
}

// captureManualExpand records the user's current expand/collapse state keyed
// by dir relPath so it can be restored when the filter clears.
func (mt *MarkdownTree) captureManualExpand() {
	mt.manualExpand = map[string]bool{}
	rootNode := mt.tree.GetRoot()
	if rootNode == nil {
		return
	}
	rootNode.Walk(func(node, parent *tview.TreeNode) bool {
		if ref, ok := node.GetReference().(*mdNodeRef); ok && ref.isDir {
			mt.manualExpand[ref.relPath] = node.IsExpanded()
		}
		return true
	})
}

// rebuild constructs the TreeView nodes for the given filter query.
func (mt *MarkdownTree) rebuild(query string) {
	rootNode := tview.NewTreeNode("").SetReference(&mdNodeRef{isDir: true, relPath: ""})
	mt.tree.SetRoot(rootNode).SetCurrentNode(rootNode)

	if mt.scanned == nil || (len(mt.scanned.Dirs) == 0 && len(mt.scanned.Files) == 0) {
		rootNode.AddChild(disabledNode("(no markdown files)"))
		return
	}

	matched := matchingFiles(mt.scanned, query)
	if len(matched) == 0 {
		rootNode.AddChild(disabledNode("(no matches)"))
		return
	}
	forceExpand := expandedDirsForMatches(mt.scanned, matched)

	mt.addChildren(rootNode, mt.scanned, query, matched, forceExpand, 0)
	mt.selectFirstSelectable(rootNode)
}

func (mt *MarkdownTree) addChildren(parent *tview.TreeNode, dir *store.MarkdownDir, query string, matched, forceExpand map[string]bool, depth int) {
	for _, sub := range dir.Dirs {
		if query != "" && !forceExpand[sub.RelPath] {
			continue // prune dirs with no match under them
		}
		node := tview.NewTreeNode(sub.Name + "/").SetReference(&mdNodeRef{isDir: true, relPath: sub.RelPath})
		styleTreeNode(node)
		mt.applyExpansion(node, sub.RelPath, query, forceExpand, depth)
		parent.AddChild(node)
		mt.addChildren(node, sub, query, matched, forceExpand, depth+1)
	}
	for _, f := range dir.Files {
		if !matched[f.RelPath] {
			continue
		}
		node := tview.NewTreeNode(mt.fileLabel(f)).SetReference(&mdNodeRef{relPath: f.RelPath, absPath: f.AbsPath})
		styleTreeNode(node)
		parent.AddChild(node)
	}
}

// styleTreeNode applies the shared subdued color scheme to a folder or file
// node: TextSecondary base text, and a SurfaceSelection fill with TextPrimary
// text when the node is the current selection.
func styleTreeNode(node *tview.TreeNode) {
	roles := theme.Roles()
	node.SetColor(roles.TextSecondary().TCell())
	node.SetSelectedTextStyle(tcell.StyleDefault.
		Foreground(roles.TextPrimary().TCell()).
		Background(roles.SurfaceSelection().TCell()))
}

func (mt *MarkdownTree) fileLabel(f *store.MarkdownFile) string {
	if mt.isDoc != nil && mt.isDoc(f.AbsPath) {
		// the marker inherits the node color so it stays visible both at rest
		// (TextSecondary) and on the selection fill (TextPrimary). An inline
		// [color] tag would reset to the default fg and vanish when selected.
		return f.Name + " •"
	}
	return f.Name
}

func (mt *MarkdownTree) applyExpansion(node *tview.TreeNode, relPath, query string, forceExpand map[string]bool, depth int) {
	switch {
	case query != "":
		node.SetExpanded(forceExpand[relPath])
	case len(mt.manualExpand) > 0:
		node.SetExpanded(mt.manualExpand[relPath])
	default:
		node.SetExpanded(depth == 0) // top level expanded, deeper collapsed
	}
}

func disabledNode(label string) *tview.TreeNode {
	n := tview.NewTreeNode(label).SetSelectable(false)
	n.SetColor(theme.Roles().TextMuted().TCell())
	return n
}

func (mt *MarkdownTree) selectFirstSelectable(rootNode *tview.TreeNode) {
	var first *tview.TreeNode
	rootNode.Walk(func(node, parent *tview.TreeNode) bool {
		if first != nil {
			return false
		}
		if node == rootNode {
			return true
		}
		if _, ok := node.GetReference().(*mdNodeRef); ok {
			first = node
			return false
		}
		return true
	})
	if first != nil {
		mt.tree.SetCurrentNode(first)
	}
}

func (mt *MarkdownTree) handleFilterInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		mt.cfg.Cancel()
		return nil
	case tcell.KeyEnter:
		mt.activateCurrent()
		return nil
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight:
		mt.tree.InputHandler()(event, func(tview.Primitive) {})
		return nil
	case tcell.KeyCtrlU:
		mt.filterInput.SetText("")
		return nil
	case tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2:
		return event
	default:
		return nil
	}
}

func (mt *MarkdownTree) activateCurrent() {
	node := mt.tree.GetCurrentNode()
	if node == nil {
		return
	}
	ref, ok := node.GetReference().(*mdNodeRef)
	if !ok {
		return
	}
	if ref.isDir {
		node.SetExpanded(!node.IsExpanded())
		return
	}
	mt.cfg.Select(ref.relPath)
}

// --- test-only helpers ---

func (mt *MarkdownTree) selectFirstFileForTest() string {
	var found string
	mt.tree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		if found != "" {
			return false
		}
		if ref, ok := node.GetReference().(*mdNodeRef); ok && !ref.isDir {
			mt.tree.SetCurrentNode(node)
			found = ref.relPath
			return false
		}
		return true
	})
	return found
}

func (mt *MarkdownTree) pressEnterForTest() {
	mt.activateCurrent()
}

func (mt *MarkdownTree) setDirExpandedForTest(relPath string, expanded bool) {
	mt.tree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		if ref, ok := node.GetReference().(*mdNodeRef); ok && ref.isDir && ref.relPath == relPath {
			node.SetExpanded(expanded)
			return false
		}
		return true
	})
}

func (mt *MarkdownTree) dirExpandedForTest(relPath string) bool {
	var out bool
	mt.tree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		if ref, ok := node.GetReference().(*mdNodeRef); ok && ref.isDir && ref.relPath == relPath {
			out = node.IsExpanded()
			return false
		}
		return true
	})
	return out
}
