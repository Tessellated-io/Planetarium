/*
Copyright © 2023 Tessellated Geometry LLC <https://tessellated.io>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tessellated-io/pickaxe/log"
)

var (
	rawLogLevel string
	logger      *log.Logger
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   binaryName,
	Short: rootHelp,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Get a logger
		logLevel := log.ParseLogLevel(rawLogLevel)
		logger = log.NewLogger(logLevel)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&rawLogLevel, "log-level", "l", "info", "Logging level")
}
