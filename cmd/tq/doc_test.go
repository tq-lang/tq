package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// docExample holds a parsed ```tq code block from a markdown file.
type docExample struct {
	file       string
	lineNumber int
	script     string
	wantStdout string
	wantStderr string
	wantExit   int
}

// docExecTimeout is the maximum wall-clock time any single doc example may run.
const docExecTimeout = 10 * time.Second

func TestDocs(t *testing.T) {
	docsDir := findDocsDir(t)
	if docsDir == "" {
		t.Skip("docs directory not found")
	}

	mdFiles := findMarkdownFiles(t, docsDir)
	if len(mdFiles) == 0 {
		t.Skip("no markdown files found in docs/")
	}

	var examples []docExample
	for _, f := range mdFiles {
		parsed, parseErr := parseTQBlocks(f)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", f, parseErr)
		}
		examples = append(examples, parsed...)
	}

	if len(examples) == 0 {
		t.Skip("no ```tq blocks found in docs/")
	}

	for _, ex := range examples {
		ex := ex
		name := fmt.Sprintf("%s:%d", filepath.Base(ex.file), ex.lineNumber)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runDocExample(t, ex)
		})
	}
}

// findDocsDir locates the docs/ directory relative to the module root.
func findDocsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			candidate := filepath.Join(dir, "docs")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// findMarkdownFiles recursively finds all .md files under dir.
func findMarkdownFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs dir: %v", err)
	}
	return files
}

// parseTQBlocks scans a markdown file and returns all docExample values found
// inside fenced ```tq ... ``` blocks.
func parseTQBlocks(filename string) ([]docExample, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")

	var examples []docExample
	inBlock := false
	var blockLines []string
	blockStart := 0

	for i, line := range lines {
		lineNum := i + 1
		if !inBlock {
			if strings.TrimSpace(line) == "```tq" {
				inBlock = true
				blockStart = lineNum
				blockLines = nil
			}
			continue
		}
		if strings.TrimSpace(line) == "```" {
			ex := parseBlock(filename, blockStart, blockLines)
			examples = append(examples, ex)
			inBlock = false
			continue
		}
		blockLines = append(blockLines, line)
	}

	return examples, nil
}

// parseBlock converts the raw lines of a ```tq block into a docExample.
func parseBlock(file string, startLine int, lines []string) docExample {
	const (
		sectionScript = iota
		sectionStdout
		sectionStderr
	)

	ex := docExample{file: file, lineNumber: startLine}
	section := sectionScript

	var scriptLines, stdoutLines, stderrLines []string

	for _, line := range lines {
		if code, ok := parseSectionMarker(line, "# output error"); ok {
			if code >= 0 {
				ex.wantExit = code
			}
			section = sectionStderr
			continue
		}
		if code, ok := parseSectionMarker(line, "# output"); ok {
			if code >= 0 {
				ex.wantExit = code
			}
			section = sectionStdout
			continue
		}

		switch section {
		case sectionScript:
			scriptLines = append(scriptLines, line)
		case sectionStdout:
			stdoutLines = append(stdoutLines, line)
		case sectionStderr:
			stderrLines = append(stderrLines, line)
		}
	}

	ex.script = strings.Join(scriptLines, "\n")
	ex.wantStdout = strings.TrimSpace(strings.Join(stdoutLines, "\n"))
	ex.wantStderr = strings.TrimSpace(strings.Join(stderrLines, "\n"))

	return ex
}

