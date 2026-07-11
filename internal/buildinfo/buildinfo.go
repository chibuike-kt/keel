// Package buildinfo exposes build-time version information.
package buildinfo

// Version is the keel release version. It is overridden at build time via
// -ldflags and defaults to "dev" for source builds.
var Version = "dev"
