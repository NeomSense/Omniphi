// Package keeper — bridge ingestion and ACK emission for the cross-lane loop.
//
// BridgeIngestionService scans a directory for PoSeq-exported ExportBatch JSON
// files, ingests them via IngestExportBatchWithAck, and writes ACK JSON files
// to a return directory that PoSeq can poll.
//
// Directory layout:
//   export_dir/<epoch>.json   — written by PoSeq, consumed by this service
//   ack_dir/<epoch>.ack.json  — written by this service, consumed by PoSeq
package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"pos/x/poseq/types"
)

// BridgeIngestionResult summarizes a single ingestion cycle.
type BridgeIngestionResult struct {
	Accepted   int
	Duplicated int
	Rejected   int
	Errors     []string
}

// IngestFromDirectory scans exportDir for <epoch>.json files, ingests each
// through IngestExportBatchWithAck, and writes the ACK to ackDir.
//
// Already-ingested epochs (dedup) are handled gracefully: the ACK is still
// written (idempotent) but no double-processing occurs.
//
// The sender parameter is checked against AuthorizedSubmitter in params.
// The blockHeight is embedded in the ACK for operator reference.
func (k Keeper) IngestFromDirectory(ctx context.Context, exportDir, ackDir, sender string, blockHeight int64) BridgeIngestionResult {
	result := BridgeIngestionResult{}

	// Ensure ack dir exists
	if err := os.MkdirAll(ackDir, 0o755); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("create ack_dir: %v", err))
		return result
	}

	// Scan for export files
	entries, err := os.ReadDir(exportDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("read export_dir: %v", err))
		return result
	}

	// Collect epoch files, sort by epoch for deterministic processing
	type epochFile struct {
		epoch    uint64
		filename string
	}
	var files []epochFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		epochStr := strings.TrimSuffix(name, ".json")
		epoch, err := strconv.ParseUint(epochStr, 10, 64)
		if err != nil {
			continue // not an epoch file
		}
		files = append(files, epochFile{epoch: epoch, filename: name})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].epoch < files[j].epoch })

	for _, ef := range files {
		// Check if ACK already written (avoid re-reading and re-parsing)
		ackPath := filepath.Join(ackDir, fmt.Sprintf("%d.ack.json", ef.epoch))
		if _, err := os.Stat(ackPath); err == nil {
			// ACK already exists — skip
			continue
		}

		// Read export file
		exportPath := filepath.Join(exportDir, ef.filename)
		data, err := os.ReadFile(exportPath)
		if err != nil {
			result.Errors = append(result.Errors,
				fmt.Sprintf("epoch %d: read error: %v", ef.epoch, err))
			result.Rejected++
			continue
		}

		// Deserialize
		var batch types.ExportBatch
		if err := json.Unmarshal(data, &batch); err != nil {
			k.logger.Error("bridge.ingest.malformed",
				"epoch", ef.epoch,
				"error", err,
			)
			// Write a rejected ACK so we don't retry this bad file
			ack := BuildExportBatchAck(ef.epoch, types.AckStatusRejected,
				fmt.Sprintf("JSON parse error: %v", err), blockHeight)
			writeAckFile(ackPath, ack)
			result.Rejected++
			continue
		}

		// Validate epoch consistency
		if batch.Epoch != ef.epoch {
			reason := fmt.Sprintf("filename epoch %d != batch.epoch %d", ef.epoch, batch.Epoch)
			k.logger.Error("bridge.ingest.epoch_mismatch",
				"file_epoch", ef.epoch,
				"batch_epoch", batch.Epoch,
			)
			ack := BuildExportBatchAck(ef.epoch, types.AckStatusRejected, reason, blockHeight)
			writeAckFile(ackPath, ack)
			result.Rejected++
			continue
		}

		// Ingest with dedup
		ack, err := k.IngestExportBatchWithAck(ctx, sender, batch, blockHeight)
		if err != nil {
			result.Rejected++
		} else {
			switch ack.Status {
			case types.AckStatusAccepted:
				result.Accepted++
				k.logger.Info("bridge.ingest.accepted", "epoch", ef.epoch)
			case types.AckStatusDuplicate:
				result.Duplicated++
				k.logger.Info("bridge.ingest.duplicate", "epoch", ef.epoch)
			case types.AckStatusRejected:
				result.Rejected++
			}
		}

		// Always write the ACK file (idempotent)
		if writeErr := writeAckFile(ackPath, ack); writeErr != nil {
			result.Errors = append(result.Errors,
				fmt.Sprintf("epoch %d: write ack: %v", ef.epoch, writeErr))
		} else {
			k.logger.Info("bridge.ack.emitted",
				"epoch", ef.epoch,
				"status", ack.Status,
				"ack_path", ackPath,
			)
		}
	}

	return result
}

// writeAckFile atomically writes an ACK JSON file. Uses write-then-rename
// to prevent PoSeq from reading a partial file.
func writeAckFile(path string, ack types.ExportBatchAck) error {
	data, err := json.MarshalIndent(ack, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ack: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write tmp ack: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		// Fallback: on Windows, rename may fail if target exists. Try direct write.
		return os.WriteFile(path, data, 0o644)
	}
	return nil
}
