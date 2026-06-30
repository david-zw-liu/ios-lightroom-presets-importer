package main

import "github.com/spf13/cobra"

var (
	flagUDID       string
	flagBundleID   string
	flagPathPrefix string
)

var rootCmd = &cobra.Command{
	Use:           "lrpush",
	Short:         "Push Lightroom presets to an iPhone's Lightroom mobile app over USB",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagUDID, "udid", "", "target device udid (default: first USB device)")
	pf.StringVar(&flagBundleID, "bundle-id", "com.adobe.lrmobile", "app bundle id")
	pf.StringVar(&flagPathPrefix, "path-prefix", "", "override AFC root prefix (e.g. Documents)")

	rootCmd.AddCommand(newInspectCmd(), newPushCmd(), newRmCmd())
}

func Execute() error { return rootCmd.Execute() }
