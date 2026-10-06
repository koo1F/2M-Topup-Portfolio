// Package migrate embeds SQL migration files into the binary so that
// no external migration directory is needed at runtime (works in Docker too).
package migrate

import "embed"

// FS holds all migration SQL files embedded at compile time.
//
//go:embed sql
var FS embed.FS
