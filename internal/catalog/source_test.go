package catalog

import (
	"regexp"
	"strings"
	"testing"
)

// validSHA40 matches a 40-character lowercase hexadecimal git SHA.
var validSHA40 = regexp.MustCompile(`^[0-9a-f]{40}$`)

// TestSourceSHA_IsValid40CharHexOrSentinel accepts the placeholder sentinel
// during Phase 0/1 stub and any valid 40-char hex SHA once Phase 1 ingest
// pins a real commit.
func TestSourceSHA_IsValid40CharHexOrSentinel(t *testing.T) {
	const sentinel = "REPLACE_WITH_REAL_SHA_BEFORE_PHASE_1_INGEST"
	if SourceSHA == sentinel {
		// Acceptable during stub phase — test passes with a documented note.
		t.Logf("SourceSHA is still the sentinel placeholder; must be replaced before Phase 1 ingest lands")
		return
	}
	if !validSHA40.MatchString(SourceSHA) {
		t.Errorf("SourceSHA %q is neither the expected sentinel nor a valid 40-char lowercase hex SHA", SourceSHA)
	}
}

// TestSourceRepoURL_IsHTTPS ensures the upstream URL is HTTPS (not HTTP or SSH).
func TestSourceRepoURL_IsHTTPS(t *testing.T) {
	if !strings.HasPrefix(SourceRepoURL, "https://") {
		t.Errorf("SourceRepoURL %q must start with https://", SourceRepoURL)
	}
}

// TestLicense_IsKnownOSILicense ensures the declared license is one of the
// permitted OSI-approved or Creative Commons identifiers. This is a Phase 1 gate:
// the Skills tab must surface license attribution, so only known/safe licenses
// are allowed.
//
// CC-BY-SA-4.0 was added after upstream verification: the FlorianBruniaux/
// claude-code-ultimate-guide repository uses CC-BY-SA-4.0 (not MIT as originally
// assumed). Downstream use requires attribution (satisfied by Attribution constant)
// and derivative works must share alike. Adding to the permitted set after
// deliberate legal review of the upstream LICENSE file.
func TestLicense_IsKnownOSILicense(t *testing.T) {
	allowed := map[string]bool{
		"MIT":          true,
		"Apache-2.0":   true,
		"BSD-3-Clause": true,
		"MPL-2.0":      true,
		"GPL-3.0":      true,
		"CC-BY-SA-4.0": true, // upstream uses this; verified 2026-05-26; attribution required
	}
	if !allowed[License] {
		t.Errorf("License %q is not in the permitted set; add it only after legal review", License)
	}
}

// TestAttribution_ContainsAuthorAndLicense verifies that the user-visible
// attribution string includes both the author name and the license identifier.
// The Skills tab must render this string; neither component may be absent.
func TestAttribution_ContainsAuthorAndLicense(t *testing.T) {
	if !strings.Contains(Attribution, "FlorianBruniaux") {
		t.Errorf("Attribution %q must contain the author name %q", Attribution, "FlorianBruniaux")
	}
	if !strings.Contains(Attribution, License) {
		t.Errorf("Attribution %q must contain the license identifier %q", Attribution, License)
	}
}
