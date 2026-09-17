// PATCHES.md is this fork's provenance record, and it has been WRONG in published tags: it named
// the wrong upstream base (v0.6.8 for a tree based on v0.6.9), described a `replace` directive and a
// third_party/ tree that never existed here, claimed "0 test files" for a tree carrying 82, printed
// line-number citations that had drifted by up to 120 lines, gave two different sections the number
// 8 while leaving 6 unused, labelled the patch-5 comment in http2/client_conn_pool.go "PATCH 3",
// pointed at a file called FHTTP_LAYER_PATCH.md that has never existed, and asserted that the
// inverted cancelStream() condition leaked nothing when it leaked one clientStream and one of the
// connection's 100 concurrency slots per context-cancelled request.
//
// Prose is not self-guarding, so this file guards it. Every check below re-derives a fact from the
// TREE and fails if PATCHES.md stopped describing it. The layer is deliberate: the product defects
// these patches fix are guarded by the tests each patch entry names, and on Sightglass's shipped
// path (sightglass.NewSessionFactory -> Session.Do) by parity.TestParityHTTP2*; what was unguarded
// was the record itself, which is what this file is for.
//
// Run just these: go test . -run TestPatchesMD

package http_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const patchesFile = "PATCHES.md"

// patchesDoc reads PATCHES.md from the package directory, which is this fork's root.
func patchesDoc(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(patchesFile)
	if err != nil {
		t.Fatalf("PATCHES.md is this fork's provenance record and it must exist at the tree root: %v", err)
	}
	return string(b)
}

// gitDiffNameStatus returns base->working-tree status letters keyed by path, or ok=false with a
// reason when this tree is not a git checkout (a consumer that got the module from the proxy has no
// .git, and only THIS check needs one — every other check in the file runs regardless).
func gitDiffNameStatus(t *testing.T, base string) (map[string]string, bool, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		return nil, false, "git is not on PATH"
	}
	if out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").CombinedOutput(); err != nil ||
		strings.TrimSpace(string(out)) != "true" {
		return nil, false, "this copy of the module is not a git checkout (no history to diff against the base)"
	}
	if err := exec.Command("git", "cat-file", "-e", base+"^{commit}").Run(); err != nil {
		t.Fatalf("PATCHES.md names upstream base commit %s, which does not exist in this repository. "+
			"The base is the one fact everything else in that file is relative to; if it is wrong, every "+
			"diff, file count and hunk count below it is unverifiable.", base)
	}
	if err := exec.Command("git", "merge-base", "--is-ancestor", base, "HEAD").Run(); err != nil {
		t.Fatalf("PATCHES.md names upstream base commit %s, but it is NOT an ancestor of HEAD. "+
			"This fork is supposed to be that commit plus the patches documented in PATCHES.md.", base)
	}
	out, err := exec.Command("git", "diff", "--name-status", base, "--").Output()
	if err != nil {
		t.Fatalf("git diff --name-status %s failed: %v", base, err)
	}
	m := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			m[f[len(f)-1]] = f[0][:1]
		}
	}
	return m, true, ""
}

// forkPathRewrite matches the only change the 70 "rename-only" files carry.
var forkPathRewrite = regexp.MustCompile(`bogdanfinn|Berserk-Automation-Hub`)

// substantivelyModified reports which of the modified files carry a change that is NOT just the
// module-path rewrite.
func substantivelyModified(t *testing.T, base string, status map[string]string) map[string]bool {
	t.Helper()
	sub := map[string]bool{}
	for path, st := range status {
		if st != "M" {
			continue
		}
		out, err := exec.Command("git", "diff", "-U0", base, "--", path).Output()
		if err != nil {
			t.Fatalf("git diff -U0 %s -- %s: %v", base, path, err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			if len(line) == 0 || (line[0] != '+' && line[0] != '-') {
				continue
			}
			if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
				continue
			}
			if forkPathRewrite.MatchString(line) {
				continue
			}
			sub[path] = true
			break
		}
	}
	return sub
}

var baseCommitRE = regexp.MustCompile(`commit ` + "`" + `([0-9a-f]{40})` + "`")

func docBaseCommit(t *testing.T, doc string) string {
	t.Helper()
	m := baseCommitRE.FindStringSubmatch(doc)
	if m == nil {
		t.Fatal("PATCHES.md no longer names the upstream base commit as a 40-character sha in backticks. " +
			"Without it nothing else in the file can be checked against the tree, which is exactly how it " +
			"came to claim a v0.6.8 base for a v0.6.9 fork.")
	}
	return m[1]
}

// docManifest parses the `A path` / `M path` lines out of the "## Files touched" fenced blocks.
func docManifest(t *testing.T, doc string) (added, modified map[string]bool, renameOnlyCount int) {
	t.Helper()
	added, modified = map[string]bool{}, map[string]bool{}
	sec := section(t, doc, "## Files touched")
	for _, line := range strings.Split(sec, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "A":
			added[f[1]] = true
		case "M":
			modified[f[1]] = true
		}
	}
	m := regexp.MustCompile(`### Modified by the module-path rewrite ONLY \((\d+)\)`).FindStringSubmatch(sec)
	if m == nil {
		t.Fatal(`PATCHES.md's "Files touched" section no longer states how many files are modified by the ` +
			`module-path rewrite alone. That count is what separates "86 files modified" from "16 files we ` +
			`actually changed", and the two have been conflated before.`)
	}
	renameOnlyCount, _ = strconv.Atoi(m[1])
	return added, modified, renameOnlyCount
}

