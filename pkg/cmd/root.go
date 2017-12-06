package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	dataDirFlag  = "dataDir"
	logLevelFlag = "logLevel"
	envVarPrefix = "LIBRI_EXP"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "libri-exp",
	Short: "simulated experiment on a libri cluster",
	Long:  "simulated experiment on a libri cluster",
}

// Execute is the main entrypoint for the libri CLI.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringP(dataDirFlag, "d", "",
		"local data directory")
	RootCmd.PersistentFlags().StringP(logLevelFlag, "l", zap.InfoLevel.String(),
		"log level")

	// bind viper flags
	viper.SetEnvPrefix(envVarPrefix)
	viper.AutomaticEnv()
	if err := viper.BindPFlags(RootCmd.PersistentFlags()); err != nil {
		panic(err)
	}
}
