package docs

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

const (
	browserURL = "https://kube-pg-upgrade.containerinfra.com"
)

// NewOpenDocs returns the Cobra version sub command
func NewOpenDocs() *cobra.Command {
	var versionCmd = &cobra.Command{
		Use:   "online-docs",
		Short: "Open the online documentation for kube-pg-upgrade",
		Long:  `Open the online documentation for kube-pg-upgrade. This will open a new tab in your default browser for kube-pg-upgrade.containerinfra.com.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Opening %s ...\n", browserURL)
			return openBrowser(browserURL)
		},
	}

	return versionCmd
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}
