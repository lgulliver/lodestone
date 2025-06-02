package goregistry

import "time"

// GoModInfo represents Go module information
type GoModInfo struct {
	Version string    `json:"Version"`
	Time    time.Time `json:"Time"`
	Origin  *GoOrigin `json:"Origin,omitempty"`
}

// GoOrigin represents information about the source of a Go module
type GoOrigin struct {
	VCS    string `json:"VCS"`
	URL    string `json:"URL"`
	Subdir string `json:"Subdir,omitempty"`
	Ref    string `json:"Ref,omitempty"`
	Hash   string `json:"Hash,omitempty"`
}