// section returns the text from the given `## ` heading up to the next `## ` heading.
func section(t *testing.T, doc, heading string) string {
	t.Helper()
	i := strings.Index(doc, "\n"+heading+"\n")
	if i < 0 {
		t.Fatalf("PATCHES.md has no %q section", heading)
	}
	rest := doc[i+1+len(heading):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func diffSets(want, got map[string]bool) (missing, extra []string) {
	for k := range want {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	for k := range got {
		if !want[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return
}

// TestPatchesMDFileManifestMatchesTheTree re-derives the "Files touched" manifest from git and fails
// if PATCHES.md does not match it exactly.
func TestPatchesMDFileManifestMatchesTheTree(t *testing.T) {
	doc := patchesDoc(t)
	base := docBaseCommit(t, doc)
	status, ok, why := gitDiffNameStatus(t, base)
	if !ok {
		t.Skipf("the file manifest is checked against `git diff %s`, and %s. "+
			"Every other check in this file still ran.", base[:7], why)
	}
	docAdded, docModified, docRenameOnly := docManifest(t, doc)

	realAdded := map[string]bool{}
	for p, st := range status {
		if st == "A" {
			realAdded[p] = true
		}
	}
	realSub := substantivelyModified(t, base, status)
	realRenameOnly := 0
	for p, st := range status {
		if st == "M" && !realSub[p] {
			realRenameOnly++
		}
	}

	if missing, extra := diffSets(realAdded, docAdded); len(missing) > 0 || len(extra) > 0 {
		t.Errorf("PATCHES.md's ADDED manifest does not match the tree.\n"+
			"  files this fork adds but PATCHES.md does not list: %v\n"+
			"  files PATCHES.md lists as added that this fork does not add: %v\n"+
			"An undocumented added file is an undocumented patch; a documented file that is not there is a "+
			"patch that has silently been dropped.", missing, extra)
	}
	if missing, extra := diffSets(realSub, docModified); len(missing) > 0 || len(extra) > 0 {
		t.Errorf("PATCHES.md's MODIFIED manifest does not match the tree.\n"+
			"  files changed beyond the module-path rewrite but not listed: %v\n"+
			"  files listed as changed that carry only the rewrite (or no change): %v\n"+
			"tree has %d substantively modified files, PATCHES.md lists %d",
			missing, extra, len(realSub), len(docModified))
	}
	if realRenameOnly != docRenameOnly {
		t.Errorf("PATCHES.md says %d files are modified by the module-path rewrite ONLY; the tree has %d. "+
			"That number is what keeps 'we changed 86 files' from being read as 86 patches.",
			docRenameOnly, realRenameOnly)
	}
}

var (
	// Markers in the source. `6+10` means one site serves two patches.
	markerRE = regexp.MustCompile(`SIGHTGLASS PATCH (\d+b?(?:\+\d+b?)*)`)
	// `## Patch 11 — ...` headings and `| 11 | ... |` rows of the numbering table.
	headingRE  = regexp.MustCompile(`(?m)^## Patch (\d+b?) [—-]`)
	tableRowRE = regexp.MustCompile(`(?m)^\| (\d+b?)\s*\|`)
)

func markerNumbers(t *testing.T) map[string]bool {
	t.Helper()
	nums := map[string]bool{}
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		// This file talks ABOUT the markers; it must not be counted as carrying them.
		if !strings.HasSuffix(path, ".go") || filepath.Base(path) == "patches_doc_test.go" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range markerRE.FindAllStringSubmatch(string(b), -1) {
			for _, n := range strings.Split(m[1], "+") {
				nums[n] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree for patch markers: %v", err)
	}
	return nums
}

// TestPatchesMDPatchNumbersMatchTheSourceMarkers pins the three places a patch number appears — the
// `[SIGHTGLASS PATCH n]` markers in the source, the `## Patch n` headings, and the numbering table —
// against each other.
func TestPatchesMDPatchNumbersMatchTheSourceMarkers(t *testing.T) {
	doc := patchesDoc(t)

	headings := map[string]bool{}
	for _, m := range headingRE.FindAllStringSubmatch(doc, -1) {
		headings[m[1]] = true
	}
	table := map[string]bool{}
	for _, m := range tableRowRE.FindAllStringSubmatch(section(t, doc, "## Patch numbering"), -1) {
		table[m[1]] = true
	}
	markers := markerNumbers(t)

	if len(headings) == 0 || len(table) == 0 || len(markers) == 0 {
		t.Fatalf("expected patch numbers in all three places; got %d headings, %d table rows, %d markers",
			len(headings), len(table), len(markers))
	}
	if missing, extra := diffSets(headings, table); len(missing) > 0 || len(extra) > 0 {
		t.Errorf("the numbering table in PATCHES.md disagrees with its own `## Patch n` headings.\n"+
			"  headings with no table row: %v\n  table rows with no heading: %v\n"+
			"headings=%v table=%v", missing, extra, sortedKeys(headings), sortedKeys(table))
	}
	if missing, extra := diffSets(headings, markers); len(missing) > 0 || len(extra) > 0 {
		t.Errorf("PATCHES.md and the source markers disagree about which patches exist.\n"+
			"  documented patches with NO [SIGHTGLASS PATCH n] marker in any .go file: %v\n"+
			"    (a patch nothing marks is a patch the next upstream merge will silently drop)\n"+
			"  markers in the source with NO section in PATCHES.md: %v\n"+
			"    (an unnumbered or mis-numbered marker is how the patch-5 comment in "+
			"http2/client_conn_pool.go came to read \"PATCH 3\")\n"+
			"documented=%v marked=%v", missing, extra, sortedKeys(headings), sortedKeys(markers))
	}
}

var goTestNameRE = regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9_]*`)

// runPatternRE finds the argument of every `go test -run` in the document, quoted or bare.
var runPatternRE = regexp.MustCompile(`-run\s+(?:'([^']+)'|"([^"]+)"|(\S+))`)

// runPatterns returns every alternative of every `-run` argument in the document. These are
// PATTERNS, not test names: `-run 'TestParityHTTP2'` legitimately names no function.
func runPatterns(doc string) map[string]bool {
	out := map[string]bool{}
	for _, m := range runPatternRE.FindAllStringSubmatch(doc, -1) {
		arg := m[1] + m[2] + m[3]
		for _, alt := range strings.Split(arg, "|") {
			out[strings.Trim(alt, "^$")] = true
		}
	}
	return out
}

// TestPatchesMDNamesGuardsThatExist checks that every Go test PATCHES.md names as a guard is a test
// that actually exists in this fork. `parity.*` guards live in Sightglass, not here.
func TestPatchesMDNamesGuardsThatExist(t *testing.T) {
	doc := patchesDoc(t)

	external := map[string]bool{}
	for _, m := range regexp.MustCompile(`\b(?:parity|tlsemu|sightglass)\.(Test[A-Za-z0-9_]+)`).
		FindAllStringSubmatch(section(t, doc, "## Where each patch is guarded"), -1) {
		external[m[1]] = true
	}

	have := map[string]bool{}
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`).FindAllStringSubmatch(string(b), -1) {
			have[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree for test functions: %v", err)
	}

	var ghosts []string
	for _, name := range goTestNameRE.FindAllString(doc, -1) {
		if have[name] {
			continue
		}
		// Guards that live in Sightglass rather than here. They must be declared, by name, in the
		// "Guards that live in Sightglass" section — naming a test that exists in neither place is the
		// same claim as an ablation that was never run.
		if external[name] {
			continue
		}
		// Prefixed inline elsewhere in the prose as parity.X / sightglass.X: also a declaration that it
		// lives in the consumer.
		if strings.Contains(doc, "parity."+name) || strings.Contains(doc, "sightglass."+name) ||
			strings.Contains(doc, "tlsemu."+name) {
			continue
		}
		// A `go test -run` PATTERN rather than a test name. Both conditions are required: it has to be
		// written as a -run argument in this file AND it has to match at least one test that really is
		// in this tree, so "-run TestNothingLikeThis" is still a ghost.
		if runPatterns(doc)[name] {
			matches := false
			for real := range have {
				if real != name && strings.HasPrefix(real, name) {
					matches = true
					break
				}
			}
			if matches {
				continue
			}
		}
		ghosts = append(ghosts, name)
	}
	sort.Strings(ghosts)
	if len(ghosts) > 0 {
		t.Errorf("PATCHES.md names %d test(s) as guards that do not exist in this fork and are not marked as "+
			"living in Sightglass: %v\nA named guard that does not exist is the same claim as an ablation that "+
			"was never run.", len(ghosts), unique(ghosts))
	}
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// testFileSplitRE reads the front matter's test-file sentence. It matches against a
// whitespace-collapsed copy of the document so that re-wrapping a paragraph cannot silently stop the
// guard from finding the sentence it guards — the previous version embedded a literal "\n" at the
// wrap point, which made the check hostage to a line break.
var testFileSplitRE = regexp.MustCompile(`\*\*(\d+) ` + "`" + `\*_test\.go` + "`" +
	` files: (\d+) from upstream \S+, of which (\d+) are byte-identical to upstream, ` +
	`(\d+) carry nothing but the module-path rewrite and (\d+) are edited, ` +
	`plus (\d+) added by this fork\.\*\*`)

// docTestFileSplit returns total, upstream, byte-identical, rewrite-only, edited, added as PATCHES.md
// states them.
func docTestFileSplit(t *testing.T, doc string) (total, upstream, identical, rewriteOnly, edited, added int) {
	t.Helper()
	m := testFileSplitRE.FindStringSubmatch(strings.Join(strings.Fields(doc), " "))
	if m == nil {
		t.Fatal(`PATCHES.md no longer states how many *_test.go files this tree carries, how many come from ` +
			`upstream, how the upstream ones split between byte-identical, rewrite-only and edited, and how ` +
			`many the fork adds. That sentence exists because its predecessor ("0 test files") was false and ` +
			`the Maintenance section turned it into an instruction to delete them — and because its ` +
			`successor ("68 carry nothing but the module-path rewrite") was false too.`)
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	return n(1), n(2), n(3), n(4), n(5), n(6)
}

// TestPatchesMDTestFileSplitIsDerivedFromTheTree measures the split the front matter states, instead
// of checking that it adds up.
//
// The previous guard checked `untouched + edited == upstream` and nothing else. That is an identity,
// not a measurement: it was green for four published tags while the sentence said "68 carry nothing
// but the module-path rewrite and 5 are edited" about a tree where 68 is not the count of anything.
// Only 45 of the 73 upstream test files appear in the diff against the base at all; the remaining 28
// are byte-identical to upstream because they never mention the module path, and 45 − 5 substantively
// edited leaves 40 carrying only the rewrite. The correct split has three parts, so the sentence and
// this guard now both have three.
func TestPatchesMDTestFileSplitIsDerivedFromTheTree(t *testing.T) {
	doc := patchesDoc(t)
	base := docBaseCommit(t, doc)
	_, upstream, identical, rewriteOnly, edited, added := docTestFileSplit(t, doc)

	status, ok, why := gitDiffNameStatus(t, base)
	if !ok {
		t.Skipf("the test-file split is measured against `git diff %s`, and %s. "+
			"Every other check in this file still ran; this is the only one that cannot.", base[:7], why)
	}

	upstreamTests, err := exec.Command("git", "ls-tree", "-r", "--name-only", base).Output()
	if err != nil {
		t.Fatalf("git ls-tree -r --name-only %s: %v", base, err)
	}
	realUpstream := 0
	for _, p := range strings.Split(string(upstreamTests), "\n") {
		if strings.HasSuffix(p, "_test.go") {
			realUpstream++
		}
	}

	sub := substantivelyModified(t, base, status)
	realEdited, realRewriteOnly, realAdded := 0, 0, 0
	for p, st := range status {
		if !strings.HasSuffix(p, "_test.go") {
			continue
		}
		switch {
		case st == "A":
			realAdded++
		case st == "M" && sub[p]:
			realEdited++
		case st == "M":
			realRewriteOnly++
		}
	}
	realIdentical := realUpstream - realEdited - realRewriteOnly

	t.Logf("measured against %s: %d upstream test files = %d byte-identical + %d rewrite-only + %d edited; "+
		"%d added by this fork", base[:7], realUpstream, realIdentical, realRewriteOnly, realEdited, realAdded)

	for _, c := range []struct {
		what      string
		doc, tree int
		why       string
	}{
		{"test files kept from upstream", upstream, realUpstream,
			"This is the count the retracted \"strip tests\" instruction would have destroyed."},
		{"upstream test files byte-identical to upstream", identical, realIdentical,
			"A file that never mentions the module path is not in the diff at all. Counting it as " +
				"\"carrying the rewrite\" is what produced the false 68."},
		{"upstream test files carrying ONLY the module-path rewrite", rewriteOnly, realRewriteOnly,
			"This is the number that separates \"we renamed a package\" from \"we changed a test\"."},
		{"upstream test files this fork edits substantively", edited, realEdited,
			"Which upstream tests this fork touched is the difference between \"we kept upstream's " +
				"suite\" and \"we kept the parts of it that still pass\"."},
		{"test files added by this fork", added, realAdded,
			"An undocumented added test file is an undocumented guard."},
	} {
		if c.doc != c.tree {
			t.Errorf("PATCHES.md states the number of %s as %d; the tree has %d.\n%s",
				c.what, c.doc, c.tree, c.why)
		}
	}
}

// TestPatchesMDTestFileCountIsTrue pins the count in the front matter. The claim it replaced —
// "stripped of *_test.go ... 0 test files" — was not merely stale: the retained upstream suite is
// what found patch 4b, so a future maintainer who believed it would delete the thing that catches
// the defects.
func TestPatchesMDTestFileCountIsTrue(t *testing.T) {
	doc := patchesDoc(t)
	total, upstream, identical, rewriteOnly, edited, added := docTestFileSplit(t, doc)
	if identical+rewriteOnly+edited != upstream {
		t.Errorf("PATCHES.md splits the %d upstream test files into %d byte-identical + %d rewrite-only "+
			"+ %d edited, which is %d. Note that this is only ARITHMETIC: the identity it checks was "+
			"green for four tags while the split it checked was a fabrication (68+5 for a tree whose "+
			"real split is 40+28+5). TestPatchesMDTestFileSplitIsDerivedFromTheTree is what measures it.",
			upstream, identical, rewriteOnly, edited, identical+rewriteOnly+edited)
	}

	real := 0
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			real++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("counting test files: %v", err)
	}
	if real != total {
		t.Errorf("PATCHES.md says this tree carries %d *_test.go files; it carries %d. "+
			"Upstream's suite is kept on purpose — it is what found patch 4b — so this count is a load-bearing "+
			"claim, not decoration.", total, real)
	}
	if upstream+added != total {
		t.Errorf("PATCHES.md's test-file arithmetic does not add up: %d upstream + %d added != %d total",
			upstream, added, total)
	}

	docAdded, docModified, _ := docManifest(t, doc)
	addedTests, editedTests := 0, 0
	for p := range docAdded {
		if strings.HasSuffix(p, "_test.go") {
			addedTests++
		}
	}
	for p := range docModified {
		if strings.HasSuffix(p, "_test.go") {
			editedTests++
		}
	}
	if addedTests != added {
		t.Errorf("PATCHES.md says this fork adds %d test files, but its own ADDED manifest lists %d",
			added, addedTests)
	}
	if editedTests != edited {
		t.Errorf("PATCHES.md says %d upstream test files are edited, but its own MODIFIED manifest lists %d. "+
			"Which upstream tests this fork touched is the difference between \"we kept upstream's suite\" "+
			"and \"we kept the parts of it that still pass\".", edited, editedTests)
	}

	// EVERY occurrence, not only the sentence above. This check used to parse the front-matter
	// sentence and nothing else, and while it was green the RETRACTED CLAIMS section 18 lines below
	// corrected a false count ("0 test files") with another false count ("the tree carries 81"), and
	// the string "FALSE. The tree carries 81" was additionally hard-coded in THIS FILE as the `why`
	// of that retraction. A guard that pins one occurrence of a number pins the number nowhere.
	checked := 0
	checked += checkCounts(t, patchesFile, doc, totalTestFilesRE, real,
		"the number of *_test.go files this tree carries",
		"Upstream's suite is kept on purpose — it is what found patch 4b — so every statement of this "+
			"count is load-bearing, including the ones inside the RETRACTED CLAIMS section.")
	checked += checkCounts(t, patchesFile, doc, treeCarriesRE, real,
		"the number of *_test.go files this tree carries",
		"A retraction section that corrects a false count with a false count is the same defect one "+
			"layer up.")
	checked += checkCounts(t, patchesFile, doc, upstreamTestFilesRE, upstream,
		"the number of test files kept from upstream v0.6.9",
		"This is the count the retracted \"strip tests\" instruction would have destroyed.")
	checked += checkCounts(t, patchesFile, doc, fromUpstreamRE, upstream,
		"the number of test files kept from upstream v0.6.9",
		"This is the count the retracted \"strip tests\" instruction would have destroyed.")
	// The sweep also has to read THIS FILE's own prose. The `why` text of the "0 test files"
	// retraction hard-coded the string "FALSE. The tree carries 81, and the retained upstream suite
	// is what found patch 4b." — the guard that PATCHES.md credits with pinning the count was
	// itself publishing the wrong count, and printing it in its own failure messages.
	for _, r := range retracted {
		checked += checkCounts(t, "patches_doc_test.go `why`", r.why, treeCarriesRE, real,
			"the number of *_test.go files this tree carries, in the `why` of retracted claim "+
				strconv.Quote(r.claim),
			"This text is what the guard prints when it fails, so a wrong count here is a wrong count "+
				"handed to the next reader at the exact moment they are trusting the guard.")
		checked += checkCounts(t, "patches_doc_test.go `why`", r.why, upstreamTestFilesRE, upstream,
			"the number of test files kept from upstream, in the `why` of retracted claim "+
				strconv.Quote(r.claim),
			"Same reason: the correction text is published by the guard itself.")
	}

	if checked < 4 {
		t.Errorf("the test-file-count sweep matched only %d count claims in PATCHES.md. It is supposed to "+
			"read the front-matter sentence, the retraction of \"0 test files\", and the Maintenance "+
			"section's \"Upstream's N test files are KEPT\" — if the prose has been reshaped, reshape the "+
			"sweep with it rather than letting it pass on nothing.", checked)
	}
}

// numberWords is here because every count an adversarial reader found wrong in
// v0.6.9-sightglass.17 was spelled out rather than written in digits — "ten guards", "Seven are
// new", "the nine guards this fork adds", "the nine files this fork adds to the root package" —
// and every check in this file only ever looked at digits. A count that no guard can read is a
// count nobody checks, and four of them were wrong at once.
var numberWords = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7,
	"eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14,
	"fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20,
	"twenty-four": 24, "twenty-five": 25, "twenty-six": 26,
}

// countIn reads a count written as digits or as an English word. ok is false when the token is not
// a count at all, which is how the sweeps below tolerate a regex that also matches ordinary prose.
func countIn(s string) (int, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, true
	}
	n, ok := numberWords[strings.ToLower(s)]
	return n, ok
}

// countTok matches a count written either way. The `-` alternative is for "twenty-four".
const countTok = `(\d+|[A-Za-z]+(?:-[A-Za-z]+)?)`

var (
	totalTestFilesRE    = regexp.MustCompile(countTok + " `\\*_test\\.go` files")
	treeCarriesRE       = regexp.MustCompile(`[Tt]he tree carries \*{0,2}` + countTok)
	upstreamTestFilesRE = regexp.MustCompile(`[Uu]pstream'?s ` + countTok + ` test files`)
	fromUpstreamRE      = regexp.MustCompile(countTok + ` from upstream v`)
	addedToRootPkgRE    = regexp.MustCompile(countTok + ` files this fork adds to\s+the root package`)

	productAndGuardsRE = regexp.MustCompile(countTok + ` product files and ` + countTok + ` guards`)
	newWithPatchRE     = regexp.MustCompile(countTok + ` are new with the patch they guard`)
	numberedMarkersRE  = regexp.MustCompile(`all ` + countTok + ` files that carry a numbered`)
	allMarkersRE       = regexp.MustCompile(`lists all ` + countTok + ` marked files`)
	guardsOfGuardsRE   = regexp.MustCompile(countTok + ` of the ` + countTok + ` guards this fork adds`)
	patch4bFilesRE     = regexp.MustCompile(`the ` + countTok + ` upstream files patch 4b edits`)
)

// lineAndSnippet locates a byte offset in some text for a failure message. Line numbers are banned
// in PATCHES.md's own prose (TestPatchesMDCitesNoLineNumbersIntoThisTree) because they drift; in a
// failure message they are computed at the moment of failure and are the fastest way to the sentence.
func lineAndSnippet(where, doc string, off int) string {
	line := strings.Count(doc[:off], "\n") + 1
	lo := strings.LastIndexByte(doc[:off], '\n') + 1
	hi := off
	for n := 0; n < 2 && hi < len(doc); n++ {
		j := strings.IndexByte(doc[hi+1:], '\n')
		if j < 0 {
			hi = len(doc)
			break
		}
		hi += j + 1
	}
	return where + ":" + strconv.Itoa(line) + ": " + squash(doc[lo:hi])
}

// checkCounts requires capture group 1 of every match of re to equal want, and returns how many
// count claims it actually read so the caller can refuse to pass on zero. `where` names the text
// being swept, because this sweep reads THIS FILE's own prose as well as PATCHES.md and a failure
// message that says the wrong one sends the reader to the wrong place.
func checkCounts(t *testing.T, where, doc string, re *regexp.Regexp, want int, what, why string) int {
	t.Helper()
	n := 0
	for _, loc := range re.FindAllStringSubmatchIndex(doc, -1) {
		tok := doc[loc[2]:loc[3]]
		got, ok := countIn(tok)
		if !ok {
			continue // the regex also matches ordinary prose; that is not a count claim
		}
		n++
		if got != want {
			t.Errorf("%s states %s as %q; it is %d.\n  %s\n%s",
				where, what, tok, want, lineAndSnippet(where, doc, loc[0]), why)
		}
	}
	return n
}

// TestPatchesMDGuardAndMarkerCountsAreTrue pins the counts that describe THIS FORK'S OWN additions:
// how many guards it adds, how many of them are new with the patch they guard, how many files carry
// a numbered marker, and how many the Maintenance grep returns. All four were wrong at
// v0.6.9-sightglass.17 and all four were spelled in words, which is why nothing caught them.
func TestPatchesMDGuardAndMarkerCountsAreTrue(t *testing.T) {
	doc := patchesDoc(t)
	docAdded, docModified, _ := docManifest(t, doc)

	addedGuards, addedProduct, addedRootPkgTests := 0, 0, 0
	for p := range docAdded {
		switch {
		case strings.HasSuffix(p, "_test.go"):
			addedGuards++
			if !strings.Contains(p, "/") {
				addedRootPkgTests++
			}
		case strings.HasSuffix(p, ".go"):
			addedProduct++
		}
	}

	markers := markerNumbersByFile(t)
	guardsWithAPatchNumber := 0
	for p := range docAdded {
		if strings.HasSuffix(p, "_test.go") && len(markers[p]) > 0 {
			guardsWithAPatchNumber++
		}
	}

	numbered := len(markers)
	allMarked := filesCarryingAnyMarker(t)

	patch4bUpstream := 0
	for p, ps := range docFilePatches(t, doc) {
		if ps["4b"] && docModified[p] {
			patch4bUpstream++
		}
	}

	checked := 0
	checked += checkCounts(t, patchesFile, doc, addedToRootPkgRE, addedRootPkgTests,
		"the number of test files this fork adds to the ROOT package",
		"The other added guards live in http2/ and http2/hpack/ and cannot affect TestOmitHTTP2's "+
			"package at all, so quoting the whole added set here makes the regression diff say something "+
			"it did not measure.")
	checked += checkCounts(t, patchesFile, doc, newWithPatchRE, guardsWithAPatchNumber,
		"the number of added guards that are new WITH the patch they guard",
		"patches_doc_test.go guards this document rather than a numbered patch, so it is not one of "+
			"them; markerNumbersByFile is what decides.")
	checked += checkCounts(t, patchesFile, doc, numberedMarkersRE, numbered,
		"the number of files carrying a NUMBERED [SIGHTGLASS PATCH n] comment",
		"Maintenance step 4 sends the next maintainer to this number; if it is wrong they stop looking "+
			"before they have seen every patch site.")
	checked += checkCounts(t, patchesFile, doc, allMarkersRE, allMarked,
		"the number of files the Maintenance grep for SIGHTGLASS PATCH actually returns",
		"patches_doc_test.go carries an UNnumbered marker, so the grep returns one more file than the "+
			"numbered count. A maintainer who diffs the two and finds a discrepancy stops trusting both.")
	checked += checkCounts(t, patchesFile, doc, patch4bFilesRE, patch4bUpstream,
		"the number of upstream files patch 4b edits",
		"Derived from the manifest's own patch attribution crossed with git's M status.")

	for _, loc := range productAndGuardsRE.FindAllStringSubmatchIndex(doc, -1) {
		prod, okP := countIn(doc[loc[2]:loc[3]])
		guards, okG := countIn(doc[loc[4]:loc[5]])
		if !okP || !okG {
			continue
		}
		checked += 2
		if prod != addedProduct || guards != addedGuards {
			t.Errorf("PATCHES.md's ADDED manifest summary says %d product file(s) and %d guard(s); the "+
				"manifest it summarises lists %d product file(s) and %d guard(s).\n  %s\n"+
				"The summary and the list it sits under must agree, or one of them is decoration.",
				prod, guards, addedProduct, addedGuards, lineAndSnippet(patchesFile, doc, loc[0]))
		}
	}
	for _, loc := range guardsOfGuardsRE.FindAllStringSubmatchIndex(doc, -1) {
		some, okS := countIn(doc[loc[2]:loc[3]])
		all, okA := countIn(doc[loc[4]:loc[5]])
		if !okS || !okA {
			continue
		}
		checked += 2
		if some != guardsWithAPatchNumber || all != addedGuards {
			t.Errorf("PATCHES.md says %d of the %d guards this fork adds carry a numbered marker; it is "+
				"%d of %d.\n  %s", some, all, guardsWithAPatchNumber, addedGuards, lineAndSnippet(patchesFile, doc, loc[0]))
		}
	}

	if checked < 7 {
		t.Errorf("the guard/marker count sweep read only %d count claims. It is supposed to read the "+
			"ADDED manifest summary, the root-package sentence in the regression diff, and Maintenance "+
			"step 4's two marker counts; if the prose has been reshaped, reshape the sweep with it.", checked)
	}
}

// filesCarryingAnyMarker counts what `grep -rln 'SIGHTGLASS PATCH' --include='*.go' .` returns —
// numbered markers plus the unnumbered one in patches_doc_test.go, which markerNumbersByFile
// deliberately skips. The two numbers differ by exactly that file, and Maintenance step 4 cites
// both, so both are derived here rather than typed.
func filesCarryingAnyMarker(t *testing.T) int {
	t.Helper()
	n := 0
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "SIGHTGLASS PATCH") {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree for markers: %v", err)
	}
	return n
}

// retracted are claims and instructions this file published in tags that are now known to be false
// or harmful. They are allowed to APPEAR — the tags that carried them are public and cannot be
// edited, so the correction has to stay visible in the tree that supersedes them — but only inside a
// paragraph that retracts them. Reintroducing one as an assertion, in a paragraph that does not say
// it is wrong, is the failure this pins.
// discriminators is what BINDS a marker to the claim it retracts, and it is the third thing this
// guard needed. Markers alone were not enough: FALSE, HARMFUL, RETRACTED and RETRACTION occur 30-odd
// times in PATCHES.md, so "a marker within 400 bytes" was satisfied by a marker that retracts
// something else. An adversarial reader defeated the guard with exactly that, inserting
//
//	Note for the reader: no stream-map leak resulted from the old code, because `Close()` calls
//	`forgetStreamID` unconditionally. (Separately, the old claim that the base is v0.6.8 is FALSE;
//	it is v0.6.9.)
//
// after "## Maintenance" and watching all fourteen doc guards stay green. The re-asserted sentence
// is the one this fork exists to retract; the nearby ALL-CAPS FALSE retracts the BASE VERSION.
// Proximity is not association.
//
// So a window now has to carry a marker AND a phrase that could only have been written while
// correcting THIS claim. The phrases are drawn from the claim's own `why` text, and
// TestRetractionDiscriminatorsComeFromTheWhy enforces that they are, so the list cannot quietly
// loosen into generic prose the way retractionMarkers did.
var retracted = []struct {
	claim          string
	why            string
	discriminators []string
}{
	{"stream-map leak",
		"FALSE. It is true only of transportResponseBody.Close(). On the context-cancel path " +
			"(awaitRequestCancel -> cancelStream) the inverted condition sent no RST_STREAM AND called no " +
			"cc.forgetStreamID, so the clientStream stayed in cc.streams for the life of the connection. " +
			"Measured on the shipped path by parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots: 100 " +
			"context-cancelled requests fill ChromeInitialMaxConcurrentStreams and the 101st never goes out.",
		[]string{"cc.forgetStreamID", "transportResponseBody.Close()"}},
	{"v0.6.8",
		"FALSE twice over: the base is v0.6.9, commit a8b1417 — v0.6.8 is ecfe905, one release earlier " +
			"— and this is a fork with a rewritten module path, not a vendored tree.",
		[]string{"a8b1417", "ecfe905"}},
	{"replace github.com/bogdanfinn/fhttp",
		"FALSE. go/go.mod has zero replace directives and go/AGENTS.md forbids them, because a directory " +
			"replace does not propagate to dependents. There is no such directive and no third_party/fhttp " +
			"directory, here or in Sightglass.",
		[]string{"no such directive", "third_party"}},
	{"0 test files",
		"FALSE. The tree carries 82, and the retained upstream suite is what found patch 4b.",
		[]string{"retained upstream suite", "found patch 4b"}},
	{"strip tests",
		"HARMFUL. It was an instruction in the Maintenance section; following it deletes the suite that " +
			"catches the defects. Upstream's 73 test files are KEPT, and they are the reason patch 4b exists.",
		[]string{"the suite that catches the defects", "Upstream's 73 test files are KEPT"}},
	{"re-copy upstream",
		"HARMFUL. This is a fork with a rewritten module path across 86 files; a re-copy reverts the " +
			"rewrite and the tree stops compiling against go/go.mod. Merge the upstream tag instead.",
		[]string{"rewritten module path"}},
	{"re-apply the hunks above",
		"HARMFUL. The Maintenance section that said it sat in the MIDDLE of the file, so \"above\" " +
			"excluded patches 4b, 6, 7, 8, 8b and 10 — six of the fourteen entries.",
		[]string{"MIDDLE of the file", "six of the fourteen"}},
	{"68 carry nothing but the module-path rewrite",
		"FALSE. Only 45 of the 73 upstream test files are in the diff against the base at all; the " +
			"other 28 are byte-identical to upstream, and 45 minus the 5 edited leaves 40 carrying only " +
			"the rewrite. 68 was never measured: the guard checked untouched+edited==upstream, an " +
			"identity any wrong split satisfies.",
		[]string{"byte-identical to upstream", "45 minus the 5 edited"}},
}

// retractionMarkers are what a paragraph must say for it to count as RETRACTING, rather than
// repeating, one of the claims above.
//
// This list used to read {FALSE, HARMFUL, false, wrong, retract, corrected, KEPT, not merely stale}
// and it did not work. An adversarial reader re-asserted the "stream-map leak" claim verbatim, as a
// standalone sentence, and the guard stayed green — because the same paragraph happened to contain
// the word "wrong" in an unrelated clause ("getting the tag order wrong"). Instrumented, the two
// stream-map paragraphs passed on "wrong" alone and "corrected" alone, and "not merely stale" was
// matched by no paragraph at all: a dead entry padding a list that was already too loose.
//
// Ordinary English is therefore not admissible. A marker has to be a word nobody writes by accident,
// which TestRetractionMarkersCannotBeOrdinaryProse enforces mechanically: ALL CAPS, five characters
// or more. That rules out "wrong", "corrected", "false" and "KEPT" by construction, so the list
// cannot quietly loosen again.
//
// "RETRACTION" used to be a fifth entry and it was the very defect the paragraph above condemns: it
// occurred ZERO times in PATCHES.md, so it retracted nothing and only padded the list.
// TestRetractionMarkersAreLoadBearing now fails on any entry that no paragraph of the document
// actually carries, so a dead marker cannot sit here again.
var retractionMarkers = []string{"FALSE", "HARMFUL", "RETRACTED"}

// retractionWindow is how close a discriminating phrase has to be to the claim it corrects, and
// retractionMarkerWindow is how close the ALL-CAPS marker has to be. Presence anywhere in the
// paragraph was the second half of the hole: a long paragraph can retract one thing and assert
// another, and the guard could not tell which sentence the marker belonged to.
//
// retractionMarkerWindow is DERIVED, and nothing in this comment states the number it is derived
// from — because a number written in prose is exactly what went wrong here. Up to
// v0.6.9-sightglass.18 this comment claimed "the largest marker-to-claim distance in PATCHES.md is
// 123 bytes, at the Maintenance section's quotation of all three retracted instructions at once".
// That was FALSE and load-bearing: measured with this guard's own window semantics over every
// retracted claim and every occurrence, the largest is well above 123, and setting the constant to
// 123 turns TestPatchesMDDoesNotRepeatItsRetractedClaims RED on the document it is supposed to
// describe. One sentence turned "chosen" into "derived" and the sentence was wrong, so nothing
// derived it.
//
// TestRetractionMarkerWindowIsDerivedFromTheDocument does the deriving now. It measures the widest
// marker-to-claim distance actually present in PATCHES.md, prints it, and fails unless
//
//	measured <= retractionMarkerWindow <= measured + retractionMarkerSlack
//
// The lower bound is the guard refusing to be red on its own document. The upper bound is what
// keeps the constant tight: a marker two paragraphs away from the claim it is supposed to retract is
// the failure this pins, and widening the constant to admit one is the wrong instinct, so the
// slack is small enough that the fix has to be moving the marker.
const (
	retractionWindow       = 400
	retractionMarkerWindow = 150
	retractionMarkerSlack  = 15
)

// TestRetractionMarkersCannotBeOrdinaryProse is the guard ON the guard below. Without it,
// TestPatchesMDDoesNotRepeatItsRetractedClaims can be defeated by widening its own word list, which
// is exactly how it was defeated: the entry that let the re-assertion through was "wrong".
func TestRetractionMarkersCannotBeOrdinaryProse(t *testing.T) {
	if len(retractionMarkers) == 0 {
		t.Fatal("retractionMarkers is empty, so every retracted claim below would count as retracted " +
			"by a paragraph that says nothing at all")
	}
	for _, w := range retractionMarkers {
		if w != strings.ToUpper(w) || len(w) < 5 {
			t.Errorf("retraction marker %q is ordinary prose: a marker must be ALL CAPS and at least 5 "+
				"characters, so that it cannot be satisfied by a word a writer uses for something else. "+
				"%q was added to this list once and it let a retracted claim be re-asserted verbatim.", w, "wrong")
		}
	}
}

// TestRetractionMarkersAreLoadBearing is the third guard ON the list, and it closes the defect the
// comment above `retractionMarkers` describes but did not enforce. "RETRACTION" sat in that list
// through four tags and occurred ZERO times in PATCHES.md: it could never retract anything, it only
// made the list look thorough. A marker nothing in the document carries is a dead entry, and a dead
// entry is how a list that is too loose stays looking rigorous.
func TestRetractionMarkersAreLoadBearing(t *testing.T) {
	doc := patchesDoc(t)
	for _, w := range retractionMarkers {
		if !strings.Contains(doc, w) {
			t.Errorf("retraction marker %q does not occur anywhere in %s, so it retracts nothing and "+
				"only pads the list. Remove it, or write the retraction that uses it. %q was such an "+
				"entry for four tags.", w, patchesFile, "RETRACTION")
		}
	}
}

var paraSplitRE = regexp.MustCompile(`\n\s*\n`)

func paragraphsOf(doc string) []string { return paraSplitRE.Split(doc, -1) }

// minMarkerWindow reports the SMALLEST w for which retractionNear's window `para[at-w : at+n+w]`
// contains a retraction marker, or -1 when the paragraph carries no marker at all.
//
// It is the inverse of the check retractionNear performs, and retractionNear is written in terms of
// it so the two cannot drift apart: what the guard accepts and what the derivation measures are the
// same function of the same document.
func minMarkerWindow(para string, at, n int) int {
	best := -1
	for _, m := range retractionMarkers {
		for off := 0; ; {
			i := strings.Index(para[off:], m)
			if i < 0 {
				break
			}
			j := off + i
			off = j + len(m)
			// The window reaches a marker at [j, j+len(m)) once w >= at-j (to the left) and
			// w >= j+len(m)-(at+n) (to the right). Clamping at the paragraph edges never makes a
			// marker harder to reach, so no special case is needed for it.
			need := 0
			if d := at - j; d > need {
				need = d
			}
			if d := j + len(m) - (at + n); d > need {
				need = d
			}
			if best < 0 || need < best {
				best = need
			}
		}
	}
	return best
}

// TestRetractionMarkerWindowIsDerivedFromTheDocument is what makes retractionMarkerWindow a
// measurement instead of a number somebody liked.
//
// The constant used to be justified by one sentence of prose — "the largest marker-to-claim distance
// in PATCHES.md is 123 bytes" — and that sentence was false. Nothing computed it, nothing rechecked
// it, and the document had since grown a paragraph whose widest distance is larger; set the constant
// to the claimed 123 and TestPatchesMDDoesNotRepeatItsRetractedClaims goes red on PATCHES.md itself,
// twice. A derivation that only a human performed once is not a derivation.
//
// This measures it on every run, prints it, and brackets the constant from both sides: at least the
// measurement (or the guard is red on its own document) and at most the measurement plus
// retractionMarkerSlack (or the constant has been widened to admit a marker that should have been
// moved instead).
func TestRetractionMarkerWindowIsDerivedFromTheDocument(t *testing.T) {
	doc := patchesDoc(t)
	worst, worstClaim, checked := -1, "", 0
	for _, r := range retracted {
		for _, para := range paragraphsOf(doc) {
			for off := 0; ; {
				i := strings.Index(para[off:], r.claim)
				if i < 0 {
					break
				}
				at := off + i
				off = at + len(r.claim)
				w := minMarkerWindow(para, at, len(r.claim))
				if w < 0 {
					// No marker anywhere in the paragraph. That is
					// TestPatchesMDDoesNotRepeatItsRetractedClaims's failure to report, not this
					// one's, and reporting it here too would just double the noise.
					continue
				}
				checked++
				if w > worst {
					worst, worstClaim = w, r.claim
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no retracted claim occurs in PATCHES.md beside any marker, so there is nothing to " +
			"derive the window from. Either the retractions have been deleted or `retracted` no longer " +
			"describes this document; both are worse than a wrong constant.")
	}
	t.Logf("widest marker-to-claim distance in %s: %d bytes over %d occurrences (claim %q). "+
		"retractionMarkerWindow = %d, slack %d of %d permitted.",
		patchesFile, worst, checked, worstClaim, retractionMarkerWindow,
		retractionMarkerWindow-worst, retractionMarkerSlack)
	if worst > retractionMarkerWindow {
		t.Errorf("retractionMarkerWindow is %d, but the widest marker-to-claim distance PATCHES.md "+
			"actually contains is %d bytes, at the claim %q. The guard is therefore RED on the very "+
			"document it describes: a legitimate retraction is being reported as a re-assertion. "+
			"Raise the constant to %d (and no further), or move that marker closer to the claim.",
			retractionMarkerWindow, worst, worstClaim, worst)
	}
	if retractionMarkerWindow > worst+retractionMarkerSlack {
		t.Errorf("retractionMarkerWindow is %d but the document only needs %d, which is %d bytes of "+
			"slack against a permitted %d. A window loose enough to reach a marker the document never "+
			"puts there is a window that will one day reach a marker retracting something else — the "+
			"exact hole this guard was rewritten to close. Set it to %d.",
			retractionMarkerWindow, worst, retractionMarkerWindow-worst, retractionMarkerSlack,
			worst+retractionMarkerSlack)
	}
}

// TestPatchesMDStatesTheWindowsItIsGuardedBy binds the front matter's description of this guard to
// the guard. PATCHES.md tells the reader that a marker must sit "within 150 bytes" and a
// discriminating phrase "within 400"; those two numbers and the list of markers were prose that
// nothing checked, so a change to either constant would have left the document quietly describing a
// guard that no longer exists.
func TestPatchesMDStatesTheWindowsItIsGuardedBy(t *testing.T) {
	doc := patchesDoc(t)
	flat := strings.Join(strings.Fields(doc), " ")
	m := regexp.MustCompile(`it is QUOTED, not stated; a ([A-Z/ ]+) marker sits within (\d+) bytes of it; ` +
		`and a phrase out of THAT claim's own correction sits within (\d+)`).FindStringSubmatch(flat)
	if m == nil {
		t.Fatal("PATCHES.md no longer states the three conditions its retraction guard applies, or no " +
			"longer states them in the form this test reads. That sentence is how a reader knows what " +
			"\"RETRACTED\" in this file is worth; it is not decoration.")
	}
	var named []string
	for _, w := range strings.Fields(m[1]) {
		if w != "/" {
			named = append(named, w)
		}
	}
	sort.Strings(named)
	want := append([]string(nil), retractionMarkers...)
	sort.Strings(want)
	if strings.Join(named, ",") != strings.Join(want, ",") {
		t.Errorf("PATCHES.md says the markers are %v; retractionMarkers is %v. The document has to name "+
			"the markers the guard actually accepts, or a reader who follows it writes a retraction the "+
			"guard rejects — or, worse, believes a word that is not a marker retracts something.",
			named, want)
	}
	if got, _ := strconv.Atoi(m[2]); got != retractionMarkerWindow {
		t.Errorf("PATCHES.md says a marker must sit within %d bytes of the claim; retractionMarkerWindow "+
			"is %d.", got, retractionMarkerWindow)
	}
	if got, _ := strconv.Atoi(m[3]); got != retractionWindow {
		t.Errorf("PATCHES.md says a discriminating phrase must sit within %d bytes of the claim; "+
			"retractionWindow is %d.", got, retractionWindow)
	}

	// The same paragraph states how many markers there are and how often they occur, and that pair is
	// the whole reason the window exists: it is BECAUSE the markers are common that proximity alone
	// was not association. It used to read "Those four markers occur thirty-odd times" — a count of a
	// list that has three entries, written as a word so that no count guard could read it, which is
	// T0534's defect verbatim.
	c := regexp.MustCompile(`Those (\d+) markers occur (\d+) times in this file`).FindStringSubmatch(flat)
	if c == nil {
		t.Fatal("PATCHES.md no longer says how many retraction markers there are and how often they " +
			"occur. That sentence is the justification for retractionMarkerWindow existing at all — the " +
			"markers are common, so nearness to one proves nothing by itself — and it must be a pair of " +
			"digits, because the previous version wrote it as \"Those four markers occur thirty-odd " +
			"times\" for a list of three and no count guard could read either number.")
	}
	if got, _ := strconv.Atoi(c[1]); got != len(retractionMarkers) {
		t.Errorf("PATCHES.md says there are %d retraction markers; retractionMarkers has %d: %v",
			got, len(retractionMarkers), retractionMarkers)
	}
	occurrences := 0
	for _, w := range retractionMarkers {
		occurrences += strings.Count(doc, w)
	}
	if got, _ := strconv.Atoi(c[2]); got != occurrences {
		t.Errorf("PATCHES.md says its retraction markers occur %d times; they occur %d. How common the "+
			"markers are is the reason the window is tight, so this is not a decorative number.",
			got, occurrences)
	}
}

// TestRetractionDiscriminatorsComeFromTheWhy is the second guard ON the guard below, and it is what
// keeps `discriminators` from decaying into "any word that happens to be nearby" — which is how
// retractionMarkers decayed the first time. A discriminator must be a phrase out of the correction
// itself, so writing one costs you the correction.
func TestRetractionDiscriminatorsComeFromTheWhy(t *testing.T) {
	for _, r := range retracted {
		if len(r.discriminators) == 0 {
			t.Errorf("retracted claim %q has no discriminating phrase, so any paragraph carrying any "+
				"ALL-CAPS marker would count as retracting it — which is exactly the hole an adversarial "+
				"reader walked through with %q", r.claim, "no stream-map leak resulted from the old code")
			continue
		}
		for _, d := range r.discriminators {
			if len(d) < 7 {
				t.Errorf("discriminator %q for claim %q is %d bytes: too short to be specific to one "+
					"correction", d, r.claim, len(d))
			}
			if !strings.Contains(r.why, d) {
				t.Errorf("discriminator %q is not a phrase from the `why` of claim %q:\n  why: %s\n"+
					"A discriminator that is not part of the correction does not bind the marker to the "+
					"claim; it is just another word that can appear by accident.", d, r.claim, r.why)
			}
		}
	}
}

// retractionNear reports whether the claim occurrence at `at` is RETRACTED rather than merely
// mentioned near something strong-sounding: within retractionWindow bytes there must be both
//
//	(a) an ALL-CAPS retraction marker, and
//	(b) a phrase from THIS claim's own correction.
//
// (b) is the half that was missing. With (a) alone the guard could be satisfied by a marker that
// retracts a different claim, and it was: see the comment on `discriminators`.
//
// It returns the two answers separately so the failure message can say WHICH half is missing — a
// guard that says only "this failed" sends the next reader to read the whole paragraph.
func retractionNear(para string, at, n int, discriminators []string) (marker, discriminator bool) {
	window := func(w int) string {
		lo, hi := at-w, at+n+w
		if lo < 0 {
			lo = 0
		}
		if hi > len(para) {
			hi = len(para)
		}
		return para[lo:hi]
	}
	// Expressed through minMarkerWindow so that TestRetractionMarkerWindowIsDerivedFromTheDocument
	// measures exactly what this accepts. A second copy of the window arithmetic here is how the
	// constant and its justification came apart in the first place.
	if w := minMarkerWindow(para, at, n); w >= 0 && w <= retractionMarkerWindow {
		marker = true
	}
	wide := window(retractionWindow)
	for _, d := range discriminators {
		if strings.Contains(wide, d) {
			discriminator = true
			break
		}
	}
	return marker, discriminator
}

// quotedAt reports whether the claim occurrence at [at, at+n) sits inside a double-quoted span.
//
// This is the third condition, and it is the one that closes the attack the other two do not. A
// marker plus a discriminating phrase can both be COPIED into a sentence that asserts the claim
// anyway — "no stream-map leak resulted from the old code, because transportResponseBody.Close()
// calls cc.forgetStreamID unconditionally, so nothing was RETRACTED here after all" satisfies both
// and is a re-assertion. What a retraction never does is STATE the claim: it QUOTES it and then
// corrects it. So every occurrence of a retracted claim in this file has to be inside quotes.
//
// What this still cannot prove: that the prose around the quotation means what it says. A writer
// determined to mislead can quote the claim, put FALSE beside it and then agree with it. Three
// lexical conditions make that require deliberate work rather than carelessness, which is the bar a
// document guard can actually reach; the claim's TRUTH is pinned in code by
// TestCancelStreamForgetsTheStream and on the shipped path by
// parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots, and that is where it belongs.
func quotedAt(para string, at, n int) bool {
	return strings.Count(para[:at], `"`)%2 == 1 && strings.Contains(para[at+n:], `"`)
}

// TestPatchesMDDoesNotRepeatItsRetractedClaims fails if any retracted claim appears anywhere that
// does not retract it, ADJACENTLY. Three conditions, and all three were needed:
//
//	(1) the claim must still appear at all — the published tags carrying it cannot be edited, so the
//	    correction has to live in the tree that supersedes them;
//	(2) EVERY occurrence must have, within retractionWindow bytes of it and in the same paragraph,
//	    BOTH a retraction marker AND a phrase from that claim's own correction — a marker alone
//	    proves only that some retraction is nearby, not that THIS claim is the one being retracted;
//	(3) at least one occurrence must sit beside the literal "RETRACTED", so that there is one place a
//	    reader can find the correction rather than only places that are not the assertion.
func TestPatchesMDDoesNotRepeatItsRetractedClaims(t *testing.T) {
	doc := patchesDoc(t)
	paras := regexp.MustCompile(`\n\s*\n`).Split(doc, -1)
	for _, r := range retracted {
		found, explicit := 0, false
		for _, para := range paras {
			if !strings.Contains(para, r.claim) {
				continue
			}
			if strings.Contains(para, "RETRACTED") {
				explicit = true
			}
			for off := 0; ; {
				i := strings.Index(para[off:], r.claim)
				if i < 0 {
					break
				}
				at := off + i
				found++
				marker, disc := retractionNear(para, at, len(r.claim), r.discriminators)
				switch {
				case !quotedAt(para, at, len(r.claim)):
					t.Errorf("PATCHES.md STATES the retracted claim %q rather than quoting it:\n  ...%s...\n"+
						"Why it is retracted: %s\n"+
						"A retraction never asserts the claim; it quotes the sentence that was published and "+
						"then corrects it. Put the claim in double quotes, or do not write it.",
						r.claim, squash(para), r.why)
				case !marker:
					t.Errorf("PATCHES.md states the retracted claim %q with no retraction marker %v within "+
						"%d bytes of it:\n  ...%s...\nWhy it is retracted: %s",
						r.claim, retractionMarkers, retractionMarkerWindow, squash(para), r.why)
				case !disc:
					t.Errorf("PATCHES.md states the retracted claim %q beside a retraction marker that "+
						"retracts something else. Within %d bytes there is no phrase from THIS claim's "+
						"correction (%v), so the paragraph re-asserts the claim and retracts a different "+
						"one:\n  ...%s...\nWhy %q is retracted: %s\n"+
						"Proximity is not association: a reader who greps for this claim has to land on ITS "+
						"correction, not on a paragraph that happens to say FALSE about the base version.",
						r.claim, retractionWindow, r.discriminators, squash(para), r.claim, r.why)
				}
				off = at + len(r.claim)
			}
		}
		if found == 0 {
			t.Errorf("PATCHES.md no longer mentions %q anywhere, but the retraction of it must stay: %s "+
				"The tags that published it cannot be edited, so the correction has to live in the tree that "+
				"supersedes them.", r.claim, r.why)
		}
		if found > 0 && !explicit {
			t.Errorf("PATCHES.md mentions the retracted claim %q, but no paragraph containing it also "+
				"carries the literal word RETRACTED. A reader who greps for the claim must land on the "+
				"correction, not on a paragraph that merely happens to contain a strong-sounding word.",
				r.claim)
		}
	}
}

func squash(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// TestPatchesMDMaintenanceIsLast pins the structural half of the Maintenance defect. The old section
// said "re-apply the hunks above" while sitting in the MIDDLE of the file, so "above" silently
// excluded patches 4b, 6, 7, 8, 8b and 10 — six of the fourteen entries.
func TestPatchesMDMaintenanceIsLast(t *testing.T) {
	doc := patchesDoc(t)
	const maint = "\n## Maintenance\n"
	i := strings.Index(doc, maint)
	if i < 0 {
		t.Fatal("PATCHES.md has no ## Maintenance section")
	}
	if j := strings.Index(doc[i+len(maint):], "\n## "); j >= 0 {
		after := doc[i+len(maint)+j:]
		if k := strings.IndexByte(after[1:], '\n'); k >= 0 {
			after = after[1 : k+1]
		}
		t.Errorf("## Maintenance is not the last section of PATCHES.md — %q comes after it. "+
			"It must be last, because it tells the next maintainer to re-apply \"the patches above\", and "+
			"when it sat in the middle of the file that phrase excluded six of the fourteen.",
			strings.TrimSpace(after))
	}
}

// sightglassAuthored returns PATCHES.md plus every .go file in this tree that carries a
// [SIGHTGLASS PATCH n] marker — i.e. the prose this fork is responsible for, as opposed to upstream's.
func sightglassAuthored(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	b, err := os.ReadFile(patchesFile)
	if err != nil {
		t.Fatalf("reading %s: %v", patchesFile, err)
	}
	out[patchesFile] = string(b)
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || filepath.Base(path) == "patches_doc_test.go" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if markerRE.Match(b) {
			out[path] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	return out
}

// TestPatchesMDCitesNoLineNumbersIntoThisTree pins the rule that replaced the drifted citations.
// Every `file.go:NNN` reference into this tree in an earlier revision pointed at the wrong line —
// transport.go:879 had become :981, :766 had become :851, :967 had become :1075, :2743 had become
// :2864, and http2/chrome_concurrency.go still cited ":766 and :2737" and called them "exactly two"
// when patch 3 touches three lines — so this fork's own prose cites functions and markers instead.
func TestPatchesMDCitesNoLineNumbersIntoThisTree(t *testing.T) {
	// Quoted test output is not a citation: it is what the guard printed, and reproducing it exactly
	// is the point of an ablation record. That used to be a hand-maintained allow-list of one entry,
	// which is a list that rots — and did: the entry named line 159 of a file whose guard now prints
	// 161. The rule is structural instead. A fenced block in PATCHES.md is machine output, so line
	// numbers inside one are quoted, not cited; everywhere else in the PROSE they are forbidden,
	// which is where every stale citation actually lived. The exemption is not a blank cheque: a
	// quoted line number must still be inside the file it names, so output copied from a tree that
	// has since moved by hundreds of lines is caught.
	re := regexp.MustCompile(`([A-Za-z0-9_./]+\.go):(\d+)`)
	for file, body := range sightglassAuthored(t) {
		scan := body
		if file == patchesFile {
			scan = stripFencedBlocks(body)
			for _, m := range re.FindAllStringSubmatch(body, -1) {
				if !fileExistsInTree(m[1]) {
					continue
				}
				n, _ := strconv.Atoi(m[2])
				if lines := fileLineCount(m[1]); lines > 0 && n > lines {
					t.Errorf("PATCHES.md quotes %s, and %s has only %d lines. Output quoted from a tree "+
						"that has since moved is a record of a run nobody can reproduce.", m[0], m[1], lines)
				}
			}
		}
		var bad []string
		for _, m := range re.FindAllStringSubmatch(scan, -1) {
			// Only citations into THIS tree drift with our own edits; spdy_session.cc:837 and friends are
			// pinned by the Chromium version named beside them.
			if !fileExistsInTree(m[1]) {
				continue
			}
			bad = append(bad, m[0])
		}
		if len(bad) > 0 {
			t.Errorf("%s cites line numbers into this tree: %v\n"+
				"Every such citation in the published revisions was stale by up to 120 lines, because the twelve "+
				"patches move each other's code. Cite the function and its [SIGHTGLASS PATCH n] marker instead.",
				file, unique(bad))
		}
	}
}

// testCallRE matches the opening of a testing call, which is the only kind of source line a
// `file.go:NNN:` prefix in Go test output can ever name.
var testCallRE = regexp.MustCompile(`\bt\.(Error|Errorf|Fatal|Fatalf|Log|Logf|Skip|Skipf)\(`)

// goStringJoinRE matches the concatenation joint between two adjacent Go string literals, including
// the line break and indentation gofmt puts there. Removing it reassembles the message the test
// actually prints from the source that prints it.
var goStringJoinRE = regexp.MustCompile(`"\s*\+\s*"`)

// docCitationRE matches one quoted `file.go:NNN: message` line of test output inside a fenced block.
var docCitationRE = regexp.MustCompile(`([A-Za-z0-9_./]+\.go):(\d+): (.*)`)

// formatVerbRE matches a printf verb, which is the only place a published line of test output is
// allowed to differ from the source that printed it.
var formatVerbRE = regexp.MustCompile(`%[-+# 0-9.*]*[a-zA-Z]`)

// sourceMessageAt reassembles the message printed by the testing call starting at line n of src, as
// literal segments separated by the format verbs that were substituted at run time.
func sourceMessageAt(src string, n int) []string {
	lines := strings.Split(src, "\n")
	if n < 1 || n > len(lines) {
		return nil
	}
	hi := n + 15
	if hi > len(lines) {
		hi = len(lines)
	}
	win := strings.Join(lines[n-1:hi], "\n")
	// Everything before the call's opening parenthesis is code, not message.
	loc := testCallRE.FindStringIndex(win)
	if loc == nil {
		return nil
	}
	win = strings.Join(strings.Fields(concatenatedLiteral(win[loc[1]:])), " ")
	var segs []string
	for _, seg := range formatVerbRE.Split(win, -1) {
		segs = append(segs, strings.TrimSpace(seg))
	}
	return segs
}

// concatenatedLiteral reads the run of Go string literals joined by `+` that begins at the start of
// s, and returns the string they denote. It stops at the first thing that is not a literal, `+` or
// whitespace — the closing paren, a comma, an argument — so the message does not run on into the
// code that follows it. Reading a fixed number of lines instead is what made the first version of
// this guard report a whole function body as the "message".
func concatenatedLiteral(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '"':
			i++
			for i < len(s) {
				if s[i] == '\\' && i+1 < len(s) {
					switch s[i+1] {
					case 'n', 't', 'r':
						out.WriteByte(' ') // a newline in the output is whitespace here
					default:
						out.WriteByte(s[i+1])
					}
					i += 2
					continue
				}
				if s[i] == '"' {
					i++
					break
				}
				out.WriteByte(s[i])
				i++
			}
		case c == '+' || c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		default:
			return out.String()
		}
	}
	return out.String()
}

// resolveInTree turns a citation like `cancel_stream_reset_test.go` into the path it names, so a
// bare basename is checked rather than silently skipped. fileLineCount and the older citation guard
// both read the citation as written, which meant every basename citation — which is all of them —
// got a free pass from the line-count half of that check.
func resolveInTree(rel string) string {
	if fi, err := os.Stat(rel); err == nil && !fi.IsDir() {
		return rel
	}
	found := ""
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) == rel {
			found = path
		}
		return nil
	})
	return found
}

// TestPatchesMDQuotedAblationOutputIsReproducible closes the hole that
// TestPatchesMDCitesNoLineNumbersIntoThisTree's own comment warns about and then walks through.
//
// That test exempts fenced blocks, on the correct grounds that quoted machine output is a record and
// not a citation, and checks only that a quoted line number is inside the file it names. It is not
// enough, and the failure was live: PATCHES.md published patch 11's ablation as
// `cancel_stream_reset_test.go:161: didReset=false: the clientStream was still in cc.streams…`
// long after that assertion had moved to a different line AND grown a second error line the block
// did not show. 161 was still inside the file, so the guard passed on a record of a run nobody could
// reproduce — which is precisely what C3 asks a published ablation not to be.
//
// The rule here is the one the exemption needs: a quoted `file.go:NNN:` must land on a testing call,
// and the message quoted after it must be the message THAT call prints. Format verbs are the only
// slack — the doc shows the substituted value, the source shows `%d` — so the comparison uses the
// longest verb-free, digit-free prefix of the quoted message.
func TestPatchesMDQuotedAblationOutputIsReproducible(t *testing.T) {
	doc := patchesDoc(t)
	prose := stripFencedBlocks(doc)
	checked := 0
	for _, m := range docCitationRE.FindAllStringSubmatch(doc, -1) {
		file, lineStr, msg := m[1], m[2], m[3]
		if !fileExistsInTree(file) {
			continue
		}
		if strings.Contains(prose, m[0]) {
			continue // prose, not a fenced block: the other guard bans it outright
		}
		n, _ := strconv.Atoi(lineStr)
		path := resolveInTree(file)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("PATCHES.md quotes output from %s, which cannot be read: %v", file, err)
			continue
		}
		src := string(b)
		lines := strings.Split(src, "\n")
		if n < 1 || n > len(lines) {
			t.Errorf("PATCHES.md quotes %s:%d, and %s has %d lines. Output quoted from a tree that has "+
				"since moved is a record of a run nobody can reproduce.", file, n, file, len(lines))
			continue
		}
		checked++
		if !testCallRE.MatchString(lines[n-1]) {
			t.Errorf("PATCHES.md quotes test output as coming from %s:%d, but line %d of that file is "+
				"not a t.Error/t.Fatal/t.Log call:\n  %s\nGo test output names the line of the call that "+
				"printed it, so this block is output from a different revision of the file.",
				file, n, n, strings.TrimSpace(lines[n-1]))
			continue
		}
		// The message is compared segment by segment, where the segments are what the source says
		// literally and the gaps are the printf verbs whose values only exist at run time. The quoted
		// line may stop early — Go test output wraps and a document quotes what fits — so a segment
		// the doc never reaches is not a failure; a segment it contradicts is.
		docNorm := strings.Join(strings.Fields(msg), " ")
		segs := sourceMessageAt(src, n)
		if len(segs) == 0 {
			t.Errorf("PATCHES.md quotes %s:%d, but no message could be read out of the call there.", file, n)
			continue
		}
		// The document may wrap mid-segment — Go test output is long and PATCHES.md is prose — so
		// the two only have to AGREE as far as the shorter of them goes.
		if segs[0] != "" && !strings.HasPrefix(docNorm, segs[0]) && !strings.HasPrefix(segs[0], docNorm) {
			t.Errorf("PATCHES.md publishes this as the output of %s:%d:\n  %s\n"+
				"but the call at %s:%d starts:\n  %s\n"+
				"A published ablation record is only worth anything if re-running it produces the text in "+
				"the document; this is the text of a run nobody can reproduce. PATCHES.md carried "+
				"`cancel_stream_reset_test.go:161` for two tags after that assertion had moved, and the "+
				"line-number check waved it through because 161 was still inside the file.",
				file, n, squash(msg), file, n, squash(segs[0]))
			continue
		}
		agreed, pos := 0, 0
		if segs[0] != "" {
			agreed = len(segs[0])
			if len(docNorm) < agreed {
				agreed = len(docNorm)
			}
			pos = agreed
		}
		truncated := false
		for _, seg := range segs[1:] {
			if len(seg) < 4 {
				continue // punctuation between two substituted values proves nothing either way
			}
			i := strings.Index(docNorm[pos:], seg)
			if i < 0 {
				truncated = true // the quoted line stopped before this segment
				break
			}
			pos += i + len(seg)
			agreed += len(seg)
		}
		// Twenty literal characters is enough on its own. Below that the message is mostly
		// substituted values — `d.search(%v) = %v, %v; want %v, %v` has nine — and the check is that
		// EVERY literal fragment the quoted line was long enough to reach is present, in order.
		if agreed < 20 && (truncated || agreed < 6) {
			t.Errorf("PATCHES.md publishes this as the output of %s:%d:\n  %s\n"+
				"and only %d characters of it are literal text the message at %s:%d contains:\n  %v\n"+
				"Quote enough of the output for it to be checkable against the guard that prints it.",
				file, n, squash(msg), agreed, file, n, segs)
		}
	}
	if checked == 0 {
		t.Error("no quoted `file.go:NNN:` test output was found in PATCHES.md's fenced blocks. The " +
			"ablation records are the evidence this document rests on; a file with none of them is not " +
			"a provenance record, and this guard has nothing to check.")
	}
}

// tagCitationRE reads a sentence of the form
//
//	`v0.6.9-sightglass.11` through `.14` cited line 159 of `cancel_stream_reset_test.go`
//
// i.e. a claim about what a RANGE of published tags said.
var tagCitationRE = regexp.MustCompile(
	"`(v[0-9.]+-sightglass\\.(\\d+))` through `\\.(\\d+)` cited line (\\d+) of `([A-Za-z0-9_./]+\\.go)`")

// TestPatchesMDTagCitationHistoryIsTrue checks what this file says about what OLDER TAGS published.
//
// A retraction has to say which revisions carried the retracted text, or a reader cannot tell which
// published artefact is wrong. That sentence is prose about immutable objects, so nothing stops it
// being written from memory — and the first attempt at it was: it said tags .9 through .18 all
// printed the patch-11 ablation at line 161, when .9 and .10 printed no citation at all, .11 through
// .14 printed 159, and only .15 through .18 printed 161. A false statement about which tag was wrong,
// inside the correction of a tag that was wrong, is the recurring defect of this whole document.
//
// Published tags cannot change, so this is decidable: read each named tag's PATCHES.md back out of
// git and require it to contain the citation the sentence attributes to it.
func TestPatchesMDTagCitationHistoryIsTrue(t *testing.T) {
	doc := patchesDoc(t)
	flat := strings.Join(strings.Fields(doc), " ")
	claims := tagCitationRE.FindAllStringSubmatch(flat, -1)
	if len(claims) == 0 {
		t.Fatal("PATCHES.md no longer says which published tags carried the retracted ablation " +
			"citation. A retraction that does not name the artefacts it retracts cannot be acted on by " +
			"anyone holding one of them.")
	}
	// The guard ON this guard. The first version of the sentence wrote its second half as
	// "and `.15` through `.18` cited line 161" — a range whose first element is an abbreviation, which
	// tagCitationRE does not match, so HALF the claim went unchecked and the test was green on a
	// deliberately wrong second half. Every "cited line" in this file has to be in the checkable form.
	if said := strings.Count(flat, "cited line "); said != len(claims) {
		t.Errorf("PATCHES.md makes %d claim(s) of the form \"... cited line N ...\" but only %d of them "+
			"are in the form this guard can read, which is\n"+
			"  `<full tag>` through `.<n>` cited line <N> of `<file>`\n"+
			"An abbreviated first element (\"`.15` through `.18`\") is not checkable, and an unchecked "+
			"half of a sentence is where the false half lived last time.", said, len(claims))
	}
	if !gitHere() {
		t.Skipf("the %d tag-range claim(s) in the retraction paragraph are checked with "+
			"`git show <tag>:PATCHES.md`, and this copy of the module is not a git checkout. Every other "+
			"check in this file still ran.", len(claims))
	}
	for _, c := range claims {
		firstTag, lo, hi, line, file := c[1], c[2], c[3], c[4], c[5]
		prefix := strings.TrimSuffix(firstTag, lo)
		loN, _ := strconv.Atoi(lo)
		hiN, _ := strconv.Atoi(hi)
		if hiN < loN {
			t.Errorf("PATCHES.md claims the range %s through .%s, which runs backwards", firstTag, hi)
			continue
		}
		for n := loN; n <= hiN; n++ {
			tag := prefix + strconv.Itoa(n)
			out, err := exec.Command("git", "show", tag+":"+patchesFile).Output()
			if err != nil {
				t.Errorf("PATCHES.md says tag %s cited %s:%s, and that tag's %s cannot be read: %v. "+
					"A claim about a published tag is only checkable while the tag is here.",
					tag, file, line, patchesFile, err)
				continue
			}
			want := file + ":" + line
			if !strings.Contains(string(out), want) {
				var got []string
				for _, m := range regexp.MustCompile(regexp.QuoteMeta(file)+`:(\d+)`).FindAllStringSubmatch(string(out), -1) {
					got = append(got, m[0])
				}
				t.Errorf("PATCHES.md says %s cited %q, and it does not. That tag's %s cites %v.\n"+
					"Published tags are immutable, so a sentence about what one of them said is a fact "+
					"with an answer, not a recollection.", tag, want, patchesFile, unique(got))
			}
		}
	}
}

// TestSightglassProseReferencesDocsThatExist pins the other half of the same defect:
// http2/chrome_concurrency.go told the reader to "see FHTTP_LAYER_PATCH.md, patch 3", and no file of
// that name has ever existed in this tree.
func TestSightglassProseReferencesDocsThatExist(t *testing.T) {
	re := regexp.MustCompile(`(?:^|[^/A-Za-z0-9_.-])([A-Za-z0-9_-]+\.md)`)
	for file, body := range sightglassAuthored(t) {
		for _, loc := range re.FindAllStringSubmatchIndex(body, -1) {
			name := body[loc[2]:loc[3]]
			if fileExistsInTree(name) {
				continue
			}
			// A doc may be named precisely to say it does not exist — that is a retraction, not a pointer.
			lo, hi := loc[2]-300, loc[3]+300
			if lo < 0 {
				lo = 0
			}
			if hi > len(body) {
				hi = len(body)
			}
			win := body[lo:hi]
			if strings.Contains(win, "never existed") || strings.Contains(win, "does not exist") {
				continue
			}
			t.Errorf("%s points the reader at %q, which does not exist in this tree and is not marked as "+
				"a retraction. A provenance record that cites a document nobody can open is the same failure "+
				"as one that cites the wrong upstream version: the reader cannot check it.", file, name)
		}
	}
}

// TestForkPinsItsSiblingForkAtTheVersionPATCHESNames keeps this fork's go.mod and its own
// "Sibling-fork pins" paragraph from drifting apart. The cross-repository half of the rule — that
// this pin is never OLDER than the utls that go/go.mod ships — cannot be checked from inside the
// fork, which has no view of Sightglass; it is checked on the Sightglass side.
func TestForkPinsItsSiblingForkAtTheVersionPATCHESNames(t *testing.T) {
	gomod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	m := regexp.MustCompile(`github\.com/Berserk-Automation-Hub/utls (v[0-9][^\s]*)`).FindSubmatch(gomod)
	if m == nil {
		t.Fatal("go.mod no longer requires github.com/Berserk-Automation-Hub/utls. This fork imports utls " +
			"for its TLS types; if the requirement is gone, the module path rule (HR-2, one utls in the " +
			"binary) is no longer being enforced by the compiler.")
	}
	pinned := string(m[1])
	doc := patchesDoc(t)
	sec := section(t, doc, "## Maintenance")
	// The paragraph names the CURRENT pin in one fixed form, so that quoting an OLD version in the
	// retraction beside it cannot be mistaken for naming the current one.
	d := regexp.MustCompile("currently\n\\*\\*`(v[0-9][^`]*)`\\*\\*").FindStringSubmatch(sec)
	if d == nil {
		t.Fatal("PATCHES.md's \"Sibling-fork pins\" paragraph no longer names the current utls pin in the " +
			"form \"currently **`vX.Y.Z-sightglass.N`**\". That sentence is the only thing tying this fork's " +
			"go.mod to its own documentation, and the two silently diverged for five tags.")
	}
	if d[1] != pinned {
		t.Errorf("go.mod pins utls at %s, but PATCHES.md's \"Sibling-fork pins\" paragraph says the current "+
			"pin is %s. This fork shipped v0.6.9-sightglass.11 pinning utls v1.7.8-sightglass.1 while "+
			"go/go.mod shipped v1.7.8-sightglass.6, so the fork's own suite was testing a utls the product "+
			"does not use and nothing said so.", pinned, d[1])
	}
}

func fileExistsInTree(rel string) bool {
	if fi, err := os.Stat(rel); err == nil && !fi.IsDir() {
		return true
	}
	// A bare basename like `transport.go` also names a file in this tree.
	found := false
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if !info.IsDir() && filepath.Base(path) == rel {
			found = true
		}
		return nil
	})
	return found
}

