package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

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
}

// Ordered list to preserve display order.
var providerOrder = []string{"claude", "codex", "gemini", "antigravity"}

var jsonMode bool

func main() {
	rootCmd := &cobra.Command{
		Use:   "fuelcheck [proveedores...]",
		Short: "Consulta el estado de uso de tus suscripciones de IA",
		Long: ui.Banner() + `

Consulta en paralelo el estado de uso de tus suscripciones
de Claude, Codex, Gemini y Antigravity.

Proveedores disponibles: claude, codex, gemini, antigravity

Ejemplos:
  fuelcheck                  # Todos los proveedores
  fuelcheck claude           # Solo Claude
  fuelcheck claude codex     # Claude y Codex
  fuelcheck --json           # Todos en formato JSON
  fuelcheck gemini --json    # Solo Gemini en JSON`,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: providerCompletion,
		RunE:              run,
		SilenceUsage:      true,
		Version:           Version,
	}

	rootCmd.Flags().BoolVar(&jsonMode, "json", false, "Salida en formato JSON")

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// providerCompletion provides shell completion for provider names.
func providerCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var suggestions []string
	for _, p := range providerOrder {
		if !contains(args, p) {
			suggestions = append(suggestions, p)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// run is the main command handler.
func run(cmd *cobra.Command, args []string) error {
	selected := providerOrder
	if len(args) > 0 {
		selected = nil
		for _, arg := range args {
			name := strings.ToLower(strings.TrimSpace(arg))
			if _, ok := allProviders[name]; !ok {
				return fmt.Errorf("proveedor desconocido: %q\nDisponibles: %s",
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

	// Return error if all requested providers failed.
	for _, r := range results {
		if r.OK {
			return nil
		}
	}
	return fmt.Errorf("todos los proveedores fallaron")
}

// fetchSelected runs the given provider fetches concurrently.
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

// printJSON outputs the combined results as JSON.
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
