package types

import "time"

// AuthorInfo contains information about who created a file
type AuthorInfo struct {
	Name       string
	Email      string
	Date       time.Time
	CommitHash string
	Message    string
}

// FileVersion represents the content of a file at a specific commit
type FileVersion struct {
	Hash    string
	Author  string
	Email   string
	When    time.Time
	Content string
}
