/*
Copyright © 2023 Tessellated Geometry LLC <https://tessellated.io>
*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tessellated-io/planetarium/server"
)

var serverPort int

var (
	chainRegistryDirectory     string
	validatorRegistryDirectory string
)

var startCommand = &cobra.Command{
	Use:   "start",
	Short: startHelp,
	Run: func(cmd *cobra.Command, args []string) {
		server, err := server.NewServer(chainRegistryDirectory, validatorRegistryDirectory, logger)
		if err != nil {
			logger.Error().Err(err).Msg("fatal error")
			return
		}

		go func() {
			err = server.Start(serverPort)
			if err != nil {
				logger.Error().Err(err).Msg("fatal error")
				return
			}
		}()

		logger.Info().Int("listen_port", serverPort).Msg(fmt.Sprintf("💫 %s %s service started and listening", binaryIcon, binaryName))

		// Wait for a signal to gracefully stop the server
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
	},
}

func init() {
	startCommand.Flags().IntVarP(&serverPort, "port", "p", defaultListenPort, fmt.Sprintf("Listening port for the %s service", binaryName))
	startCommand.Flags().StringVarP(&chainRegistryDirectory, "chain-registry-directory", "c", "", "Where to serve chain registry data from")
	startCommand.Flags().StringVarP(&validatorRegistryDirectory, "validator-registry-directory", "v", "", "Where to serve validator registry data from")

	// Mark the flag as required, forcing the user to provide a value
	err := startCommand.MarkFlagRequired("chain-registry-directory")
	if err != nil {
		log.Fatal("unable to mark --chain-registry-directory as required")
		panic(err)
	}
	err = startCommand.MarkFlagRequired("validator-registry-directory")
	if err != nil {
		log.Fatal("unable to mark --validator-registry-directory as required")
		panic(err)
	}

	rootCmd.AddCommand(startCommand)
}
