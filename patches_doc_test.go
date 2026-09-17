// PATCHES.md is this fork's provenance record, and it has been WRONG in published tags: it named
// the wrong upstream base (v0.6.8 for a tree based on v0.6.9), described a `replace` directive and a
// third_party/ tree that never existed here, claimed "0 test files" for a tree carrying 80, printed
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

// TestPatchesMDTestFileCountIsTrue pins the count in the front matter. The claim it replaced —
// "stripped of *_test.go ... 0 test files" — was not merely stale: the retained upstream suite is
// what found patch 4b, so a future maintainer who believed it would delete the thing that catches
// the defects.
func TestPatchesMDTestFileCountIsTrue(t *testing.T) {
	doc := patchesDoc(t)
	m := regexp.MustCompile(`\*\*(\d+) ` + "`" + `\*_test\.go` + "`" + ` files: (\d+) from upstream [^,]+, of which (\d+) carry nothing but the\nmodule-path rewrite and (\d+) are edited, plus (\d+) added by this fork\.\*\*`).FindStringSubmatch(doc)
	if m == nil {
		t.Fatal(`PATCHES.md no longer states how many *_test.go files this tree carries, how many come from ` +
			`upstream and how many the fork adds. That sentence exists because its predecessor ("0 test ` +
			`files") was false and the Maintenance section turned it into an instruction to delete them.`)
	}
	total, _ := strconv.Atoi(m[1])
	upstream, _ := strconv.Atoi(m[2])
	untouched, _ := strconv.Atoi(m[3])
	edited, _ := strconv.Atoi(m[4])
	added, _ := strconv.Atoi(m[5])
	if untouched+edited != upstream {
		t.Errorf("PATCHES.md splits the %d upstream test files into %d untouched + %d edited, which is %d",
			upstream, untouched, edited, untouched+edited)
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
}

// retracted are claims and instructions this file published in tags that are now known to be false
// or harmful. They are allowed to APPEAR — the tags that carried them are public and cannot be
// edited, so the correction has to stay visible in the tree that supersedes them — but only inside a
// paragraph that retracts them. Reintroducing one as an assertion, in a paragraph that does not say
// it is wrong, is the failure this pins.
var retracted = []struct {
	claim string
	why   string
}{
	{"stream-map leak",
		"FALSE. It is true only of transportResponseBody.Close(). On the context-cancel path " +
			"(awaitRequestCancel -> cancelStream) the inverted condition sent no RST_STREAM AND called no " +
			"cc.forgetStreamID, so the clientStream stayed in cc.streams for the life of the connection. " +
			"Measured on the shipped path by parity.TestParityHTTP2CancelledStreamsDoNotConsumeSlots: 100 " +
			"context-cancelled requests fill ChromeInitialMaxConcurrentStreams and the 101st never goes out."},
	{"v0.6.8",
		"FALSE twice over: the base is v0.6.9, and this is a fork with a rewritten module path, not a " +
			"vendored tree."},
	{"replace github.com/bogdanfinn/fhttp",
		"FALSE. go/go.mod has zero replace directives and go/AGENTS.md forbids them, because a directory " +
			"replace does not propagate to dependents."},
	{"0 test files",
		"FALSE. The tree carries 81, and the retained upstream suite is what found patch 4b."},
	{"strip tests",
		"HARMFUL. It was an instruction in the Maintenance section; following it deletes the suite that " +
			"catches the defects."},
	{"re-copy upstream",
		"HARMFUL. This is a fork with a rewritten module path across 86 files; a re-copy reverts the " +
			"rewrite and the tree stops compiling against go/go.mod. Merge the upstream tag instead."},
	{"re-apply the hunks above",
		"HARMFUL. The Maintenance section that said it sat in the MIDDLE of the file, so \"above\" " +
			"excluded patches 4b, 6, 7, 8, 8b and 10 — six of the fourteen entries."},
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
var retractionMarkers = []string{"FALSE", "HARMFUL", "RETRACTED", "RETRACTION"}

// retractionWindow is how close a marker has to be to the claim it retracts. Presence anywhere in
// the paragraph was the second half of the hole: a long paragraph can retract one thing and assert
// another, and the guard could not tell which sentence the marker belonged to.
const retractionWindow = 400

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

// markerNear reports whether one of the retraction markers sits within retractionWindow bytes of the
// claim occurrence at `at`.
func markerNear(para string, at, n int) bool {
	lo, hi := at-retractionWindow, at+n+retractionWindow
	if lo < 0 {
		lo = 0
	}
	if hi > len(para) {
		hi = len(para)
	}
	win := para[lo:hi]
	for _, w := range retractionMarkers {
		if strings.Contains(win, w) {
			return true
		}
	}
	return false
}

// TestPatchesMDDoesNotRepeatItsRetractedClaims fails if any retracted claim appears anywhere that
// does not retract it, ADJACENTLY. Three conditions, and all three were needed:
//
//	(1) the claim must still appear at all — the published tags carrying it cannot be edited, so the
//	    correction has to live in the tree that supersedes them;
//	(2) EVERY occurrence must have a retraction marker within retractionWindow bytes of it, in the
//	    same paragraph;
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
				if !markerNear(para, at, len(r.claim)) {
					t.Errorf("PATCHES.md states the retracted claim %q with no retraction marker %v within "+
						"%d bytes of it:\n  ...%s...\nWhy it is retracted: %s",
						r.claim, retractionMarkers, retractionWindow, squash(para), r.why)
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

// TestPatchesMDGuardTableNamesFilesThatExist pins the other half of "a guard cannot be cited into
// existence". TestPatchesMDNamesGuardsThatExist checks TEST NAMES and exempts anything spelled
// `parity.X`; it never checked the fork-side FILE names in the same table. An adversarial reader
// added a row naming a fork test file and a parity test of which NEITHER exists, and all ten guards
// stayed green. This check covers the file half; the Sightglass half — that every `parity.Test*`
// this file names resolves to a real test in go/ — is checked there, by
// parity.TestForkPATCHESNamesSightglassGuardsThatExist, because the fork has no view of Sightglass.
func TestPatchesMDGuardTableNamesFilesThatExist(t *testing.T) {
	doc := patchesDoc(t)
	sec := section(t, doc, "## Where each patch is guarded")
	re := regexp.MustCompile("`([A-Za-z0-9_./-]+\\.go)`")
	seen := map[string]bool{}
	checked := 0
	for _, m := range re.FindAllStringSubmatch(sec, -1) {
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
