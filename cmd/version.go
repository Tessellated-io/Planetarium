/*
Copyright © 2023 Tessellated Geometry LLC <https://tessellated.io>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the current version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s %s\n", binaryIcon, productName)
		fmt.Printf("   - Version: %s\n", ProductVersion)
		fmt.Printf("   - Git Revision: %s\n", GitRevision)
		fmt.Printf("   - Go Version: %s\n", GoVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
