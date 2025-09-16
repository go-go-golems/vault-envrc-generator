package analyze

import (
	"context"

	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
)

// EmitRows streams the analysis result into a Glazed processor.
func EmitRows(ctx context.Context, result *Result, includeValues bool, censor string, processor middlewares.Processor) error {
	if result == nil {
		return nil
	}

	summaryRow := types.NewRow(
		types.MRP("category", "summary"),
		types.MRP("env_source", result.EnvSource),
		types.MRP("seed_config", result.SeedConfig),
		types.MRP("mapped_present", result.Summary.MappedPresent),
		types.MRP("mapped_missing", result.Summary.MappedMissing),
		types.MRP("unmapped_present", result.Summary.UnmappedPresent),
	)
	if err := processor.AddRow(ctx, summaryRow); err != nil {
		return err
	}

	maskValue := func(actual string) string {
		if !includeValues {
			return ""
		}
		if censor == "" {
			return actual
		}
		return censor
	}

	for _, entry := range result.Details.MappedPresent {
		row := types.NewRow(
			types.MRP("category", "mapped_present"),
			types.MRP("env_var", entry.EnvVar),
			types.MRP("value", maskValue(entry.Value)),
			types.MRP("mapped_set", entry.SetName),
			types.MRP("vault_path", entry.VaultPath),
			types.MRP("vault_key", entry.VaultKey),
		)
		if err := processor.AddRow(ctx, row); err != nil {
			return err
		}
	}

	for _, entry := range result.Details.MappedMissing {
		row := types.NewRow(
			types.MRP("category", "mapped_missing"),
			types.MRP("env_var", entry.EnvVar),
			types.MRP("mapped_set", entry.SetName),
			types.MRP("vault_path", entry.VaultPath),
			types.MRP("vault_key", entry.VaultKey),
		)
		if err := processor.AddRow(ctx, row); err != nil {
			return err
		}
	}

	for _, entry := range result.Details.UnmappedPresent {
		row := types.NewRow(
			types.MRP("category", "unmapped_present"),
			types.MRP("env_var", entry.EnvVar),
			types.MRP("value", maskValue(entry.Value)),
		)
		if err := processor.AddRow(ctx, row); err != nil {
			return err
		}
	}

	for _, input := range result.NonEnvSeedInputs {
		row := types.NewRow(
			types.MRP("category", "non_env_seed_input"),
			types.MRP("set", input.SetName),
			types.MRP("type", input.Type),
			types.MRP("keys", input.Keys),
		)
		if err := processor.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}
