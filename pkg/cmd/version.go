package cmd

import (
	"os"

	"github.com/drausin/libri-experiments/pkg/version"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the libri-experiments version",
	Long:  "print the libri-experiments version",
	Run: func(cmd *cobra.Command, args []string) {
		_, err := os.Stdout.WriteString(version.Current.Version.String() + "\n")
		errors.MaybePanic(err)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
