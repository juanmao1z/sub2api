package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/poolmaintainer"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return flag.ErrHelp
	}

	switch args[0] {
	case "--help", "-h", "help":
		printUsage()
		return flag.ErrHelp
	case "collect":
		return runCollect(args[1:])
	case "apply":
		return runApply(args[1:])
	case "open-browser":
		return runOpenBrowser(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usageText())
	}
}

func runCollect(args []string) error {
	fs := flag.NewFlagSet("collect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "Path to pool-maintainer.yaml")
	htmlDir := fs.String("html-dir", "", "Directory containing <upstream_id>.html snapshots")
	outDir := fs.String("out", "", "Output run directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" || *htmlDir == "" || *outDir == "" {
		return errors.New("collect requires --config, --html-dir, and --out")
	}

	cfg, err := poolmaintainer.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	token, err := adminToken(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := poolmaintainer.NewAdminClient(cfg.LocalSub2API.BaseURL, token, http.DefaultClient)
	accounts, err := client.ListAccounts(ctx)
	if err != nil {
		return fmt.Errorf("list local accounts through Admin API: %w", err)
	}
	now := time.Now().UTC()
	collections := poolmaintainer.ReadCollectionSnapshots(cfg, *htmlDir, now)
	plan := poolmaintainer.BuildPlan(cfg, accounts, collections, now)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	planPath := filepath.Join(*outDir, "apply-plan.json")
	reportPath := filepath.Join(*outDir, "report.html")
	if err := poolmaintainer.WritePlanJSON(plan, planPath); err != nil {
		return fmt.Errorf("write apply plan: %w", err)
	}
	if err := poolmaintainer.WriteHTMLReport(plan, reportPath); err != nil {
		return fmt.Errorf("write html report: %w", err)
	}

	fmt.Printf("wrote %s\n", planPath)
	fmt.Printf("wrote %s\n", reportPath)
	return nil
}

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "Path to pool-maintainer.yaml")
	planPath := fs.String("plan", "", "Path to apply-plan.json")
	dryRun := fs.Bool("dry-run", false, "Check drift and planned actions without mutating accounts")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" || *planPath == "" {
		return errors.New("apply requires --config and --plan")
	}

	cfg, err := poolmaintainer.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	token, err := adminToken(cfg)
	if err != nil {
		return err
	}
	plan, err := poolmaintainer.ReadPlanJSON(*planPath)
	if err != nil {
		return fmt.Errorf("read apply plan: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := poolmaintainer.NewAdminClient(cfg.LocalSub2API.BaseURL, token, http.DefaultClient)
	result, err := client.ApplyPlan(ctx, plan, *dryRun)
	if err != nil {
		return fmt.Errorf("apply plan: %w", err)
	}

	resultPath := filepath.Join(filepath.Dir(*planPath), "apply-result.json")
	if err := writeApplyResult(result, resultPath); err != nil {
		return fmt.Errorf("write apply result: %w", err)
	}
	fmt.Printf("wrote %s\n", resultPath)
	return nil
}

func runOpenBrowser(args []string) error {
	fs := flag.NewFlagSet("open-browser", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "Path to pool-maintainer.yaml")
	profilesDir := fs.String("profiles-dir", "runs/pool-maintainer-profiles", "Directory for reusable browser profile notes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return errors.New("open-browser requires --config")
	}

	cfg, err := poolmaintainer.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	for _, upstream := range cfg.Upstreams {
		if err := poolmaintainer.OpenBrowserProfile(*profilesDir, upstream); err != nil {
			return err
		}
		fmt.Printf("opened %s\n", upstream.ID)
	}
	return nil
}

func adminToken(cfg *poolmaintainer.Config) (string, error) {
	envName := cfg.LocalSub2API.AdminTokenEnv
	token := os.Getenv(envName)
	if token == "" {
		return "", fmt.Errorf("admin token environment variable %s is not set", envName)
	}
	return token, nil
}

func writeApplyResult(result *poolmaintainer.ApplyResult, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func printUsage() {
	fmt.Fprint(os.Stdout, usageText())
}

func usageText() string {
	return `Usage:
  pool-maintainer collect --config <yaml> --html-dir <dir> --out <run-dir>
  pool-maintainer apply --config <yaml> --plan <apply-plan.json> [--dry-run]
  pool-maintainer open-browser --config <yaml> --profiles-dir <dir>

Commands:
  collect       Read upstream snapshots and local Admin API account state, then write report.html and apply-plan.json.
  apply         Apply an approved JSON plan through Admin API after drift checks.
  open-browser Open upstream pricing pages for manual login and snapshot capture.
`
}
