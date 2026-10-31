module github.com/sarathi/sarathi/backend

go 1.27

require github.com/jackc/pgx/v5 v5.7.2

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

// SANDBOX WORKAROUND (see docs/operations/known-limitations.md):
// The build sandbox could not reach proxy.golang.org / golang.org / gopkg.in,
// so these indirect modules are resolved from their GitHub mirrors.
// Remove this block and run `go mod tidy` on a machine with normal Go proxy access.
replace (
	golang.org/x/crypto => github.com/golang/crypto v0.31.0
	golang.org/x/sync => github.com/golang/sync v0.10.0
	golang.org/x/text => github.com/golang/text v0.21.0
	gopkg.in/check.v1 => github.com/go-check/check v0.0.0-20201130134442-10cb98267c6c
	gopkg.in/yaml.v3 => github.com/go-yaml/yaml/v3 v3.0.1
)
