// Package migrations embeds the SQL migration files shipped with the API binary.
package migrations

import "embed"

// FS contains every NNNN_name.sql migration.
//
//go:embed *.sql
var FS embed.FS
