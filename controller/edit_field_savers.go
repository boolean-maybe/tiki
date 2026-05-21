package controller

// TagsTextAreaSavable is the optional view-side hook used by the
// input router's field-aware Ctrl-S routing. The view implements it
// when its focused tags editor buffers input internally and only emits
// on explicit save (e.g. a textarea). Without the buffered-emit
// pattern, the view is free to omit this interface and the router's
// Ctrl-S falls through to the standard ActionDetailSave dispatch.
type TagsTextAreaSavable interface {
	SaveTagsFromTextArea()
}

// DescriptionTextAreaSavable mirrors TagsTextAreaSavable for the
// description editor. The configurable detail view (and the legacy
// TikiEditView while it still exists) implements this when description
// editing uses a textarea that buffers Ctrl-S internally.
type DescriptionTextAreaSavable interface {
	SaveDescriptionFromTextArea()
}
