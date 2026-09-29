package cmd

import (
	"os"

	exportreplay "github.com/AliceFord/es-compress/exporter/replay"
	importreplay "github.com/AliceFord/es-compress/importer/replay"
	"github.com/spf13/cobra"
)

func init() {
	compressCmd.Flags().StringP("output", "o", "output.txt", "output file")
	rootCmd.AddCommand(compressCmd)
}

var compressCmd = &cobra.Command{
	Use:   "compress [file]",
	Short: "Compress an ES replay file directly to txt format",
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

		exported := exportreplay.Export(parsed)

		outPath, err := cmd.Flags().GetString("output")
		if err != nil {
			return err
		}

		return os.WriteFile(outPath, []byte(exported), 0o644)
	},
}