// TestPatchesMDCarriesARegressionDiff pins C3's last requirement: a fork whose upstream is not green
// must publish a regression diff, not a green-suite claim.
func TestPatchesMDCarriesARegressionDiff(t *testing.T) {
	doc := patchesDoc(t)
	if !strings.Contains(doc, "REGRESSION-DIFF-BEGIN") || !strings.Contains(doc, "REGRESSION-DIFF-END") {
		t.Fatal("PATCHES.md has lost its machine-findable regression-diff block. Upstream v0.6.9 is not green, " +
			"so 'the suite passes' is never an available claim here; the only honest statement is a before/after " +
			"failure-set diff, and it has to be findable to be kept current.")
	}
	blk := doc[strings.Index(doc, "REGRESSION-DIFF-BEGIN"):strings.Index(doc, "REGRESSION-DIFF-END")]
	if strings.Contains(blk, "PLACEHOLDER") {
		t.Fatal("the regression-diff block in PATCHES.md still contains PLACEHOLDER — it was never filled in " +
			"with a measured before/after failure set.")
	}
	for _, want := range []string{"before:", "after:", "NEW failures:"} {
		if !strings.Contains(blk, want) {
			t.Errorf("the regression-diff block in PATCHES.md has no %q line. A diff without both sides and an "+
				"explicit new-failure count is a green-suite claim wearing a diff's clothes.", want)
		}
	}
}

