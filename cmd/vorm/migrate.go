package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vorzela/vorm/config"
	"github.com/vorzela/vorm/migrate"
	"github.com/vorzela/vorm/query"
)

// migrateFlags are the flags shared by the migration commands.
type migrateFlags struct {
	dsn          string
	path         string
	steps        string
	migration    string
	force        bool
	dryRun       bool
	verbose      bool
	skipLock     bool
	dropDisabled bool
	step         int
	rest         []string
}

func parseMigrateFlags(args []string) (migrateFlags, error) {
	var f migrateFlags
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func(name string) (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", name)
			}
			i++
			return args[i], nil
		}
		var err error
		switch {
		case a == "--dsn", a == "-d":
			f.dsn, err = next(a)
		case strings.HasPrefix(a, "--dsn="):
			f.dsn = strings.TrimPrefix(a, "--dsn=")
		case a == "--path", a == "-p":
			f.path, err = next(a)
		case strings.HasPrefix(a, "--path="):
			f.path = strings.TrimPrefix(a, "--path=")
		case a == "--steps", a == "--step", a == "-n":
			f.steps, err = next(a)
		case strings.HasPrefix(a, "--steps="):
			f.steps = strings.TrimPrefix(a, "--steps=")
		case strings.HasPrefix(a, "--step="):
			f.steps = strings.TrimPrefix(a, "--step=")
		case a == "--migration", a == "-m":
			f.migration, err = next(a)
		case strings.HasPrefix(a, "--migration="):
			f.migration = strings.TrimPrefix(a, "--migration=")
		case a == "--force":
			f.force = true
		case a == "--dry-run":
			f.dryRun = true
		case a == "--verbose", a == "-v":
			f.verbose = true
		case a == "--skip-lock":
			f.skipLock = true
		case a == "--drop-disabled":
			f.dropDisabled = true
		default:
			f.rest = append(f.rest, a)
		}
		if err != nil {
			return f, err
		}
	}
	if f.steps != "" && !strings.EqualFold(f.steps, "all") {
		n, err := strconv.Atoi(f.steps)
		if err != nil {
			return f, fmt.Errorf("--steps wants a number or \"all\", got %q", f.steps)
		}
		f.step = n
	}
	return f, nil
}

// cmdMigrate runs migrations in-process.
func cmdMigrate(cmd string, args []string) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	flags, err := parseMigrateFlags(args)
	if err != nil {
		return err
	}

	// `vorm migrate status` and `vorm status` mean the same thing; accepting
	// both spellings avoids silently running a migration when the user asked
	// for something else.
	if cmd == "migrate" && len(flags.rest) > 0 {
		switch sub := strings.ToLower(flags.rest[0]); sub {
		case "up":
			flags.rest = flags.rest[1:]
		case "down":
			cmd, flags.rest = "rollback", flags.rest[1:]
		case "status", "rollback", "fresh", "refresh":
			cmd, flags.rest = sub, flags.rest[1:]
		}
	}
	if len(flags.rest) > 0 {
		return fmt.Errorf("unexpected argument %q for %s", flags.rest[0], cmd)
	}

	url := flags.dsn
	if url == "" {
		url = cfg.ResolveDatabaseURL()
	}
	if url == "" {
		return fmt.Errorf("no database connection: set DATABASE_URL in the environment or .env, add it to %s, or pass --dsn", config.DefaultFile)
	}

	opts := cfg.ToMigrateOptions()
	if flags.path != "" {
		opts.Dir = flags.path
	}
	opts.Dialect = migrate.DetectDialect(url)
	opts.RunPrereq = opts.Dialect == query.DialectPostgres
	opts.Force = flags.force
	opts.DryRun = flags.dryRun
	opts.SkipLock = flags.skipLock
	opts.Step = 0
	if cmd == "migrate" && flags.step > 0 {
		opts.Step = flags.step
	}
	if flags.verbose {
		opts.Logger = log.New(os.Stderr, "vorm: ", 0)
	}

	ctx := context.Background()
	conn, err := query.Open(ctx, url)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	runner := migrate.New(conn, opts)

	switch cmd {
	case "status":
		rows, err := runner.Status(ctx)
		if err != nil {
			return err
		}
		printStatus(rows)
		return nil

	case "migrate":
		report, err := runner.Up(ctx)
		printReport(cmd, report, opts.DryRun)
		return err

	case "rollback":
		var report *migrate.Report
		switch {
		case flags.migration != "":
			report, err = runner.DownByName(ctx, flags.migration)
		case strings.EqualFold(flags.steps, "all"):
			report, err = runner.DownAll(ctx)
		default:
			report, err = runner.Down(ctx, flags.step)
		}
		printReport(cmd, report, opts.DryRun)
		return err

	case "fresh", "refresh":
		if !flags.force && !flags.dryRun {
			return fmt.Errorf("%s drops and re-applies every migration — pass --force to confirm", cmd)
		}
		report, err := runner.Fresh(ctx)
		printReport(cmd, report, opts.DryRun)
		return err
	}
	return fmt.Errorf("unknown migration command %q", cmd)
}

