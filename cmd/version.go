package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("impact version %s\n", rootCmd.Version)
		fmt.Println("Multi-language code change impact analyzer")
		fmt.Println("Supports: Go, Java, JavaScript, TypeScript, Rust")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
