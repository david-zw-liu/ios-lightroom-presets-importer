package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/davidliu/lrpush/internal/device"
)

func newDevicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "devices",
		Short: "List connected USB devices and their udids",
		RunE: func(cmd *cobra.Command, args []string) error {
			infos, err := device.List()
			if err != nil {
				return err
			}
			if len(infos) == 0 {
				fmt.Println("no devices connected")
				return nil
			}
			fmt.Printf("%d device(s):\n", len(infos))
			for _, in := range infos {
				if in.Err != "" {
					fmt.Printf("  %s  (could not read values: %s)\n", in.UDID, in.Err)
					continue
				}
				fmt.Printf("  %s  %q  %s  iOS %s\n", in.UDID, in.Name, in.ProductType, in.Version)
			}
			return nil
		},
	}
}
