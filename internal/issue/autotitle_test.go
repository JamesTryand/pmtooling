package issue

import "testing"

func TestNextAutoTitleNoExisting(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "x")

	got, err := NextAutoTitle(dir, "bug", 4)
	if err != nil {
		t.Fatalf("NextAutoTitle: %v", err)
	}
	if got != "0001" {
		t.Errorf("NextAutoTitle = %q, want 0001", got)
	}
}

func TestNextAutoTitleSequential(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "x")
	makeBranch(t, dir, "bug/0001")
	makeBranch(t, dir, "bug/0002")

	got, err := NextAutoTitle(dir, "bug", 4)
	if err != nil {
		t.Fatalf("NextAutoTitle: %v", err)
	}
	if got != "0003" {
		t.Errorf("NextAutoTitle = %q, want 0003", got)
	}
}

func TestNextAutoTitleSkipsNonNumeric(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "x")
	makeBranch(t, dir, "bug/investigate-perf")
	makeBranch(t, dir, "bug/0001")

	got, err := NextAutoTitle(dir, "bug", 4)
	if err != nil {
		t.Fatalf("NextAutoTitle: %v", err)
	}
	if got != "0002" {
		t.Errorf("NextAutoTitle = %q, want 0002 (non-numeric sibling should be skipped)", got)
	}
}

func TestNextAutoTitleIsMaxPlusOneNotCountPlusOne(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "x")
	makeBranch(t, dir, "bug/0001")
	makeBranch(t, dir, "bug/0005") // gap: only 2 branches exist, but max is 5

	got, err := NextAutoTitle(dir, "bug", 4)
	if err != nil {
		t.Fatalf("NextAutoTitle: %v", err)
	}
	if got != "0006" {
		t.Errorf("NextAutoTitle = %q, want 0006 (max+1, not count+1 which would be 0003)", got)
	}
}

func TestNextAutoTitleCustomPadWidth(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "x")

	got, err := NextAutoTitle(dir, "bug", 6)
	if err != nil {
		t.Fatalf("NextAutoTitle: %v", err)
	}
	if got != "000001" {
		t.Errorf("NextAutoTitle = %q, want 000001", got)
	}
}

func TestNextAutoTitleWidensPastPadWidth(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "x")
	makeBranch(t, dir, "bug/9999")

	got, err := NextAutoTitle(dir, "bug", 4)
	if err != nil {
		t.Fatalf("NextAutoTitle: %v", err)
	}
	if got != "10000" {
		t.Errorf("NextAutoTitle = %q, want 10000 (widens past the pad width, not truncated)", got)
	}
}

func TestNextAutoTitleIsolatedByType(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f.txt", "x")
	makeBranch(t, dir, "bug/0001")
	makeBranch(t, dir, "bug/0002")

	got, err := NextAutoTitle(dir, "feature", 4)
	if err != nil {
		t.Fatalf("NextAutoTitle: %v", err)
	}
	if got != "0001" {
		t.Errorf("NextAutoTitle(feature) = %q, want 0001 (numbering scoped per-type, unaffected by bug/*)", got)
	}
}
