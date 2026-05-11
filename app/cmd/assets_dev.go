//go:build !embed

package main

// embeddedNetwork and embeddedStory are nil in dev builds.
// The app falls back to reading network.json and story.json from disk.
var embeddedNetwork []byte
var embeddedStory []byte
