package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the OmniTun CLI version",
		Long:  "Display version, build commit, Go version, platform, and build date.",
		RunE:  runVersion,
	}
	return cmd
}

type versionOutput struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	BuildDate string `json:"build_date"`
}

func runVersion(cmd *cobra.Command, args []string) error {
	useJSON := isJSON(cmd)

	if useJSON {
		vo := versionOutput{
			Version:   Version,
			Commit:    Commit,
			GoVersion: runtime.Version(),
			Platform:  runtime.GOOS + "/" + runtime.GOARCH,
			BuildDate: BuildDate,
		}
		data, _ := json.MarshalIndent(vo, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s  OmniTun CLI v%s\n", bold("●"), cyan(Version))
	fmt.Printf("%s  %s\n", dim("  "), dim(buildInfo()))
	fmt.Println()
	return nil
}

func buildInfo() string {
	return fmt.Sprintf("commit=%s  go=%s  %s/%s  built=%s",
		Commit[:min(8, len(Commit))],
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		BuildDate,
	)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
