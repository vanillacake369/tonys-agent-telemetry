// Package catalog holds the authoritative source metadata and minimum-viable
// threshold for the Phase 1 skill corpus ingest.
//
// SSoT: all upstream constants (repo URL, commit SHA, license, attribution)
// live in source.go. Nothing else in the codebase should redeclare them.
//
// Phase 1 ingest will add fetch logic here; this stub pre-stages the constants
// and policy so that Phase 1 only needs to plug in the HTTP fetch, not
// re-negotiate the source pin or the stale-cache threshold.
//
// See PIVOT_PLAN.md Phase 1 and GA gate for the full requirements that drive
// the constants and the [ResolveMinViable] function in this package.
package catalog
