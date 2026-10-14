package main

import (
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeBatch(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rows.jsonl")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func decodeBatch(t *testing.T, output string) batchResult {
	t.Helper()
	var result batchResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("batch output is not JSON: %v\n%s", err, output)
	}
	return result
}

func TestPreviewBatchWritesOnePNGPerRow(t *testing.T) {
	path := testLabelConfig(t)
	rows := writeBatch(t, `{"url":"https://homebox.example/item/1"}

{"url":42}
{"url":"https://homebox.example/item/3"}
`)
	outDir := filepath.Join(t.TempDir(), "previews")
	output, err := captureCLI(t, "preview", "--config", path, "--design", "box", "--batch", rows, "--out", outDir)
	if err != nil {
		t.Fatalf("preview batch: %v\n%s", err, output)
	}
	result := decodeBatch(t, output)
	if !result.OK || len(result.Rows) != 3 {
		t.Fatalf("result = %+v", result)
	}
	for _, row := range result.Rows {
		f, err := os.Open(row.Path)
		if err != nil {
			t.Fatalf("row %d: %v", row.Row, err)
		}
		_, err = png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatalf("row %d: invalid PNG: %v", row.Row, err)
		}
	}
}

func TestPrintBatchReportsEveryInvalidRowBeforeConnecting(t *testing.T) {
	path := testLabelConfig(t)
	rows := writeBatch(t, `{"url":"ok"}
{"title":"no url"}
{"url":"ok"}
{"url":""}
`)
	output, err := captureCLI(t, "print", "--config", path, "--design", "box", "--batch", rows)
	var exit *commandExit
	if !errors.As(err, &exit) || exit.code != 1 {
		t.Fatalf("exit = %v\n%s", err, output)
	}
	result := decodeBatch(t, output)
	if result.OK || result.Printed != 0 || len(result.Rows) != 2 || result.Rows[0].Row != 2 || result.Rows[1].Row != 4 {
		t.Fatalf("result = %+v", result)
	}
}

func TestBatchRejectsMalformedInput(t *testing.T) {
	path := testLabelConfig(t)
	for _, tc := range []struct {
		name string
		args []string
		rows string
	}{
		{"not an object", []string{"--design", "box"}, `["url"]`},
		{"nested value", []string{"--design", "box"}, `{"url":{"a":1}}`},
		{"empty", []string{"--design", "box"}, "\n\n"},
		{"needs design", []string{"--qr", "x"}, `{"url":"x"}`},
		{"no --set", []string{"--design", "box", "--set", "url=x"}, `{"url":"x"}`},
	} {
		args := append([]string{"preview", "--config", path, "--out", t.TempDir(), "--batch", writeBatch(t, tc.rows)}, tc.args...)
		output, err := captureCLI(t, args...)
		var exit *commandExit
		if !errors.As(err, &exit) || exit.code == 0 {
			t.Fatalf("%s: error = %v\n%s", tc.name, err, output)
		}
	}
}
