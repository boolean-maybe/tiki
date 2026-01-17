package header

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/boolean-maybe/tiki/config"

	"github.com/rivo/tview"
)

// StatCollector allows components to register and manage dynamic stats
// displayed in the header's stats widget.
type StatCollector interface {
	// AddStat registers or updates a stat. Lower priority values display higher.
	// Returns false if the stat limit (6) is reached and key doesn't exist.
	AddStat(key, value string, priority int) bool

	// RemoveStat removes a stat by key. Returns true if stat existed.
	RemoveStat(key string) bool

	// GetStat retrieves current value for a stat. Returns empty string if not found.
	GetStat(key string) string
}

// statEntry represents a single stat in the widget
type statEntry struct {
	key      string
	value    string
	priority int
}

// StatsWidget displays application statistics dynamically
type StatsWidget struct {
	*tview.TextView

	stats    map[string]*statEntry // key -> entry for O(1) lookup
	sorted   []*statEntry          // sorted by priority for rendering
	mu       sync.RWMutex          // thread safety
	maxStats int                   // fixed at 6
}

// NewStatsWidget creates a new stats display widget
func NewStatsWidget() *StatsWidget {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetTextAlign(tview.AlignLeft)

	sw := &StatsWidget{
		TextView: tv,
		stats:    make(map[string]*statEntry),
		sorted:   make([]*statEntry, 0, 6),
		maxStats: 6,
	}

	return sw
}

// AddStat registers or updates a stat. Lower priority values display higher.
// Returns false if the stat limit (6) is reached and key doesn't exist.
func (sw *StatsWidget) AddStat(key, value string, priority int) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Check if key exists (update case)
	if entry, exists := sw.stats[key]; exists {
		entry.value = value
		entry.priority = priority
		sw.rebuildSorted()
		sw.update()
		return true
	}

	// Check limit for new key
	if len(sw.stats) >= sw.maxStats {
		return false
	}

	// Add new entry
	entry := &statEntry{
		key:      key,
		value:    value,
		priority: priority,
	}
	sw.stats[key] = entry
	sw.rebuildSorted()
	sw.update()
	return true
}

// RemoveStat removes a stat by key. Returns true if stat existed.
func (sw *StatsWidget) RemoveStat(key string) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if _, exists := sw.stats[key]; !exists {
		return false
	}

	delete(sw.stats, key)
	sw.rebuildSorted()
	sw.update()
	return true
}

// GetStat retrieves current value for a stat. Returns empty string if not found.
func (sw *StatsWidget) GetStat(key string) string {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if entry, exists := sw.stats[key]; exists {
		return entry.value
	}
	return ""
}

// GetKeys returns all current stat keys
func (sw *StatsWidget) GetKeys() []string {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	keys := make([]string, 0, len(sw.stats))
	for k := range sw.stats {
		keys = append(keys, k)
	}
	return keys
}

// Primitive returns the underlying tview primitive
func (sw *StatsWidget) Primitive() tview.Primitive {
	return sw.TextView
}

// rebuildSorted rebuilds the sorted slice from the map (must be called with lock held)
func (sw *StatsWidget) rebuildSorted() {
	sw.sorted = make([]*statEntry, 0, len(sw.stats))
	for _, entry := range sw.stats {
		sw.sorted = append(sw.sorted, entry)
	}
	sort.Slice(sw.sorted, func(i, j int) bool {
		return sw.sorted[i].priority < sw.sorted[j].priority
	})
}

// update refreshes the stats display (must be called with lock held)
func (sw *StatsWidget) update() {
	if len(sw.sorted) == 0 {
		sw.SetText("")
		return
	}

	// find max label length for value alignment
	maxLabelLen := 0
	for _, entry := range sw.sorted {
		if len(entry.key) > maxLabelLen {
			maxLabelLen = len(entry.key)
		}
	}

	colors := config.GetColors()

	var lines []string
	for _, entry := range sw.sorted {
		// pad after colon to align values
		padding := strings.Repeat(" ", maxLabelLen-len(entry.key))
		lines = append(lines, fmt.Sprintf("%s%s:%s%s %s", colors.HeaderInfoLabel, entry.key, colors.HeaderInfoValue, padding, entry.value))
	}

	sw.SetText(strings.Join(lines, "\n"))
}
