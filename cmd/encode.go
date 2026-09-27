package cmd

import (
	"os"

	"github.com/AliceFord/es-compress/exporter"
	"github.com/AliceFord/es-compress/importer"
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

		parsed, err := importer.Parse(string(data))
		if err != nil {
			return err
		}

		outPath, err := cmd.Flags().GetString("output")
		if err != nil {
			return err
		}

		return os.WriteFile(outPath, exporter.Export(parsed), 0o644)
	},
}