// stripFencedBlocks removes ``` fenced blocks from a Markdown body. What is left is the prose.
func stripFencedBlocks(body string) string {
	var out strings.Builder
	in := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			in = !in
			continue
		}
		if !in {
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// fileLineCount returns the number of lines in a tree-relative path, or 0 if it cannot be read as
// one (a bare basename cited from elsewhere, for instance).
func fileLineCount(rel string) int {
	b, err := os.ReadFile(rel)
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "\n") + 1
}

// ---------------------------------------------------------------------------------------------
// PER-FILE PATCH ATTRIBUTION
//
// TestPatchesMDPatchNumbersMatchTheSourceMarkers above compares the patch numbers as SETS — the
// union of every marker in the tree against the union of the headings. An adversarial reader broke
// it in two moves that a set comparison cannot see:
//
//	(a) swapping two markers between two files (http2/client_conn_pool.go's "PATCH 5" became
//	    "PATCH 9" and http2/transport.go's "PATCH 9" became "PATCH 5"). Both markers then named the
//	    wrong patch — which is the EXACT published defect this file was written for, the patch-5
//	    comment in client_conn_pool.go that read "PATCH 3" — and every guard stayed green, because
//	    the union {5,9} was unchanged;
//	(b) re-attributing a file in the manifest, "M request.go patch 8b" -> "patch 7". Green, because
//	    docManifest parsed only the A/M letter and the path and threw the rest away.
//
// Those two are one hole: the doc's patch<->file MAPPING was unchecked. It is load-bearing, because
// Maintenance step 4 tells the next maintainer to resolve upstream merge conflicts "against the
// markers", and a marker that names the wrong patch sends them to the wrong section of this file.
//
// This check closes it in both directions: every marker in a .go file must be listed beside that
// file in the manifest, and every patch number the manifest lists beside a .go file must appear as a
// marker in it.
// ---------------------------------------------------------------------------------------------

