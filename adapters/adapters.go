// Package adapters embeds the built-in adapter definitions so the rtdd binary ships
// them and stays a single static file. Spec D4 promises a host repo gains no runtime
// dependency from RTDD; an adapter YAML that had to be installed alongside the binary
// would be exactly such a dependency.
package adapters

import "embed"

// FS holds every built-in adapter declaration, read through adapter.Builtin.
//
//go:embed *.yaml
var FS embed.FS
