---
name: add-config-file
description: Add support for a new agent config file type to the Keel compiler. Covers the full pipeline from DSL schema through IR, accumulator, reader, writer, profile, and tests. Follow this when an agent adds a new config file that Keel should compile.
---

# Add a New Config File Type

Full pipeline walkthrough. Reference implementations: `loop.md` and `.worktreeinclude` (commit `12e9f00`).

## Prerequisites

Before building, determine:
1. **Which agent(s)** support this file? (Claude-only? Both? Cursor-only?)
2. **What scope?** Project only? User scope too?
3. **Merge strategy**: concatenate across layers (like instructions) or last-writer-wins (like overlays)?
4. **Output path**: inside the agent dir (`.claude/loop.md`) or project root (`.worktreeinclude`)?

## Step 1: Schema

**File:** `pkg/schema/schema.go`

Add a `[]FileRef` field to `KeelConfig`:
```go
MyFile []FileRef `yaml:"my-file,omitempty"`
```

Add bounds check in `Validate()` — find the `checks` slice and add:
```go
{"my-file", len(c.MyFile), 100},
```

Add to `validateFileRefFields()` — find the `collections` slice and add:
```go
{"my-file", c.MyFile},
```

## Step 2: IR

**File:** `pkg/compiler/ir.go`

Add to `LayerIR` (per-layer content blocks):
```go
MyFile []InstructionBlock
```

Add to `AccumulatedResult` (final concatenated output):
```go
MyFile string
```

## Step 3: Accumulator

**File:** `pkg/compiler/accumulator.go`

Add to `AccumulationState`:
```go
myFile []InstructionBlock
```

In `Fold()`, after existing content folding, append blocks:
```go
for _, block := range ir.MyFile {
    block.Header = header
    s.myFile = append(s.myFile, block)
}
```

In `Result()`, concatenate blocks into a string:
```go
var myFileContent string
if len(s.myFile) > 0 {
    var parts []string
    for _, block := range s.myFile {
        parts = append(parts, strings.TrimSpace(block.Content))
    }
    myFileContent = strings.Join(parts, "\n\n") // or "\n" for line-per-entry files
}
```

Add to the returned `AccumulatedResult`:
```go
MyFile: myFileContent,
```

## Step 4: Profile capabilities

**File:** `pkg/compiler/profile.go`

Add to `Capabilities`:
```go
MyFile bool
```

Add output path field to `AgentProfile`:
```go
MyFileOutput string // relative to outputDir, e.g. ".claude/my-file.md"
```

## Step 5: Agent profile

**File:** `pkg/compiler/claude/claude.go` (or cursor equivalent)

In the profile constructor, set the capability and output path:
```go
Capabilities: compiler.Capabilities{
    MyFile: true,
    // ... existing ...
},
MyFileOutput: ".claude/my-file.md",
```

If the file is project-scope only, clear it in `UserProfile()`:
```go
p.MyFileOutput = ""
```

## Step 6: Capability warnings

**File:** `pkg/compiler/accumulator.go`

Add to `capChecks` table:
```go
{"my-file", func(c *schema.KeelConfig) bool { return len(c.MyFile) > 0 }, func(cap Capabilities) bool { return cap.MyFile }},
```

Add IR-level lossy warning in `emitLossyWarnings`:
```go
if len(ir.MyFile) > 0 && p.MyFileOutput == "" {
    slog.Warn("my-file has no equivalent in target agent, skipping",
        "target", p.Name, "layer", header)
}
```

## Step 7: DSL reader

**File:** `pkg/compiler/reader_dsl.go`

In `Read()`, add a call after existing reads:
```go
r.readMyFile(layerPath, cfg, ir)
```

Add the method:
```go
func (r *KeelDSLReader) readMyFile(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
    paths := resolveInstructionFiles(layerPath, cfg.MyFile)
    for _, p := range paths {
        data, err := pathutil.SafeReadFile(p)
        if err != nil {
            slog.Debug("could not read my-file", "path", p, "error", err)
            continue
        }
        ir.MyFile = append(ir.MyFile, InstructionBlock{
            Filename: filepath.Base(p),
            Content:  string(data),
        })
    }
}
```

## Step 8: Native reader

**File:** `pkg/compiler/claude/reader.go`

In `Read()`, add a call:
```go
r.readMyFile(layerPath, ir)
```

Add the method (check agent dir then layer root):
```go
func (r *Reader) readMyFile(layerPath string, ir *compiler.LayerIR) {
    candidates := []string{
        filepath.Join(layerPath, ".claude", "my-file.md"),
        filepath.Join(layerPath, "my-file.md"),
    }
    for _, path := range candidates {
        data, err := pathutil.SafeReadFile(path)
        if err != nil {
            continue
        }
        ir.MyFile = []compiler.InstructionBlock{
            {Filename: "my-file.md", Content: string(data)},
        }
        return
    }
}
```

## Step 9: Writer

**File:** `pkg/compiler/claude/writer.go`

In `Write()`, add output:
```go
if profile.MyFileOutput != "" && result.MyFile != "" {
    myFilePath := filepath.Join(outputDir, profile.MyFileOutput)
    if err := os.MkdirAll(filepath.Dir(myFilePath), 0755); err != nil {
        return nil, err
    }
    if err := os.WriteFile(myFilePath, []byte(result.MyFile+"\n"), 0644); err != nil {
        return nil, err
    }
    records = append(records, schema.FileRecord{
        Output:   profile.MyFileOutput,
        Strategy: "concatenate",
    })
}
```

## Step 10: MergeWithDefaults

**File:** `pkg/schema/defaults.go`

In `MergeWithDefaults`, add:
```go
result.MyFile = firstNonNilFileRefSlice(userConfig.MyFile, defaults.MyFile)
```

## Step 11: Tests

Add to fixture:
1. Create source file: `examples/compiler/layers/01-keel-base/my-file.md`
2. Reference in keel.yml: `my-file: [source: my-file.md]`
3. Add fixture test: file exists, content correct, not present for other agents, warnings emitted

Test pattern for agent-only files:
```go
func TestFixture_MyFile_Claude(t *testing.T) {
    outputDir, _ := compileTo(t, "claude", compiler.CompileOptions{})
    t.Run("Exists", func(t *testing.T) { assert.True(t, testutil.FileExists(...)) })
    t.Run("Content", func(t *testing.T) { assert.Contains(t, readFile(...), "expected content") })
}

func TestFixture_MyFile_Cursor(t *testing.T) {
    capture, outputDir, _ := captureLogsForCompile(t, "cursor", compiler.CompileOptions{})
    t.Run("NotPresent", func(t *testing.T) { assert.False(t, testutil.FileExists(...)) })
    t.Run("WarnUnsupported", func(t *testing.T) { assert.True(t, capture.contains("my-file")) })
}
```

## Step 12: Verify

```bash
go build ./...
go test -count=1 ./...
```

## Reference implementations

- Loop prompt: commit `12e9f00` — `.claude/loop.md`, concatenated, Claude-only
- Worktree include: same commit — `.worktreeinclude`, at project root, project-scope only
