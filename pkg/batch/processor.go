package batch

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"encoding/json"
	"github.com/go-go-golems/vault-envrc-generator/pkg/envrc"
	cmdout "github.com/go-go-golems/vault-envrc-generator/pkg/output"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Processor struct {
	Client *vault.Client
}

type ProcessorOptions struct {
	BasePath               string
	OutputOverride         string
	FormatOverride         string
	ContinueOnError        bool
	DryRun                 bool
	SortKeys               bool
	ForceOverwrite         bool
	SkipUnreadableSections bool
	AllowCommands          bool
	Verbose                bool
}

func (p *Processor) Process(ctx context.Context, cfg *Config, opts ProcessorOptions) error {
	// build template context
	tctx, err := vault.BuildTemplateContext(p.Client)
	if err != nil {
		return fmt.Errorf("failed to build template context: %w", err)
	}

	// determine base path (opts overrides YAML)
	basePath := strings.TrimSuffix(cfg.BasePath, "/")
	if opts.BasePath != "" {
		basePath = strings.TrimSuffix(opts.BasePath, "/")
	}
	if basePath != "" {
		if bp, err := vault.RenderTemplateString(basePath, tctx); err == nil {
			basePath = strings.TrimSuffix(bp, "/")
		}
	}

	return p.processSequential(ctx, cfg.Jobs, tctx, basePath, opts)
}

func (p *Processor) processSequential(ctx context.Context, jobs []Job, tctx vault.TemplateContext, basePath string, opts ProcessorOptions) error {
	var errors []error
	for i, job := range jobs {
		fmt.Printf("[%d/%d] Processing job: %s\n", i+1, len(jobs), job.Name)
		log.Debug().Int("sections", len(job.Sections)).Str("job", job.Name).Msg("batch job start")
		if err := p.processJob(ctx, job, tctx, basePath, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Job '%s' failed: %v\n", job.Name, err)
			errors = append(errors, err)
			if !opts.ContinueOnError {
				return fmt.Errorf("job '%s' failed: %w", job.Name, err)
			}
		} else {
			fmt.Printf("✓ Job '%s' completed successfully\n", job.Name)
		}
	}
	if len(errors) > 0 {
		fmt.Printf("\nCompleted with %d errors out of %d jobs\n", len(errors), len(jobs))
		return fmt.Errorf("batch processing completed with %d errors", len(errors))
	}
	fmt.Printf("\n✓ All %d jobs completed successfully\n", len(jobs))
	return nil
}

// processParallel was removed to avoid lock contention; keep sequential processing only.

