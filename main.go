// Command linkrot crawls a website and concurrently checks every link and asset it references,
// reporting the broken ones. It is dependency-light (only golang.org/x/net/html) and fast.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/andreaisabelmontana/linkrot/internal/linkrot"
)

func main() {
	opts := linkrot.DefaultOptions()
	format := flag.String("format", "table", "output format: table | json")
	onlyBroken := flag.Bool("only-broken", false, "show only broken links in table output")
	failOnError := flag.Bool("fail-on-error", false, "exit non-zero if any link is broken (CI gate)")
	timeoutSec := flag.Int("timeout", 15, "per-request timeout in seconds")
	flag.IntVar(&opts.Concurrency, "concurrency", opts.Concurrency, "max simultaneous requests")
	flag.IntVar(&opts.MaxDepth, "depth", opts.MaxDepth, "how many link-hops to crawl from the start page")
	flag.BoolVar(&opts.SameHostOnly, "internal-only", false, "only check links on the start host")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	opts.Timeout = time.Duration(*timeoutSec) * time.Second
	start := flag.Arg(0)

	// Ctrl-C cancels the crawl cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Fprintf(os.Stderr, "Crawling %s (depth %d, concurrency %d)…\n", start, opts.MaxDepth, opts.Concurrency)
	results, err := linkrot.New(opts).Run(ctx, start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch *format {
	case "json":
		fmt.Println(linkrot.JSON(results))
	case "table":
		fmt.Print(linkrot.Table(results, *onlyBroken))
	default:
		fmt.Fprintf(os.Stderr, "error: unknown format %q (want table|json)\n", *format)
		os.Exit(2)
	}

	s := linkrot.Summarize(results)
	fmt.Fprintf(os.Stderr, "\n%d links checked — %d ok, %d broken, %d skipped\n",
		s.Total, s.OK, s.Broken, s.Skipped)

	if *failOnError && s.Broken > 0 {
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `linkrot — fast concurrent broken-link checker

usage:
  linkrot [flags] <url>

example:
  linkrot --internal-only --fail-on-error https://example.com

flags:
`)
	flag.PrintDefaults()
}
