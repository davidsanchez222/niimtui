package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/davidsanchez222/niimtui/internal/api"
	"github.com/davidsanchez222/niimtui/internal/config"
	"github.com/davidsanchez222/niimtui/internal/service"
)

const maxBatchRows = 500

type batchResult struct {
	OK      bool       `json:"ok"`
	Printed int        `json:"printed,omitempty"`
	Rows    []batchRow `json:"rows"`
}

// batchRow stays terse on purpose: agents read one per label.
type batchRow struct {
	Row   int            `json:"row"`
	OK    bool           `json:"ok"`
	Path  string         `json:"path,omitempty"`
	Error *api.ErrorBody `json:"error,omitempty"`
}

func readBatchFile(path string) ([]map[string]string, error) {
	var r io.Reader = os.Stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open batch: %w", err)
		}
		defer f.Close()
		r = f
	}
	return parseBatchRows(r)
}

// parseBatchRows reads JSONL: one object of binding values per non-blank line.
// Numbers are accepted so item IDs don't need quoting.
func parseBatchRows(r io.Reader) ([]map[string]string, error) {
	var rows []map[string]string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		text := bytes.TrimSpace(scanner.Bytes())
		if len(text) == 0 {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(text))
		dec.UseNumber()
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("batch line %d: expected a JSON object: %v", line, err)
		}
		row := make(map[string]string, len(raw))
		for name, value := range raw {
			if !config.ValidBindingName(name) {
				return nil, fmt.Errorf("batch line %d: invalid binding name %q", line, name)
			}
			switch v := value.(type) {
			case string:
				row[name] = v
			case json.Number:
				row[name] = v.String()
			default:
				return nil, fmt.Errorf("batch line %d: %q must be a string or number", line, name)
			}
		}
		rows = append(rows, row)
		if len(rows) > maxBatchRows {
			return nil, fmt.Errorf("batch has more than %d rows; split it", maxBatchRows)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read batch: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("batch is empty")
	}
	return rows, nil
}

func runBatch(action string, cfg config.Config, spec labelSpec, rows []map[string]string, outDir string) error {
	svc, err := service.New(cfg)
	if err != nil {
		return err
	}
	defer svc.Close()
	var result batchResult
	if action == "print" {
		result = printBatch(context.Background(), svc, cfg, spec, rows)
	} else {
		result = previewBatch(context.Background(), svc, cfg, spec, rows, outDir)
	}
	if err := printJSON(result); err != nil {
		return err
	}
	if !result.OK {
		return &commandExit{code: 1}
	}
	return nil
}

// prepareBatch validates every row before anything prints, so a typo in row 40
// doesn't leave 39 labels printed. It reports every bad row, not just the first.
func prepareBatch(cfg config.Config, spec labelSpec, rows []map[string]string, forPrint bool) ([]labelJob, []batchRow, bool) {
	jobs := make([]labelJob, len(rows))
	results := make([]batchRow, len(rows))
	ok := true
	for i, values := range rows {
		rowSpec := spec
		rowSpec.Values = values
		job, err := prepareLabel(cfg, rowSpec, forPrint)
		results[i] = batchRow{Row: i + 1, OK: err == nil}
		if err != nil {
			results[i].Error = errorBody(err)
			ok = false
			continue
		}
		jobs[i] = job
	}
	return jobs, results, ok
}

func printBatch(ctx context.Context, svc *service.Service, cfg config.Config, spec labelSpec, rows []map[string]string) batchResult {
	jobs, results, ok := prepareBatch(cfg, spec, rows, true)
	if !ok {
		return batchResult{Rows: invalidRows(results)}
	}
	printed := 0
	for i, job := range jobs {
		resp := job.print(ctx, svc)
		results[i].OK = resp.OK
		results[i].Error = resp.Error
		if !resp.OK {
			// The printer is likely offline or out of labels; later rows would fail too.
			return batchResult{Printed: printed, Rows: results[:i+1]}
		}
		printed++
	}
	return batchResult{OK: true, Printed: printed, Rows: results}
}

func previewBatch(ctx context.Context, svc *service.Service, cfg config.Config, spec labelSpec, rows []map[string]string, outDir string) batchResult {
	jobs, results, ok := prepareBatch(cfg, spec, rows, false)
	if !ok {
		return batchResult{Rows: invalidRows(results)}
	}
	for i, job := range jobs {
		png, err := job.previewPNG(ctx, svc)
		if err == nil {
			results[i].Path = filepath.Join(outDir, fmt.Sprintf("%s-%03d.png", spec.Design, i+1))
			err = writePNG(results[i].Path, png)
		}
		if err != nil {
			results[i].OK, results[i].Path, results[i].Error = false, "", errorBody(err)
			return batchResult{Rows: results[:i+1]}
		}
	}
	return batchResult{OK: true, Rows: results}
}

func invalidRows(results []batchRow) []batchRow {
	invalid := []batchRow{}
	for _, row := range results {
		if !row.OK {
			invalid = append(invalid, row)
		}
	}
	return invalid
}
