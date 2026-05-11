//go:build embed

package main

import _ "embed"

//go:embed network.json
var embeddedNetwork []byte

//go:embed story.json
var embeddedStory []byte
