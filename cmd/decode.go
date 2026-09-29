package cmd

import (
	"log/slog"
	"os"

	exportreplay "github.com/AliceFord/es-compress/exporter/replay"
	importesc "github.com/AliceFord/es-compress/importer/esc"
	"github.com/spf13/cobra"
)

func init() {
	decodeCmd.Flags().StringP("output", "o", "output.txt", "output file")
	rootCmd.AddCommand(decodeCmd)
}

var decodeCmd = &cobra.Command{
	Use:   "decode [file]",
	Short: "Decode an ES replay file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		file, err := os.Open(args[0])
		if err != nil {
			return err
		}

		defer file.Close()

		parsed, err := importesc.NewParser(logger).Parse(file)
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