// parseSectionMarker detects "# <prefix>" and "# <prefix> (exit: N)".
// Returns (exitCode, true) when matched; exitCode is -1 when no exit code specified.
func parseSectionMarker(line, prefix string) (int, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == prefix {
		return -1, true
	}
	if after, ok := strings.CutPrefix(trimmed, prefix+" (exit: "); ok {
		if rest, ok2 := strings.CutSuffix(after, ")"); ok2 {
			if n, err := strconv.Atoi(rest); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

// runDocExample interprets the script from a docExample directly in Go.
// No shell is involved — only tq, echo, printf, and cat-heredoc are supported,
// connected by pipes. Security is structural: the interpreter cannot execute
// anything outside this fixed set.
func runDocExample(t *testing.T, ex docExample) {
	t.Helper()

	workDir := t.TempDir()
	pipeline, err := parseScript(ex.script)
	if err != nil {
		t.Fatalf("parse script at %s:%d: %v", ex.file, ex.lineNumber, err)
	}

	gotStdout, gotStderr, gotExit := execPipeline(t, pipeline, workDir)

	if gotExit != ex.wantExit {
		t.Errorf("exit code = %d, want %d\nstdout: %s\nstderr: %s",
			gotExit, ex.wantExit, gotStdout, gotStderr)
	}
	if ex.wantStdout != "" && gotStdout != ex.wantStdout {
		t.Errorf("stdout mismatch\ngot:  %q\nwant: %q", gotStdout, ex.wantStdout)
	}
	if ex.wantStderr != "" && !strings.Contains(gotStderr, ex.wantStderr) {
		t.Errorf("stderr %q does not contain %q", gotStderr, ex.wantStderr)
	}
}

// scriptStep is one logical operation in a doc script.
type scriptStep struct {
	// For heredoc file creation: write content to filename in workDir.
	heredocFile    string
	heredocContent string

	// For a command pipeline: sequence of piped commands.
	// Each command is []string{program, args...}.
	commands [][]string
}

// parseScript parses the script section into a sequence of steps.
// Supported constructs:
//   - cat <<'EOF' > filename / content / EOF
//   - cmd1 | cmd2 | ... (where each cmd is tq, echo, or printf)
func parseScript(script string) ([]scriptStep, error) {
	lines := strings.Split(script, "\n")
	var steps []scriptStep

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Heredoc: cat <<'EOF' > filename
		if strings.HasPrefix(line, "cat <<") {
			filename, content, advance, err := parseHeredoc(lines, i)
			if err != nil {
				return nil, err
			}
			steps = append(steps, scriptStep{heredocFile: filename, heredocContent: content})
			i += advance
			continue
		}

		// Command pipeline: cmd1 args | cmd2 args | ...
		cmds, err := parsePipeline(line)
		if err != nil {
			return nil, fmt.Errorf("line %q: %w", line, err)
		}
		steps = append(steps, scriptStep{commands: cmds})
	}

	return steps, nil
}

// parseHeredoc parses a cat <<'EOF' > filename block starting at lines[start].
// Returns the filename, content, and how many lines to advance past.
func parseHeredoc(lines []string, start int) (string, string, int, error) {
	line := strings.TrimSpace(lines[start])

	// Extract filename from: cat <<'EOF' > filename
	gtIdx := strings.LastIndex(line, ">")
	if gtIdx < 0 {
		return "", "", 0, fmt.Errorf("heredoc missing > redirect: %q", line)
	}
	filename := strings.TrimSpace(line[gtIdx+1:])
	if filename == "" {
		return "", "", 0, fmt.Errorf("heredoc missing filename: %q", line)
	}
	if strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		return "", "", 0, fmt.Errorf("heredoc filename must be relative without slashes: %q", filename)
	}

	// Collect body until EOF
	var body []string
	i := start + 1
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "EOF" {
			break
		}
		body = append(body, lines[i])
	}
	if i >= len(lines) {
		return "", "", 0, fmt.Errorf("heredoc missing closing EOF")
	}

	return filename, strings.Join(body, "\n") + "\n", i - start, nil
}

var allowedCommands = map[string]bool{
	"tq":     true,
	"echo":   true,
	"printf": true,
}

// parsePipeline splits "cmd1 args | cmd2 args" into [][]string.
func parsePipeline(line string) ([][]string, error) {
	segments := splitPipe(line)
	var cmds [][]string
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		args := shellSplit(seg)
		if len(args) == 0 {
			continue
		}
		if !allowedCommands[args[0]] {
			return nil, fmt.Errorf("disallowed command %q", args[0])
		}
		cmds = append(cmds, args)
	}
	if len(cmds) == 0 {
		return nil, fmt.Errorf("empty pipeline")
	}
	return cmds, nil
}

