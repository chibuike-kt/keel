// Package keel embeds keel's built-in module catalog. This is the only
// place it can live: go:embed cannot reach a directory outside its own
// package's tree, and modules/ sits at the repo root alongside this file.
package keel

import "embed"

//go:embed modules
var Modules embed.FS