func (p *Processor) processJob(ctx context.Context, job Job, tctx vault.TemplateContext, basePath string, opts ProcessorOptions) error {
	log.Debug().Str("job", job.Name).Int("sections", len(job.Sections)).Msg("process job")
	// job-level base path override
	effectiveBase := basePath
	if strings.TrimSpace(job.BasePath) != "" {
		effectiveBase = strings.TrimSuffix(job.BasePath, "/")
		if rbp, err := vault.RenderTemplateString(effectiveBase, tctx); err == nil {
			effectiveBase = strings.TrimSuffix(rbp, "/")
		} else {
			return fmt.Errorf("failed to render job base_path '%s': %w", job.BasePath, err)
		}
	}
	log.Debug().Str("job", job.Name).Str("effectiveBase", effectiveBase).Msg("job base path")

	if len(job.Sections) > 0 {
		// stdout aggregations (only used when output == "-")
		var stdoutJSONAgg map[string]interface{}
		var stdoutYAMLAgg map[string]interface{}
		var stdoutENVRCAgg strings.Builder
		// aggregate envrc content per output path so we can overwrite once at the end
		envrcFileBuffers := map[string]*strings.Builder{}

		// Track per-job decisions for command prompts
		commandAllRun := false
		commandAllSkip := false
		for _, sec := range job.Sections {
			select {
			case <-ctx.Done():
				return fmt.Errorf("interrupted")
			default:
			}
			log.Debug().Str("section", sec.Name).Msg("section start")
			fmt.Print(cmdout.SectionHeader(sec.Name, sec.Description))
			joinedPath := vault.JoinBaseAndPath(effectiveBase, sec.Path)
			renderedSourcePath, err := vault.RenderTemplateString(joinedPath, tctx)
			if err != nil {
				return fmt.Errorf("failed to render section path '%s': %w", sec.Path, err)
			}
			outPath := job.Output
			if sec.Output != "" {
				outPath = sec.Output
			}
			if opts.OutputOverride != "" {
				outPath = opts.OutputOverride
			}
			renderedOutPath, err := vault.RenderTemplateString(outPath, tctx)
			if err != nil {
				return fmt.Errorf("failed to render section output '%s': %w", outPath, err)
			}
			if opts.DryRun {
				renderedOutPath = "-"
			}
			format := job.Format
			if sec.Format != "" {
				format = sec.Format
			}
			if opts.FormatOverride != "" {
				format = opts.FormatOverride
			}
			if format == "" {
				format = "envrc"
			}

			log.Debug().Str("section", sec.Name).Str("source", renderedSourcePath).Str("output", renderedOutPath).Str("format", format).Msg("section io")

			// secrets
			secrets := map[string]interface{}{}
			if strings.TrimSpace(renderedSourcePath) != "" {
				s, err := p.Client.GetSecrets(renderedSourcePath)
				if err != nil {
					if opts.SkipUnreadableSections && !job.Required && !sec.Required {
						// keep going with fallbacks (fixed/commands)
						var descParts []string
						if strings.TrimSpace(job.Description) != "" {
							descParts = append(descParts, job.Description)
						}
						if strings.TrimSpace(sec.Description) != "" {
							descParts = append(descParts, sec.Description)
						}
						desc := strings.Join(descParts, " / ")
						short := cmdout.ShortError(err)
						if desc != "" {
							fmt.Println(cmdout.Warnf("unreadable section '%s' (%s) for job '%s' — %s; proceeding with fallbacks: %s", sec.Name, renderedSourcePath, job.Name, desc, short))
						} else {
							fmt.Println(cmdout.Warnf("unreadable section '%s' (%s) for job '%s'; proceeding with fallbacks: %s", sec.Name, renderedSourcePath, job.Name, short))
						}
					} else {
						var descParts []string
						if strings.TrimSpace(job.Description) != "" {
							descParts = append(descParts, job.Description)
						}
						if strings.TrimSpace(sec.Description) != "" {
							descParts = append(descParts, sec.Description)
						}
						desc := strings.Join(descParts, " / ")
						if desc != "" {
							return fmt.Errorf("failed to retrieve secrets from path %s for job '%s' — %s: %w", renderedSourcePath, job.Name, desc, err)
						}
						return fmt.Errorf("failed to retrieve secrets from path %s: %w", renderedSourcePath, err)
					}
				} else {
					for k, v := range s {
						secrets[k] = v
					}
					log.Debug().Int("keys", len(s)).Str("source", renderedSourcePath).Msg("fetched secrets")
					fmt.Println(cmdout.Notef("Fetched %d key(s) from %s", len(s), renderedSourcePath))
				}
			}

			// fixed values
			if len(job.Fixed) > 0 {
				for k, tv := range job.Fixed {
					rv, err := vault.RenderTemplateString(tv, tctx)
					if err != nil {
						return fmt.Errorf("failed to render job fixed '%s': %w", k, err)
					}
					secrets[k] = rv
				}
			}
			if len(sec.Fixed) > 0 {
				for k, tv := range sec.Fixed {
					rv, err := vault.RenderTemplateString(tv, tctx)
					if err != nil {
						return fmt.Errorf("failed to render section fixed '%s': %w", k, err)
					}
					secrets[k] = rv
				}
			}

			// variables
			if len(job.Variables) > 0 {
				for key, value := range job.Variables {
					secrets[key] = value
				}
			}
			if len(sec.Variables) > 0 {
				for key, value := range sec.Variables {
					secrets[key] = value
				}
			}

			// Populate template data for command rendering
			if tctx.Data == nil {
				tctx.Data = map[string]interface{}{}
			}
			for dk, dv := range secrets {
				tctx.Data[dk] = dv
			}

			// Execute fallback commands if provided
			commandValues := map[string]string{}
			if len(sec.Commands) > 0 {
				keys := make([]string, 0, len(sec.Commands))
				for k := range sec.Commands {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					cmdT := sec.Commands[k]
					cmdStr, err := vault.RenderTemplateString(cmdT, tctx)
					if err != nil {
						return fmt.Errorf("section '%s': failed to render command for key '%s': %w", sec.Name, k, err)
					}
					if strings.TrimSpace(cmdStr) == "" {
						continue
					}
					if opts.DryRun {
						commandValues[k] = fmt.Sprintf("<command: %s>", cmdStr)
						tctx.Data[k] = commandValues[k]
						continue
					}
					if !opts.AllowCommands {
						if commandAllSkip {
							continue
						}
						if !commandAllRun {
							prompt := fmt.Sprintf("Run command for key '%s' in job '%s' section '%s'?: %s [y/N/a/s] ", k, job.Name, sec.Name, cmdStr)
							dec := askForDecision(prompt)
							switch dec {
							case decYes:
								// proceed
							case decAllYes:
								commandAllRun = true
							case decAllNo:
								commandAllSkip = true
								continue
							case decNo:
								continue
							}
						}
					}
					out, err := runShellCommand(cmdStr)
					if err != nil {
						return fmt.Errorf("section '%s': command for key '%s' failed: %w", sec.Name, k, err)
					}
					val := strings.TrimSpace(out)
					if val == "" {
						continue
					}
					commandValues[k] = val
					tctx.Data[k] = val
				}
			}

			// options
			prefix := job.Prefix
			if sec.Prefix != "" {
				prefix = sec.Prefix
			}
			exclude := job.ExcludeKeys
			if len(sec.ExcludeKeys) > 0 {
				exclude = sec.ExcludeKeys
			}
			include := job.IncludeKeys
			if len(sec.IncludeKeys) > 0 {
				include = sec.IncludeKeys
			}
			var transform bool
			if sec.Transform != nil {
				transform = *sec.Transform
			} else if job.Transform != nil {
				transform = *job.Transform
			} else {
				transform = false
			}
			templateFile := job.Template
			if sec.Template != "" {
				templateFile = sec.Template
			}

			// env_map explicit mapping
			selected := secrets
			if len(sec.EnvMap) > 0 {
				mapped := make(map[string]interface{}, len(sec.EnvMap))
				for envName, srcKey := range sec.EnvMap {
					if v, ok := secrets[srcKey]; ok {
						mapped[envName] = v
					} else {
						log.Debug().Str("source", renderedSourcePath).Str("key", srcKey).Msg("missing key in env_map")
					}
				}
				// Merge command-produced values as fallback (do not overwrite mapped keys)
				for envName, v := range commandValues {
					if _, ok := mapped[envName]; !ok {
						mapped[envName] = v
					}
				}
				selected = mapped
				transform = false
				prefix = ""
				exclude = nil
				include = nil
			} else {
				// No env_map: merge command values into secrets as source keys if missing
				for k, v := range commandValues {
					if _, ok := secrets[k]; !ok {
						secrets[k] = v
					}
				}
			}

			suppressHeader := false
			if format == "envrc" {
				// With aggregation, suppress generic header; per-section header added below
				suppressHeader = true
			}

			options := &envrc.Options{
				Prefix:         prefix,
				ExcludeKeys:    exclude,
				IncludeKeys:    include,
				TransformKeys:  transform,
				Format:         format,
				TemplateFile:   templateFile,
				Verbose:        false,
				SuppressHeader: suppressHeader,
				SortKeys:       opts.SortKeys,
			}

			generator := envrc.NewGenerator(options)
			content, err := generator.Generate(selected)
			if err != nil {
				return fmt.Errorf("failed to generate content: %w", err)
			}
			log.Debug().Int("bytes", len(content)).Str("section", sec.Name).Msg("generated content")
			if opts.Verbose {
				fmt.Println(cmdout.EmittingCount(len(selected)))
				if len(selected) > 0 {
					var names []string
					for k := range selected {
						names = append(names, k)
					}
					sort.Strings(names)
					fmt.Print(cmdout.ListNames(names))
				}
			}

			if options.Format == "envrc" {
				header := fmt.Sprintf("# === %s", job.Name)
				if sec.Name != "" {
					header += fmt.Sprintf(": %s", sec.Name)
				}
				header += " ===\n"
				header += fmt.Sprintf("# Source path: %s\n", renderedSourcePath)
				if job.Description != "" {
					header += fmt.Sprintf("# Job: %s\n", job.Description)
				}
				if sec.Description != "" {
					header += fmt.Sprintf("# Section: %s\n", sec.Description)
				}
				header += "\n"
				content = header + content + "\n"
			}

			// stdout vs file handling: aggregate to stdout, otherwise write immediately (merge-only)
			if renderedOutPath == "-" {
				switch format {
				case "json":
					var next map[string]interface{}
					if err := json.Unmarshal([]byte(content), &next); err != nil {
						return fmt.Errorf("failed to parse generated JSON for aggregation: %w", err)
					}
					if stdoutJSONAgg == nil {
						stdoutJSONAgg = map[string]interface{}{}
					}
					for k, v := range next {
						stdoutJSONAgg[k] = v
					}
				case "yaml":
					var next map[string]interface{}
					if err := yaml.Unmarshal([]byte(content), &next); err != nil {
						return fmt.Errorf("failed to parse generated YAML for aggregation: %w", err)
					}
					if stdoutYAMLAgg == nil {
						stdoutYAMLAgg = map[string]interface{}{}
					}
					for k, v := range next {
						stdoutYAMLAgg[k] = v
					}
				default:
					// For envrc stdout aggregation, we will assemble prefix and suffix once after the loop
					stdoutENVRCAgg.WriteString(content)
				}
			} else {
				if format == "envrc" {
					b, ok := envrcFileBuffers[renderedOutPath]
					if !ok {
						b = &strings.Builder{}
						envrcFileBuffers[renderedOutPath] = b
					}
					b.WriteString(content)
				} else {
					if err := cmdout.Write(renderedOutPath, []byte(content), cmdout.WriteOptions{Format: format, SortKeys: opts.SortKeys}); err != nil {
						return err
					}
				}
			}
		}

		// flush aggregated envrc outputs (ask confirmation before overwrite unless forced)
		for outPath, b := range envrcFileBuffers {
			if b.Len() == 0 {
				continue
			}
			// Prefix/Suffix wrapping for envrc (render as templates)
			finalContent := b.String()
			if strings.TrimSpace(job.EnvrcPrefix) != "" {
				rp, err := vault.RenderTemplateString(job.EnvrcPrefix, tctx)
				if err != nil {
					return fmt.Errorf("failed to render envrc_prefix: %w", err)
				}
				finalContent = rp + "\n" + finalContent
			}
			if strings.TrimSpace(job.EnvrcSuffix) != "" {
				rs, err := vault.RenderTemplateString(job.EnvrcSuffix, tctx)
				if err != nil {
					return fmt.Errorf("failed to render envrc_suffix: %w", err)
				}
				if !strings.HasSuffix(finalContent, "\n") {
					finalContent += "\n"
				}
				finalContent = finalContent + rs + "\n"
			}
			// Ensure directory exists
			if dir := filepath.Dir(outPath); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create output directory %s: %w", dir, err)
				}
			}
			if fi, err := os.Stat(outPath); err == nil && fi.Mode().IsRegular() {
				if !opts.ForceOverwrite {
					ok, err := confirmOverwrite(outPath)
					if err != nil {
						return err
					}
					if !ok {
						log.Info().Str("path", outPath).Msg("skipped overwrite of existing .envrc")
						continue
					}
				}
			}
			if err := os.WriteFile(outPath, []byte(finalContent), 0644); err != nil {
				return fmt.Errorf("failed to write envrc output to %s: %w", outPath, err)
			}
			log.Debug().Str("output", outPath).Int("bytes", len(finalContent)).Msg("envrc file overwritten")
		}

		// flush outputs to stdout
		if stdoutJSONAgg != nil {
			if opts.SortKeys {
				keys := make([]string, 0, len(stdoutJSONAgg))
				for k := range stdoutJSONAgg {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var bld strings.Builder
				bld.WriteString("{\n")
				for i, k := range keys {
					kb, _ := json.Marshal(k)
					vb, err := json.Marshal(stdoutJSONAgg[k])
					if err != nil {
						return fmt.Errorf("failed to marshal aggregated JSON value: %w", err)
					}
					if i > 0 {
						bld.WriteString(",\n")
					}
					bld.WriteString("  ")
					bld.Write(kb)
					bld.WriteString(": ")
					bld.Write(vb)
				}
				bld.WriteString("\n}")
				fmt.Print(bld.String())
			} else {
				b, err := json.MarshalIndent(stdoutJSONAgg, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal aggregated JSON: %w", err)
				}
				fmt.Print(string(b))
			}
		}
		if stdoutYAMLAgg != nil {
			if len(stdoutYAMLAgg) > 0 {
				if opts.SortKeys {
					keys := make([]string, 0, len(stdoutYAMLAgg))
					for k := range stdoutYAMLAgg {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					node := &yaml.Node{Kind: yaml.MappingNode}
					for _, k := range keys {
						kn := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
						var valueDoc yaml.Node
						b, err := yaml.Marshal(stdoutYAMLAgg[k])
						if err != nil {
							return fmt.Errorf("failed to marshal yaml value: %w", err)
						}
						if err := yaml.Unmarshal(b, &valueDoc); err != nil {
							return fmt.Errorf("failed to unmarshal yaml value node: %w", err)
						}
						var vn *yaml.Node
						if len(valueDoc.Content) == 0 {
							vn = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "~"}
						} else {
							vn = valueDoc.Content[0]
						}
						node.Content = append(node.Content, kn, vn)
					}
					b, err := yaml.Marshal(node)
					if err != nil {
						return fmt.Errorf("failed to marshal ordered YAML: %w", err)
					}
					fmt.Print(string(b))
				} else {
					b, err := yaml.Marshal(stdoutYAMLAgg)
					if err != nil {
						return fmt.Errorf("failed to marshal aggregated YAML: %w", err)
					}
					fmt.Print(string(b))
				}
			}
		}
		if stdoutENVRCAgg.Len() > 0 {
			finalStdout := stdoutENVRCAgg.String()
			if strings.TrimSpace(job.EnvrcPrefix) != "" {
				rp, err := vault.RenderTemplateString(job.EnvrcPrefix, tctx)
				if err != nil {
					return fmt.Errorf("failed to render envrc_prefix: %w", err)
				}
				finalStdout = rp + "\n" + finalStdout
			}
			if strings.TrimSpace(job.EnvrcSuffix) != "" {
				rs, err := vault.RenderTemplateString(job.EnvrcSuffix, tctx)
				if err != nil {
					return fmt.Errorf("failed to render envrc_suffix: %w", err)
				}
				if !strings.HasSuffix(finalStdout, "\n") {
					finalStdout += "\n"
				}
				finalStdout = finalStdout + rs + "\n"
			}
			fmt.Print(finalStdout)
		}
		return nil
	}

	// legacy single-path mode
	joinedJobPath := vault.JoinBaseAndPath(effectiveBase, job.Path)
	renderedPath, err := vault.RenderTemplateString(joinedJobPath, tctx)
	if err != nil {
		return fmt.Errorf("failed to render job path '%s': %w", job.Path, err)
	}
	outPath := job.Output
	if opts.OutputOverride != "" {
		outPath = opts.OutputOverride
	}
	renderedOutput, err := vault.RenderTemplateString(outPath, tctx)
	if err != nil {
		return fmt.Errorf("failed to render job output '%s': %w", outPath, err)
	}

	secrets, err := p.Client.GetSecrets(renderedPath)
	if err != nil {
		if opts.SkipUnreadableSections {
			fmt.Fprintf(os.Stderr, "Warning: skipping unreadable job '%s' path %s: %v\n", job.Name, renderedPath, err)
			return nil
		}
		return fmt.Errorf("failed to retrieve secrets from path %s: %w", renderedPath, err)
	}

	if len(job.Fixed) > 0 {
		for k, tv := range job.Fixed {
			rv, err := vault.RenderTemplateString(tv, tctx)
			if err != nil {
				return fmt.Errorf("failed to render job fixed '%s': %w", k, err)
			}
			secrets[k] = rv
		}
	}
	if len(job.Variables) > 0 {
		for k, v := range job.Variables {
			secrets[k] = v
		}
	}

	options := &envrc.Options{
		Prefix:      job.Prefix,
		ExcludeKeys: job.ExcludeKeys,
		IncludeKeys: job.IncludeKeys,
		TransformKeys: func() bool {
			if job.Transform != nil {
				return *job.Transform
			}
			return false
		}(),
		Format:         job.Format,
		TemplateFile:   job.Template,
		Verbose:        false,
		SuppressHeader: false,
		SortKeys:       opts.SortKeys,
	}
	if opts.FormatOverride != "" {
		options.Format = opts.FormatOverride
	}
	if options.Format == "" {
		options.Format = "envrc"
	}

	// For envrc, we now overwrite instead of appending; keep headers

	generator := envrc.NewGenerator(options)
	content, err := generator.Generate(secrets)
	if err != nil {
		return fmt.Errorf("failed to generate content: %w", err)
	}
	if options.Format == "envrc" {
		header := fmt.Sprintf("# === %s ===\n# Source path: %s\n", job.Name, renderedPath)
		if job.Description != "" {
			header += fmt.Sprintf("# Description: %s\n", job.Description)
		}
		header += "\n"
		finalContent := header + content + "\n"
		if strings.TrimSpace(job.EnvrcPrefix) != "" {
			rp, err := vault.RenderTemplateString(job.EnvrcPrefix, tctx)
			if err != nil {
				return fmt.Errorf("failed to render envrc_prefix: %w", err)
			}
			finalContent = rp + "\n" + finalContent
		}
		if strings.TrimSpace(job.EnvrcSuffix) != "" {
			rs, err := vault.RenderTemplateString(job.EnvrcSuffix, tctx)
			if err != nil {
				return fmt.Errorf("failed to render envrc_suffix: %w", err)
			}
			if !strings.HasSuffix(finalContent, "\n") {
				finalContent += "\n"
			}
			finalContent = finalContent + rs + "\n"
		}
		content = finalContent
	}

	log.Debug().Str("output", renderedOutput).Msg("writing job output")
	if opts.DryRun {
		renderedOutput = "-"
	}
	if options.Format == "envrc" && renderedOutput != "-" {
		// Ensure directory exists
		if dir := filepath.Dir(renderedOutput); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory %s: %w", dir, err)
			}
		}
		if fi, err := os.Stat(renderedOutput); err == nil && fi.Mode().IsRegular() {
			if !opts.ForceOverwrite {
				ok, err := confirmOverwrite(renderedOutput)
				if err != nil {
					return err
				}
				if !ok {
					log.Info().Str("path", renderedOutput).Msg("skipped overwrite of existing .envrc")
					return nil
				}
			}
		}
		if err := os.WriteFile(renderedOutput, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write envrc output to %s: %w", renderedOutput, err)
		}
		return nil
	}
	return cmdout.Write(renderedOutput, []byte(content), cmdout.WriteOptions{Format: options.Format, SortKeys: opts.SortKeys})
}

// confirmOverwrite prompts the user to confirm overwriting an existing file.
// Returns true if overwrite is confirmed, false otherwise.
func confirmOverwrite(path string) (bool, error) {
	fmt.Printf("File %s already exists. Overwrite? [y/N]: ", path)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

// decision model copied from seed runner to keep UX consistent
type userDecision int

const (
	decNo userDecision = iota
	decYes
	decAllYes
	decAllNo
)

func askForDecision(prompt string) userDecision {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.TrimSpace(strings.ToLower(line))
	switch answer {
	case "y", "yes":
		return decYes
	case "a", "all":
		return decAllYes
	case "s", "skip", "sa", "skipall", "skip-all":
		return decAllNo
	default:
		return decNo
	}
}

func runShellCommand(command string) (string, error) {
	cmd := exec.Command("/bin/sh", "-c", command)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