var manifestPatchRE = regexp.MustCompile(`\bpatch(?:es)?\s+((?:\d+b?)(?:\s*,\s*\d+b?)*)`)

// docFilePatches maps each manifest path to the patch numbers listed beside it. Paths with no patch
// number (PATCHES.md itself, go.mod, go.sum) are absent rather than empty.
func docFilePatches(t *testing.T, doc string) map[string]map[string]bool {
	t.Helper()
	out := map[string]map[string]bool{}
	for _, line := range strings.Split(section(t, doc, "## Files touched"), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || (f[0] != "A" && f[0] != "M") {
			continue
		}
		path := f[1]
		rest := line[strings.Index(line, path)+len(path):]
		m := manifestPatchRE.FindStringSubmatch(rest)
		if m == nil {
			continue
		}
		nums := map[string]bool{}
		for _, n := range strings.Split(m[1], ",") {
			nums[strings.TrimSpace(n)] = true
		}
		out[path] = nums
	}
	if len(out) == 0 {
		t.Fatal("no manifest line in PATCHES.md attributes a file to a patch number. The manifest is how " +
			"the next maintainer finds which sections apply to a file they are merging; without the " +
			"attribution this check proves nothing.")
	}
	return out
}

// markerNumbersByFile is markerNumbers, per file rather than unioned.
func markerNumbersByFile(t *testing.T) map[string]map[string]bool {
	t.Helper()
	out := map[string]map[string]bool{}
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || filepath.Base(path) == "patches_doc_test.go" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range markerRE.FindAllStringSubmatch(string(b), -1) {
			for _, n := range strings.Split(m[1], "+") {
				if out[path] == nil {
					out[path] = map[string]bool{}
				}
				out[path][n] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree for patch markers: %v", err)
	}
	return out
}

// TestPatchesMDAttributesEveryMarkerToTheRightFile pins the doc's patch<->file mapping site by site.
func TestPatchesMDAttributesEveryMarkerToTheRightFile(t *testing.T) {
	doc := patchesDoc(t)
	docPatches := docFilePatches(t, doc)
	markers := markerNumbersByFile(t)

	for path, got := range markers {
		want, listed := docPatches[path]
		if !listed {
			t.Errorf("%s carries [SIGHTGLASS PATCH %v] markers, but PATCHES.md's \"Files touched\" manifest "+
				"attributes it to no patch at all. A marked file that the manifest does not attribute is a "+
				"patch the next upstream merge resolves against nothing.", path, sortedKeys(got))
			continue
		}
		if missing, extra := diffSets(want, got); len(missing) > 0 || len(extra) > 0 {
			t.Errorf("%s: PATCHES.md attributes it to patches %v, its markers say %v.\n"+
				"  listed in the manifest but not marked in the file: %v\n"+
				"  marked in the file but not listed in the manifest: %v\n"+
				"Maintenance step 4 tells the next maintainer to resolve conflicts \"against the markers\", so a "+
				"marker that names the wrong patch sends them to the wrong section of this file — which is "+
				"exactly how http2/client_conn_pool.go's patch-5 comment came to read \"PATCH 3\".",
				path, sortedKeys(want), sortedKeys(got), missing, extra)
		}
	}
	for path, want := range docPatches {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		if _, ok := markers[path]; ok {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue // the file manifest test owns "this path is not in the tree"
		}
		t.Errorf("PATCHES.md attributes %s to patches %v, but that file carries no [SIGHTGLASS PATCH n] "+
			"marker. `grep -rn 'SIGHTGLASS PATCH'` is what Maintenance step 4 sends the next maintainer to; "+
			"a patch site it does not list is a patch that merge will silently drop.", path, sortedKeys(want))
	}
}

// guardRowRE matches a row of the per-patch guard table whose first cell is a patch id: `| 11 | ...`
// or `| 4b | ...`. Rows keyed by something else ("this file", "sibling pins") are not patch rows and
// carry no marker obligation. The prose above the table quotes an invented row inside backticks; it
// does not begin a line with `|`, so it is not matched here.
var guardRowRE = regexp.MustCompile(`(?m)^\|\s*(\d+[a-z]?)\s*\|([^|]*)\|`)

var goPathRE = regexp.MustCompile("`([A-Za-z0-9_./-]+\\.go)`")

// TestPatchesMDGuardTableNamesFilesThatExist pins the other half of "a guard cannot be cited into
// existence". TestPatchesMDNamesGuardsThatExist checks TEST NAMES and exempts anything spelled
// `parity.X`; it never checked the fork-side FILE names in the same table. An adversarial reader
// added a row naming a fork test file and a parity test of which NEITHER exists, and all ten guards
// stayed green. This check covers the file half; the Sightglass half — that every `parity.Test*`
// this file names resolves to a real test in go/ — is checked there, by
// parity.TestForkPATCHESNamesSightglassGuardsThatExist, because the fork has no view of Sightglass.
//
// os.Stat alone was not enough either, and an adversarial reader proved that too: patch 7's row was
// changed from `transport_deflate_leak_test.go` to `http2/goaway_flush_test.go` — a real file, the
// wrong patch — and all fourteen guards stayed green. The table's stated job is "which half is
// checked by which test", so a row that names a real file guarding a DIFFERENT patch fails the job
// while passing the check. The fix is to cross the row against the markers: the file a patch row
// names must itself carry [SIGHTGLASS PATCH n] for that same n.
func TestPatchesMDGuardTableNamesFilesThatExist(t *testing.T) {
	doc := patchesDoc(t)
	sec := section(t, doc, "## Where each patch is guarded")
	markers := markerNumbersByFile(t)

	seen := map[string]bool{}
	checked := 0
	for _, m := range goPathRE.FindAllStringSubmatch(sec, -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		checked++
		if fi, err := os.Stat(m[1]); err != nil || fi.IsDir() {
			t.Errorf("PATCHES.md's guard table names %q as the fork-local guard for a patch, and no such "+
				"file exists in this tree. A guard that can be cited into existence is not a guard; it is the "+
				"same claim as an ablation that was never run.", m[1])
		}
	}
	if checked == 0 {
		t.Error("the guard table in PATCHES.md names no fork-local guard FILE at all, so this check read " +
			"nothing and would pass on a table full of invented ones")
	}

	rows := 0
	for _, row := range guardRowRE.FindAllStringSubmatch(sec, -1) {
		patch, forkCell := row[1], row[2]
		for _, f := range goPathRE.FindAllStringSubmatch(forkCell, -1) {
			path := f[1]
			if _, err := os.Stat(path); err != nil {
				continue // the existence check above owns "this path is not in the tree"
			}
			rows++
			if !markers[path][patch] {
				t.Errorf("PATCHES.md's guard table gives patch %s the fork-local guard %q, but that file "+
					"carries [SIGHTGLASS PATCH %v], not %s. The table's job is which half of a patch is "+
					"checked by which test; a row that names a REAL file guarding a DIFFERENT patch passes "+
					"os.Stat and still sends the next maintainer to the wrong test. Markers are the tie.",
					patch, path, sortedKeys(markers[path]), patch)
			}
		}
	}
	if rows == 0 {
		t.Error("no row of the per-patch guard table names a fork-local guard FILE, so the marker " +
			"cross-check read nothing. patch 7, 8, 8b, 9, 10 and 11 each have one; if the table has been " +
			"reshaped, reshape this check with it rather than letting it pass on nothing.")
	}
}

var diffTagRE = regexp.MustCompile(`(?m)^(before|after):\s+(v[0-9][A-Za-z0-9.\-]*)`)

// gitHere reports whether this copy of the module is a git checkout we can ask about tags. A
// consumer that got the module from the proxy has no .git, and the tag check below is the only one
// that needs one.
func gitHere() bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// TestPatchesMDRegressionDiffIsForTHISTree makes true a sentence the regression-diff block already
// claimed about itself: "patches_doc_test.go checks the marker is present and that the tag named
// below is the tag this tree is at." It did not. The block shipped inside v0.6.9-sightglass.14
// carrying numbers measured at .12 and .13 and a line reading "after: v0.6.9-sightglass.12, this
// tree", and nothing objected. A stale regression diff is worse than none: it reads as a
// measurement of what ships.
func TestPatchesMDRegressionDiffIsForTHISTree(t *testing.T) {
	doc := patchesDoc(t)
	i, j := strings.Index(doc, "REGRESSION-DIFF-BEGIN"), strings.Index(doc, "REGRESSION-DIFF-END")
	if i < 0 || j < 0 {
		t.Fatal("PATCHES.md has lost its machine-findable regression-diff block")
	}
	blk := doc[i:j]
	tags := map[string]string{}
	for _, m := range diffTagRE.FindAllStringSubmatch(blk, -1) {
		if _, ok := tags[m[1]]; !ok {
			tags[m[1]] = m[2]
		}
	}
	for _, side := range []string{"before", "after"} {
		if tags[side] == "" {
			t.Errorf("the regression-diff block's %q line names no vX.Y.Z tag. A diff whose sides are not "+
				"identified by tag cannot be re-run, and cannot be checked against the tree it claims to "+
				"describe.", side)
		}
	}
	if t.Failed() {
		return
	}
	if !gitHere() {
		t.Log("this copy of the module is not a git checkout, so the tags themselves are not resolved here; " +
			"the before/after lines were still required to name them")
		return
	}
	head, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	headSHA := strings.TrimSpace(string(head))

	if err := exec.Command("git", "rev-parse", "--verify", tags["before"]+"^{commit}").Run(); err != nil {
		t.Errorf("the regression-diff block says it measured `before` at %s, and no such tag exists in this "+
			"repository. The before side has to be a published tag someone else can check out and re-run.",
			tags["before"])
	} else if err := exec.Command("git", "merge-base", "--is-ancestor", tags["before"], "HEAD").Run(); err != nil {
		t.Errorf("the regression-diff block measures `before` at %s, which is not an ancestor of HEAD. "+
			"A before side off this line of history is not a diff of this tree.", tags["before"])
	}

	out, err := exec.Command("git", "rev-parse", "--verify", tags["after"]+"^{commit}").Output()
	switch {
	case err != nil:
		// The tag does not exist yet: this is the tree that is about to be tagged as it, which is the
		// only state in which the doc can name its own tag before it is cut.
		t.Logf("the regression-diff block names `after: %s`, a tag that does not exist yet — this tree is "+
			"the one waiting to be tagged as it", tags["after"])
	case strings.TrimSpace(string(out)) != headSHA:
		t.Errorf("the regression-diff block says `after: %s`, but that tag is %s and this tree is at %s. "+
			"The numbers in the block were measured somewhere else, and nothing said so — which is how .14 "+
			"shipped a diff measured at .12.", tags["after"], strings.TrimSpace(string(out))[:12], headSHA[:12])
	}
}
