package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/davidliu/lrpush/internal/device"
	"github.com/davidliu/lrpush/internal/locate"
	"github.com/davidliu/lrpush/internal/rmsync"
)

func newRmCmd() *cobra.Command {
	var (
		backupDir string
		commit    bool
		catalog   string
	)
	cmd := &cobra.Command{
		Use:   "rm <path>...",
		Short: "Delete files/folders from the device userStyles (default dry-run)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commit {
				fmt.Print(warningBanner())
			}
			sess, err := device.Connect(flagUDID, flagBundleID)
			if err != nil {
				return err
			}
			defer sess.Close()
			fmt.Printf("device: %s   bundle: %s\n", sess.Label, flagBundleID)

			docsRoot, err := locate.DocumentsRoot(sess.FS, flagPathPrefix)
			if err != nil {
				return err
			}
			cands, err := locate.FindCatalogs(sess.FS, docsRoot)
			if err != nil {
				return err
			}
			chosen, err := locate.SelectCatalog(cands, catalog, terminalPicker)
			if err != nil {
				return err
			}
			fmt.Printf("target userStyles: %s\n", chosen.UserStyles)

			if backupDir == "" {
				backupDir = filepath.Join("./_userStyles_backup", time.Now().Format("20060102-150405"))
			}
			targets := rmsync.PlanRm(sess.FS, chosen.UserStyles, args)
			return rmsync.Execute(sess.FS, targets, rmsync.ExecOptions{
				BackupDir: backupDir,
				Commit:    commit,
				Out:       os.Stdout,
			})
		},
	}
	cmd.Flags().StringVar(&backupDir, "backup-dir", "", "backup dir (default ./_userStyles_backup/<timestamp>)")
	cmd.Flags().BoolVar(&commit, "commit", false, "actually delete on device (otherwise dry-run)")
	cmd.Flags().StringVar(&catalog, "catalog", "", "select catalog by name (non-interactive)")
	return cmd
}
