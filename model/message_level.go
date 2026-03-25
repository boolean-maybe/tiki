package model

// MessageLevel identifies the severity of a statusline message.
// Determines which color pair is used when rendering the message.
type MessageLevel string

const (
	MessageLevelInfo  MessageLevel = "info"
	MessageLevelError MessageLevel = "error"
)
