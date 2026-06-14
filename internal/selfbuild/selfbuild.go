// Package selfbuild is the disciplined half of self-modification: it takes a
// proposed change to a source tree and runs it through the build-and-deploy
// pipeline already used by hand, copy, apply, compile, test, stage, so a bad
// change is caught before it can reach a running mind. It is the innate
// guardrail under self-extension; what GOLEM becomes is learned and grounded
// above it, change by change in the glass box.
//
// The pipeline never edits the original tree and never swaps the live binary.
// It works on an isolated copy and produces a candidate binary off to the side;
// applying the fitness gate and the actual swap are the caller's, behind the
// operator's say-so. A change that fails to compile or test is rejected with
// the stage it died at, and the original stands untouched.
package selfbuild

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Change is a proposed edit to one file of the source tree: the file relative
// to the root, the transform over its current contents, and why it is proposed,
// for the trail.
type Change struct {
	File  string
	Apply func(source string) (string, error)
	Why   string
}

// Result is the pipeline's verdict: whether the change built and tested clean,
// the stage it reached or died at, the staged candidate binary when accepted,
// and the error otherwise.
type Result struct {
	Accepted  bool
	Stage     string // copy, apply, build, test, tested
	Err       string
	Candidate string
}

// Build runs a proposed change through the pipeline in isolation. It copies the
// source tree to a working directory, applies the change there, compiles the
// main package to a candidate binary in the stage directory, and runs the
// tests; the original tree is never touched and the live binary never swapped.
// On success the candidate stands ready off to the side for the caller to gate
// and deploy; on failure the stage names where it died.
func Build(ctx context.Context, srcDir, stageDir, mainPkg, binName string, ch Change) Result {
	work, err := os.MkdirTemp(stageDir, "selfbuild-")
	if err != nil {
		return Result{Stage: "copy", Err: err.Error()}
	}
	defer os.RemoveAll(work)
	if err := copyTree(srcDir, work); err != nil {
		return Result{Stage: "copy", Err: err.Error()}
	}

	path := filepath.Join(work, ch.File)
	old, err := os.ReadFile(path)
	if err != nil {
		return Result{Stage: "apply", Err: err.Error()}
	}
	edited, err := ch.Apply(string(old))
	if err != nil {
		return Result{Stage: "apply", Err: err.Error()}
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		return Result{Stage: "apply", Err: err.Error()}
	}

	candidate := filepath.Join(stageDir, binName)
	if out, err := goRun(ctx, work, "build", "-o", candidate, mainPkg); err != nil {
		return Result{Stage: "build", Err: summarize(out, err)}
	}
	if out, err := goRun(ctx, work, "test", "./..."); err != nil {
		return Result{Stage: "test", Err: summarize(out, err)}
	}
	return Result{Accepted: true, Stage: "tested", Candidate: candidate}
}

// goRun invokes the toolchain in a working directory, the compiling backend the
// self-build needs where the interpreter cannot reach: it builds and tests a
// whole tree, not a snippet.
func goRun(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// copyTree mirrors a source tree into a fresh directory, skipping the version
// control directory, so the pipeline works on its own copy and the original is
// never at risk.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// summarize trims a toolchain failure to its last lines plus the exit error, so
// the trail carries the reason without the whole build log.
func summarize(out string, err error) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 6 {
		lines = lines[len(lines)-6:]
	}
	tail := strings.TrimSpace(strings.Join(lines, "\n"))
	if tail == "" {
		return err.Error()
	}
	return fmt.Sprintf("%s (%v)", tail, err)
}
