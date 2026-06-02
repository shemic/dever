package frontcompiler

import "embed"

// FS contains the Dever front plugin compiler used by the CLI during
// development and plugin builds. Runtime front packages should not depend on it.
//
//go:embed package.json vite.config.ts src
var FS embed.FS
