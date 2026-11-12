package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/openexercise/open-food-log/conformance"
)

// runConformance runs the REST conformance script and prints each check. The token comes from
// --token or OFL_TOKEN; the environment keeps it out of the process list and shell history.
func runConformance(args []string) error {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	token := flags.String("token", "", "access token with meals:read and meals:write (default $OFL_TOKEN)")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	timeout := flags.Duration("timeout", 2*time.Minute, "give up after this long")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: ofl conformance [--json] [--token T] <base-url>\n\n"+
			"Logs one meal in the token's account, checks the API's behaviour with it, and deletes it.\n"+
			"Use a test account.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return errors.New("want one base URL, such as https://api.lifterclub.com")
	}
	if *token == "" {
		*token = os.Getenv("OFL_TOKEN")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	report, err := conformance.Run(ctx, conformance.Config{BaseURL: flags.Arg(0), Token: *token})
	if err != nil {
		return err
	}
	if err := printReport(report, report.Results, *asJSON, "checks"); err != nil {
		return err
	}
	if !report.Passed() {
		return errors.New("not conformant")
	}
	return nil
}

// runMCPConformance checks an MCP server against the tool profile's core. The tokens come from
// flags or OFL_TOKEN and OFL_READ_TOKEN.
func runMCPConformance(args []string) error {
	flags := flag.NewFlagSet("conformance mcp", flag.ContinueOnError)
	token := flags.String("token", "", "token a person's assistant would use, able to write (default $OFL_TOKEN)")
	readToken := flags.String("read-token", "", "a read-only token for the same person (default $OFL_READ_TOKEN); without one, read-only access isn't checked and the server can't pass")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	timeout := flags.Duration("timeout", 2*time.Minute, "give up after this long")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: ofl conformance mcp [--json] [--token T] [--read-token R] <mcp-url>\n\n"+
			"Logs one meal in the token's account with the profile's core tools, checks the server's behaviour with it, and deletes it.\n"+
			"Use a test account.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return errors.New("want one MCP endpoint URL, such as https://mcp.lifterclub.com")
	}
	if *token == "" {
		*token = os.Getenv("OFL_TOKEN")
	}
	if *readToken == "" {
		*readToken = os.Getenv("OFL_READ_TOKEN")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	report, err := conformance.RunMCP(ctx, conformance.MCPConfig{URL: flags.Arg(0), Token: *token, ReadOnlyToken: *readToken})
	if err != nil {
		return err
	}
	if err := printReport(report, report.Results, *asJSON, "checks"); err != nil {
		return err
	}
	if !report.Passed() {
		return errors.New("not conformant")
	}
	return nil
}

// runVectors runs every published amount case through a command: the runner contract. The command
// gets one case on stdin, as JSON with its food and amount, and prints what it works out as JSON
// (conformance.VectorResult) on stdout; it runs once per case.
func runVectors(args []string) error {
	flags := flag.NewFlagSet("vectors", flag.ContinueOnError)
	asJSON := flags.Bool("json", false, "print the report as JSON")
	timeout := flags.Duration("timeout", 10*time.Second, "give up on a case after this long")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: ofl vectors [--json] -- <command> [args...]\n\n"+
			"Runs the command once per case in schema/v0/amount-vectors.json, with {\"name\", \"food\", \"amount\"} on stdin,\n"+
			"and compares the JSON it prints: {\"grams\": number, \"nutrients\": {...}} or {\"error\": \"density_unknown\"}.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		flags.Usage()
		return errors.New("want a command to run for each case")
	}
	command := flags.Args()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	report, err := conformance.CheckVectors(ctx, func(ctx context.Context, input json.RawMessage) (conformance.VectorResult, error) {
		ctx, cancel := context.WithTimeout(ctx, *timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Stdin = bytes.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			return conformance.VectorResult{}, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
		}
		var out conformance.VectorResult
		if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
			return conformance.VectorResult{}, fmt.Errorf("the output isn't a result: %v", err)
		}
		return out, nil
	})
	if err != nil {
		return err
	}
	if err := printReport(report, report.Results, *asJSON, "cases"); err != nil {
		return err
	}
	if !report.Passed() {
		return errors.New("the implementation disagrees with the published cases")
	}
	return nil
}

// printReport writes a report as JSON, or one line per result and a count.
func printReport(report any, results []conformance.Result, asJSON bool, noun string) error {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	counts := map[conformance.Status]int{}
	for _, r := range results {
		counts[r.Status]++
		line := strings.ToUpper(string(r.Status)) + " " + r.Name
		if r.Detail != "" {
			line += ": " + r.Detail
		}
		fmt.Println(line)
	}
	fmt.Printf("%d %s: %d passed, %d failed, %d skipped\n", len(results), noun, counts[conformance.Pass], counts[conformance.Fail], counts[conformance.Skip])
	return nil
}
