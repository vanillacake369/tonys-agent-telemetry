package catalog

// SourceRepoURL is the upstream best-practice corpus.
// Phase 1 ingest fetches from this repository; do NOT change this to a fork or
// mirror without updating SourceSHA and re-running the attribution check.
const SourceRepoURL = "https://github.com/FlorianBruniaux/claude-code-ultimate-guide"

// SourceSHA pins the corpus to a specific commit. Update this constant via a
// deliberate PR — never auto-track the main branch. Pinning prevents unexpected
// catalog drift between releases. Per PIVOT_PLAN Phase 1 gate.
//
// Pinned commit: 37e9335457b829b8b307c12e0b8cbdf42be7cd8b
// Author: Florian BRUNIAUX — date: 2026-05-24
// Message: "docs(cca-coverage): add 7 new sections covering CCA exam gaps"
// Verified: 181 entries (23 agents, 64 skills, 37 hooks, 57 templates)
const SourceSHA = "37e9335457b829b8b307c12e0b8cbdf42be7cd8b"

// License declares the upstream SPDX license identifier for the corpus.
// Phase 1 ingest must surface this in the Skills tab UI alongside Attribution.
//
// IMPORTANT: The upstream repository uses CC-BY-SA-4.0, NOT MIT.
// Verified against: https://github.com/FlorianBruniaux/claude-code-ultimate-guide/blob/main/LICENSE
// CC-BY-SA-4.0 is a copyleft ShareAlike license; downstream use requires attribution
// and derivative works must be shared under the same license.
// The Attribution constant satisfies the BY (attribution) requirement.
const License = "CC-BY-SA-4.0"

// Attribution is the user-visible string the Skills tab must render whenever
// catalog entries are displayed. It acknowledges both the author and the license
// to satisfy the upstream CC-BY-SA-4.0 terms and inform users of the content origin.
const Attribution = "Best-practice corpus: claude-code-ultimate-guide by FlorianBruniaux (CC-BY-SA-4.0)"
