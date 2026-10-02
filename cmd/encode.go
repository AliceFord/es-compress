package cmd

import (
	"fmt"
	"os"

	exportesc "github.com/AliceFord/es-compress/exporter/esc"
	importreplay "github.com/AliceFord/es-compress/importer/replay"
	"github.com/spf13/cobra"
)

func init() {
	encodeCmd.Flags().StringP("output", "o", "output.esc", "output file")
	rootCmd.AddCommand(encodeCmd)
}

var encodeCmd = &cobra.Command{
	Use:   "encode [file]",
	Short: "Encode an ES replay file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}

		parsed, err := importreplay.Parse(string(data))
		if err != nil {
			return err
		}

		fmt.Printf("%+v\n", parsed)

		outPath, err := cmd.Flags().GetString("output")
		if err != nil {
			return err
		}

		return os.WriteFile(outPath, exportesc.Export(parsed), 0o644)
	},
}