// splitShell splits s on unquoted occurrences of the separator rune.
// Quotes (single and double) and backslash escapes are honoured.
// When sep is '|', it splits a pipeline; when sep is ' ', it splits arguments.
// stripQuotes controls whether quote characters are removed from the output.
func splitShell(s string, sep rune, stripQuotes bool) []string {
	var parts []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	hasContent := false

	for _, ch := range s {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			hasContent = true
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			if !stripQuotes {
				current.WriteRune(ch)
			}
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			hasContent = true
			if !stripQuotes {
				current.WriteRune(ch)
			}
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			hasContent = true
			if !stripQuotes {
				current.WriteRune(ch)
			}
			continue
		}
		if ch == sep && !inSingle && !inDouble {
			// For whitespace splitting, collapse consecutive separators.
			if sep == ' ' || sep == '\t' {
				if hasContent {
					parts = append(parts, current.String())
					current.Reset()
					hasContent = false
				}
				continue
			}
			parts = append(parts, current.String())
			current.Reset()
			hasContent = false
			continue
		}
		// Treat tab as space for argument splitting.
		if (ch == ' ' || ch == '\t') && sep == ' ' && !inSingle && !inDouble {
			if hasContent {
				parts = append(parts, current.String())
				current.Reset()
				hasContent = false
			}
			continue
		}
		current.WriteRune(ch)
		hasContent = true
	}
	if hasContent || sep == '|' {
		parts = append(parts, current.String())
	}
	return parts
}

// splitPipe splits a command line on unquoted pipe characters.
func splitPipe(line string) []string {
	return splitShell(line, '|', false)
}

// shellSplit does minimal shell-like argument splitting, handling single
// and double quotes. No variable expansion, no globbing — by design.
func shellSplit(s string) []string {
	return splitShell(s, ' ', true)
}

// execPipeline runs a sequence of script steps and returns the final stdout,
// combined stderr, and exit code.
func execPipeline(t *testing.T, steps []scriptStep, workDir string) (stdout, stderr string, exitCode int) {
	t.Helper()

	var combinedStderr strings.Builder

	for _, step := range steps {
		// Heredoc: write file
		if step.heredocFile != "" {
			path := filepath.Join(workDir, step.heredocFile)
			if err := os.WriteFile(path, []byte(step.heredocContent), 0644); err != nil {
				t.Fatalf("write heredoc file %s: %v", step.heredocFile, err)
			}
			continue
		}

		// Command pipeline
		out, serr, code := execCommandPipeline(t, step.commands, workDir)
		combinedStderr.WriteString(serr)
		if code != 0 || step.commands == nil {
			return strings.TrimSpace(out), strings.TrimSpace(combinedStderr.String()), code
		}
		stdout = out
	}

	return strings.TrimSpace(stdout), strings.TrimSpace(combinedStderr.String()), 0
}

// execCommandPipeline runs a pipe chain like [["echo","hello"],["tq",".name"]]
// and returns stdout, stderr, exit code of the last command.
func execCommandPipeline(t *testing.T, cmds [][]string, workDir string) (string, string, int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), docExecTimeout)
	defer cancel()

	env := []string{
		"HOME=" + workDir,
		"TMPDIR=" + workDir,
		"LC_ALL=C",
	}
	if coverDir != "" {
		env = append(env, "GOCOVERDIR="+coverDir)
	}

	var stderrBufs []*bytes.Buffer
	var prevOut *bytes.Buffer

	collectStderr := func() string {
		var b strings.Builder
		for _, buf := range stderrBufs {
			b.Write(buf.Bytes())
		}
		return b.String()
	}

	handleErr := func(err error, argv []string, stdout string) (string, string, int) {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout, collectStderr(), exitErr.ExitCode()
		}
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("TIMEOUT after %v", docExecTimeout)
		}
		t.Fatalf("exec %v: %v", argv, err)
		return "", "", 0 // unreachable
	}

	for _, argv := range cmds {
		prog := resolveProg(t, argv[0])

		cmd := exec.CommandContext(ctx, prog, argv[1:]...)
		cmd.Dir = workDir
		cmd.Env = env
		if prevOut != nil {
			cmd.Stdin = prevOut
		}

		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		stderrBufs = append(stderrBufs, &errBuf)

		if err := cmd.Run(); err != nil {
			return handleErr(err, argv, outBuf.String())
		}
		prevOut = &outBuf
	}

	return prevOut.String(), collectStderr(), 0
}

// resolveProg maps a command name to an executable path.
// "tq" resolves to the test binary; everything else resolves via PATH.
func resolveProg(t *testing.T, name string) string {
	t.Helper()
	if name == "tq" {
		return binaryPath
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("command not found: %s", name)
	}
	return resolved
}
