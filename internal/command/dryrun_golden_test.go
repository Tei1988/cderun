package command

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"cderun/internal/config"
	"cderun/internal/logging"
	"cderun/internal/runtime"
)

var update = flag.Bool("update", false, "update golden files")

func TestGolden_DryRun(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata", "dryrun")
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("testdata/dryrun not found")
		}
		t.Fatal(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			dir := filepath.Join(testdataDir, entry.Name())
			runGoldenTest(t, dir)
		})
	}
}

func runGoldenTest(t *testing.T, dir string) {
	argsFile := filepath.Join(dir, "args.txt")
	argsData, err := os.ReadFile(argsFile)
	require.NoError(t, err)

	var args []string
	require.NoError(t, json.Unmarshal(argsData, &args), "args.txt in %s must be a JSON array", dir)

	mfs := &config.MockFileSystem{
		WD:       "/project",
		HomeDir:  "/home/user",
		ExecPath: "/usr/local/bin/cderun",
		Env:      make(map[string]string),
		Files:    make(map[string][]byte),
		Dirs:     map[string]bool{"/project": true, "/home/user": true},
	}

	// Load env if exists
	envFile := filepath.Join(dir, "env.txt")
	if data, err := os.ReadFile(envFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				mfs.Env[parts[0]] = parts[1]
			}
		}
	}

	// Load files if exists (mapped to mfs.WD)
	fsDir := filepath.Join(dir, "fs")
	if info, err := os.Stat(fsDir); err == nil && info.IsDir() {
		err := filepath.Walk(fsDir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(fsDir, p)
			// Map fixture files to mfs.WD (/project/)
			mockPath := path.Join(mfs.WD, filepath.ToSlash(rel))

			if info.IsDir() {
				if mockPath != mfs.WD {
					mfs.Dirs[mockPath] = true
				}
				return nil
			}
			content, _ := os.ReadFile(p)
			mfs.Files[mockPath] = content

			// Ensure parent dirs exist in mfs.Dirs
			curr := mockPath
			for {
				curr = path.Dir(curr)
				if curr == "/" || curr == "." {
					break
				}
				mfs.Dirs[curr] = true
			}
			return nil
		})
		require.NoError(t, err)
	}

	// Ensure binary name is present for ExecuteContextWithOptions if needed.
	// If first arg is not cderun, node, or a path, prepend cderun.
	if len(args) > 0 {
		first := args[0]
		if !strings.HasPrefix(first, "-") && !strings.Contains(first, "/") && first != "cderun" && first != "node" {
			args = append([]string{"cderun"}, args...)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	ctx := context.Background()
	err = ExecuteContextWithOptions(ctx, args, func(o *rootOptions, cmd *cobra.Command) {
		o.runtimeFactory = func(name, socket string, l *logging.Logger) (runtime.ContainerRuntime, error) {
			return &runtime.MockRuntime{}, nil
		}
		o.exitFunc = func(code int) {}
		o.fs = mfs
		o.configLoader = config.NewConfigLoaderWithFS(mfs)
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
	})

	output := stdout.String()
	if err != nil {
		output = fmt.Sprintf("ERROR: %v\nSTDOUT: %s\nSTDERR: %s", err, output, stderr.String())
	}

	if output == "" && stderr.Len() > 0 {
		output = "STDERR ONLY:\n" + stderr.String()
	}

	if output == "" {
		t.Fatalf("Dry-run produced no output in %s. Args: %v, Error: %v", dir, args, err)
	}

	// Normalize output (replace dynamic paths)
	normalized := normalizeGoldenOutput(output)

	goldenFile := filepath.Join(dir, "expected.json")
	if *update {
		err := os.MkdirAll(filepath.Dir(goldenFile), 0755)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(goldenFile, []byte(normalized), 0644))
	}

	expected, err := os.ReadFile(goldenFile)
	require.NoError(t, err)

	require.Equal(t, string(expected), normalized, "Golden mismatch in %s", dir)
}

func normalizeGoldenOutput(s string) string {
	// Simple normalization
	s = strings.ReplaceAll(s, "/home/user", "{{HOME}}")
	s = strings.ReplaceAll(s, "/project", "{{PWD}}")
	s = strings.ReplaceAll(s, "/usr/local/bin/cderun", "{{BIN}}")

	// If it's JSON, pretty-print it
	var j interface{}
	if err := json.Unmarshal([]byte(s), &j); err == nil {
		if b, err := json.MarshalIndent(j, "", "  "); err == nil {
			return string(b) + "\n"
		}
	}
	return s
}
