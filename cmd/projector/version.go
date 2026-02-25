package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// Injected at build time via -ldflags:
//
//	go build -ldflags "-X main.version=v1.2.3 -X main.commit=abc1234 -X main.buildDate=2026-02-24"
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// versionString returns the formatted version block used by both
// `pj version` and `pj --version`.
func versionString() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "📽️  pj %s\n", version)
	fmt.Fprintf(&sb, "    %-10s%s\n", "commit", commit)
	fmt.Fprintf(&sb, "    %-10s%s\n", "built", buildDate)
	fmt.Fprintf(&sb, "    %-10s%s\n", "go", runtime.Version())
	fmt.Fprintf(&sb, "    %-10s%s/%s\n", "platform", runtime.GOOS, runtime.GOARCH)
	return sb.String()
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version and build info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(versionString())
		},
	}
}
