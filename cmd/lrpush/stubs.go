package main

import "github.com/spf13/cobra"

func newInspectCmd() *cobra.Command { return &cobra.Command{Use: "inspect"} }
func newPushCmd() *cobra.Command    { return &cobra.Command{Use: "push"} }
func newRmCmd() *cobra.Command      { return &cobra.Command{Use: "rm"} }
