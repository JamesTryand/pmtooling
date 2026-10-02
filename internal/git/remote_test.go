package git

import (
	"strings"
	"testing"
)

func TestScrubURLs(t *testing.T) {
	cases := []struct{ name, in string }{
		{"https with token", "fatal: unable to access 'https://user:SECRET@host.example/org/x.git/': timed out"},
		{"https no creds", "remote: see https://host.example/org/x.git for details"},
		{"ssh scheme", "fatal: Could not read from ssh://git:SECRET@host.example:2222/x.git"},
		{"scp-like", "fatal: git@host.example:org/x.git does not appear to be a repository"},
		{"bare credential host", "error: user:SECRET@host.example rejected"},
		{"connect failure", "fatal: Failed to connect to host.example port 443 after 21 ms"},
		{"resolve failure", "fatal: Could not resolve host: host.example"},
	}
	for _, c := range cases {
		got := ScrubURLs(c.in)
		for _, leak := range []string{"SECRET", "host.example"} {
			if strings.Contains(got, leak) {
				t.Errorf("%s: ScrubURLs(%q) = %q still contains %q", c.name, c.in, got, leak)
			}
		}
		if !strings.Contains(got, "<remote>") {
			t.Errorf("%s: ScrubURLs(%q) = %q has no <remote> marker", c.name, c.in, got)
		}
	}
	if got := ScrubURLs("nothing to hide here"); got != "nothing to hide here" {
		t.Errorf("ScrubURLs changed plain text: %q", got)
	}
}

// TestFetchFailureLeaksNoCredentials drives a real git failure against a
// remote whose URL embeds a credential and checks the user-visible error.
func TestFetchFailureLeaksNoCredentials(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "x")
	const url = "https://user:SECRET@127.0.0.1:1/org/x.git"
	if _, err := Run(dir, "remote", "add", "origin", url); err != nil {
		t.Fatal(err)
	}
	err := FetchRemote(dir, "origin")
	if err == nil {
		t.Fatal("expected fetch to an unreachable remote to fail")
	}
	for _, leak := range []string{"SECRET", "127.0.0.1", "user:"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error %q leaks %q", err.Error(), leak)
		}
	}
}

// remoteFixture is a work repo with a local bare repo as origin. The bare
// repo is a filesystem path, so tests never need a credentialed URL.
type remoteFixture struct {
	work, bare, def string
}

func newRemoteFixture(t *testing.T) remoteFixture {
	t.Helper()
	bare := t.TempDir()
	if _, err := Run(bare, "init", "-q", "--bare"); err != nil {
		t.Fatal(err)
	}
	work := initRepo(t)
	commitFile(t, work, "base.txt", "base")
	def, _, err := CurrentBranch(work)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(work, "remote", "add", "origin", bare); err != nil {
		t.Fatal(err)
	}
	if err := Push(work, "origin", def); err != nil {
		t.Fatalf("seed push: %v", err)
	}
	return remoteFixture{work: work, bare: bare, def: def}
}

