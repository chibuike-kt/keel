// Package keel embeds keel's built-in module catalog. This is the only
// place it can live: go:embed cannot reach a directory outside its own
// package's tree, and modules/ sits at the repo root alongside this file.
package keel

import "embed"

//go:embed modules

// Modules is the embedded catalog of every module Keel ships, rooted at
// the modules/ directory. Consumers typically need fs.Sub(Modules, "modules")
// to get an fs.FS rooted correctly for catalog.LoadCatalog, since go:embed
// preserves the embedded directory's own name as a path component rather
// than flattening it.
var Modules embed.FS
