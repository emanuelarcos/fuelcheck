package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/emarc09/fuelcheck/internal/i18n"
	"github.com/emarc09/fuelcheck/internal/providers"
	"github.com/emarc09/fuelcheck/internal/ui"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// All available providers.
var allProviders = map[string]func() *providers.ProviderResult{
	"claude":      providers.FetchClaudeUsage,
	"codex":       providers.FetchCodexUsage,
	"gemini":      providers.FetchGeminiUsage,
	"antigravity": providers.FetchAntigravityUsage,
	"windsurf":    providers.FetchWindsurfUsage,
}

var providerOrder = []string{"claude", "codex", "gemini", "antigravity", "windsurf"}

var (
	jsonMode bool
	langFlag string
)

func main() {
	// Detect language early so cobra help text is localized.
	i18n.Detect()

	rootCmd := &cobra.Command{
		Use:   "fuelcheck [providers...]",
		Short: i18n.T("cli.short"),
		Long: ui.Banner() + "\n\n" +
			i18n.T("cli.long_desc") + "\n\n" +
			i18n.T("cli.providers") + "\n\n" +
			"Examples:\n" +
			"  fuelcheck                  # All providers\n" +
			"  fuelcheck claude           # Claude only\n" +
			"  fuelcheck claude codex     # Claude and Codex\n" +
			"  fuelcheck --json           # All providers, JSON output\n" +
			"  fuelcheck gemini --json    # Gemini only, JSON output",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: providerCompletion,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if langFlag != "" {
				i18n.Set(langFlag)
			}
		},
		RunE:         run,
		SilenceUsage: true,
		Version:      Version,
	}

	rootCmd.Flags().BoolVar(&jsonMode, "json", false, i18n.T("cli.json_flag"))
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", i18n.T("cli.lang_flag"))

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func providerCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var suggestions []string
	for _, p := range providerOrder {
		if !contains(args, p) {
			suggestions = append(suggestions, p)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func run(cmd *cobra.Command, args []string) error {
	selected := providerOrder
	if len(args) > 0 {
		selected = nil
		for _, arg := range args {
			name := strings.ToLower(strings.TrimSpace(arg))
			if _, ok := allProviders[name]; !ok {
				return fmt.Errorf(i18n.T("cli.unknown_provider"),
					arg, strings.Join(providerOrder, ", "))
			}
			selected = append(selected, name)
		}
	}

	results := fetchSelected(selected)

	if jsonMode {
		printJSON(results)
	} else {
		fmt.Println(ui.Render(results))
	}

	for _, r := range results {
		if r.OK {
			return nil
		}
	}
	return fmt.Errorf("%s", i18n.T("cli.all_failed"))
}

func fetchSelected(names []string) []*providers.ProviderResult {
	type indexedResult struct {
		index  int
		result *providers.ProviderResult
	}

	var wg sync.WaitGroup
	ch := make(chan indexedResult, len(names))

	for i, name := range names {
		fn := allProviders[name]
		wg.Add(1)
		go func(idx int, fetch func() *providers.ProviderResult) {
			defer wg.Done()
			ch <- indexedResult{index: idx, result: fetch()}
		}(i, fn)
	}

	wg.Wait()
	close(ch)

	results := make([]*providers.ProviderResult, len(names))
	for ir := range ch {
		results[ir.index] = ir.result
	}

	return results
}

func printJSON(results []*providers.ProviderResult) {
	output := make(map[string]interface{})
	for _, r := range results {
		if !r.OK {
			output[r.Provider] = map[string]interface{}{"error": r.Error}
		} else {
			output[r.Provider] = r.RawJSON
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output)
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