// second returns another clone of the same remote, for making the remote
// move independently of the first work repo.
func (f remoteFixture) second(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := Run(dir, "clone", "-q", f.bare, "."); err != nil {
		t.Fatalf("clone: %v", err)
	}
	for k, v := range map[string]string{"user.email": "test@example.com", "user.name": "pmt test"} {
		if _, err := Run(dir, "config", k, v); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRemoteExists(t *testing.T) {
	f := newRemoteFixture(t)
	if ok, err := RemoteExists(f.work, "origin"); err != nil || !ok {
		t.Errorf("RemoteExists(origin) = %v, %v; want true", ok, err)
	}
	if ok, err := RemoteExists(f.work, "nope"); err != nil || ok {
		t.Errorf("RemoteExists(nope) = %v, %v; want false", ok, err)
	}
}

func TestClassify(t *testing.T) {
	f := newRemoteFixture(t)
	w := f.work

	want := func(branch string, state RelState) Relation {
		t.Helper()
		rel, err := Classify(w, "origin", branch)
		if err != nil {
			t.Fatalf("Classify(%s): %v", branch, err)
		}
		if rel.State != state {
			t.Fatalf("Classify(%s) = %v, want %v", branch, rel.State, state)
		}
		return rel
	}

	want(f.def, RelUpToDate)

	// ahead: a local commit not yet pushed
	commitFile(t, w, "a.txt", "a")
	rel := want(f.def, RelAhead)
	if !strings.Contains(rel.Range(), "..") || strings.Contains(rel.Range(), "...") {
		t.Errorf("ahead range = %q, want a..b", rel.Range())
	}

	// no-remote: a branch that was never pushed
	if _, err := Run(w, "branch", "bug/new"); err != nil {
		t.Fatal(err)
	}
	want("bug/new", RelNoRemote)

	// behind: the remote moves, local fetches
	if err := Push(w, "origin", f.def); err != nil {
		t.Fatal(err)
	}
	other := f.second(t)
	commitFile(t, other, "b.txt", "b")
	if _, err := Run(other, "push", "-q", "origin", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if err := FetchRemote(w, "origin"); err != nil {
		t.Fatal(err)
	}
	want(f.def, RelBehind)

	// diverged: local adds a commit too
	commitFile(t, w, "c.txt", "c")
	rel = want(f.def, RelDiverged)
	if !strings.Contains(rel.Range(), "...") {
		t.Errorf("diverged range = %q, want a...b", rel.Range())
	}

	// no-local: a branch only the remote has
	if _, err := Run(other, "push", "-q", "origin", "HEAD:refs/heads/bug/remote-only"); err != nil {
		t.Fatal(err)
	}
	if err := FetchRemote(w, "origin"); err != nil {
		t.Fatal(err)
	}
	want("bug/remote-only", RelNoLocal)

	if _, err := Classify(w, "origin", "does/not-exist"); err == nil {
		t.Error("Classify of a branch that exists nowhere should error")
	}
}

func TestPushNeverForces(t *testing.T) {
	f := newRemoteFixture(t)
	other := f.second(t)
	commitFile(t, other, "other.txt", "o")
	if _, err := Run(other, "push", "-q", "origin", "HEAD"); err != nil {
		t.Fatal(err)
	}

	before, err := Run(f.bare, "rev-parse", f.def)
	if err != nil {
		t.Fatal(err)
	}
	commitFile(t, f.work, "mine.txt", "m")
	if err := Push(f.work, "origin", f.def); err == nil {
		t.Fatal("expected a non-fast-forward push to fail, not be forced")
	}
	after, err := Run(f.bare, "rev-parse", f.def)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("remote moved on a rejected push: %s -> %s", before, after)
	}
}

func TestPushSetsUpstreamForNewBranch(t *testing.T) {
	f := newRemoteFixture(t)
	if _, err := Run(f.work, "branch", "bug/new"); err != nil {
		t.Fatal(err)
	}
	if err := Push(f.work, "origin", "bug/new"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := RefExists(f.bare, "refs/heads/bug/new"); !ok {
		t.Error("branch missing on remote after Push")
	}
	if up, err := Run(f.work, "rev-parse", "--abbrev-ref", "bug/new@{upstream}"); err != nil || up != "origin/bug/new" {
		t.Errorf("upstream = %q, %v; want origin/bug/new", up, err)
	}
}

// TestLsRemoteHeadsAndRemoteMoved covers the no-fetch path push relies on:
// reading the remote's tips directly, and classifying a remote tip whose
// commit this repo has never fetched.
func TestLsRemoteHeadsAndRemoteMoved(t *testing.T) {
	f := newRemoteFixture(t)
	other := f.second(t)
	commitFile(t, other, "b.txt", "b")
	if _, err := Run(other, "push", "-q", "origin", "HEAD"); err != nil {
		t.Fatal(err)
	}

	before, err := Run(f.work, "for-each-ref", "refs/remotes")
	if err != nil {
		t.Fatal(err)
	}
	tips, err := LsRemoteHeads(f.work, "origin")
	if err != nil {
		t.Fatal(err)
	}
	after, _ := Run(f.work, "for-each-ref", "refs/remotes")
	if before != after {
		t.Errorf("LsRemoteHeads changed remote-tracking refs:\n%s\n--\n%s", before, after)
	}

	want, _ := Run(other, "rev-parse", "HEAD")
	if tips[f.def] != want {
		t.Fatalf("tip for %s = %q, want %q", f.def, tips[f.def], want)
	}

	local, _ := RevParse(f.work, "refs/heads/"+f.def)
	rel, err := ClassifyTips(f.work, local, tips[f.def])
	if err != nil {
		t.Fatal(err)
	}
	if rel.State != RelRemoteMoved {
		t.Errorf("ClassifyTips with an unfetched remote tip = %v, want remote-moved", rel.State)
	}
}