func printReport(cmd string, report *migrate.Report, dryRun bool) {
	if report == nil {
		return
	}

	header := "Running migrations"
	switch cmd {
	case "rollback":
		header = "Rolling back migrations"
	case "fresh", "refresh":
		header = "Refreshing migrations"
	}
	if dryRun {
		header = "Dry run: " + header
	}
	fmt.Printf("\n  %s %s.\n\n", cyanBold("INFO"), header)

	const width = 72
	var reverted, applied int
	for _, s := range report.Prereq {
		printStepLine("prereq", s, width)
	}
	for _, s := range report.Steps {
		label := cmd
		if s.Down {
			label = "rollback"
		}
		if s.Applied {
			if s.Down {
				reverted++
			} else {
				applied++
			}
		}
		printStepLine(label, s, width)
	}

	fmt.Println()
	prefix := ""
	if dryRun {
		prefix = "dry run: "
	}
	switch {
	case report.Failed > 0:
		fmt.Printf("  %s %s%d applied, %d failed\n",
			redBold("FAIL"), prefix, report.Applied, report.Failed)
	case report.Applied == 0:
		fmt.Printf("  %s %snothing to do\n", yellowBold("INFO"), prefix)
	case reverted > 0 && applied > 0:
		fmt.Printf("  %s %s%d reverted, %d re-applied in batch %d\n",
			greenBold("DONE"), prefix, reverted, applied, report.Batch)
	case report.Batch > 0:
		fmt.Printf("  %s %s%d migration(s) in batch %d\n",
			greenBold("DONE"), prefix, report.Applied, report.Batch)
	default:
		fmt.Printf("  %s %s%d migration(s)\n",
			greenBold("DONE"), prefix, report.Applied)
	}
	fmt.Println()
}

func printStepLine(label string, s migrate.StepResult, width int) {
	name := s.Name
	if label == "prereq" {
		name = "prereq/" + s.Name
	}
	status, colored := stepStatusParts(s)
	plain := padDots("  "+name, status, width)
	if colorEnabled() {
		idx := strings.LastIndex(plain, status)
		if idx >= 0 {
			plain = plain[:idx] + colored
		}
	}
	fmt.Println(plain)
}

func stepStatus(s migrate.StepResult) string {
	plain, _ := stepStatusParts(s)
	return plain
}

func stepStatusParts(s migrate.StepResult) (plain, colored string) {
	switch {
	case s.Err != nil:
		msg := "FAILED"
		if s.Err.Error() != "" {
			msg = "FAILED: " + s.Err.Error()
		}
		return msg, redBold(msg)
	case s.Skipped:
		msg := "SKIPPED"
		if s.Reason != "" {
			msg = "SKIPPED (" + s.Reason + ")"
		}
		return msg, yellow(msg)
	case s.Applied:
		dur := s.Duration.Round(time.Millisecond).String()
		msg := dur + " DONE"
		return msg, gray(dur) + " " + greenBold("DONE")
	default:
		return "PENDING", yellow("PENDING")
	}
}

func printStatus(rows []migrate.StatusRow) {
	if len(rows) == 0 {
		fmt.Printf("\n  %s no migrations found\n\n", yellowBold("INFO"))
		return
	}
	fmt.Printf("\n  %s Migration status.\n\n", cyanBold("INFO"))

	const width = 72
	pending := 0
	for _, r := range rows {
		var status, colored string
		switch {
		case r.Missing:
			status = "MISSING FILE"
			colored = redBold(status)
			if r.Batch > 0 {
				status = fmt.Sprintf("MISSING FILE (batch %d)", r.Batch)
				colored = redBold(status)
			}
		case r.Applied && !r.ChecksumOK:
			status = fmt.Sprintf("CHANGED (batch %d)", r.Batch)
			colored = yellowBold(status)
		case r.Applied:
			status = fmt.Sprintf("Ran (batch %d)", r.Batch)
			colored = green(status)
		default:
			status = "Pending"
			colored = yellow(status)
			pending++
		}
		line := padDots("  "+r.Name, status, width)
		if colorEnabled() {
			idx := strings.LastIndex(line, status)
			if idx >= 0 {
				line = line[:idx] + colored
			}
		}
		fmt.Println(line)
	}
	fmt.Printf("\n  %s %d migration(s), %s\n\n",
		cyanBold("INFO"), len(rows),
		func() string {
			if pending == 0 {
				return green("nothing pending")
			}
			return yellow(fmt.Sprintf("%d pending", pending))
		}())
}
