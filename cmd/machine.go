package cmd

import (
	"github.com/spf13/cobra"
)

var machineRemoteCmd = &cobra.Command{
	Use:     "machine",
	Aliases: []string{"m"},
	Short:   "Manage remote machines",
}

func init() {
	machineRemoteCmd.AddCommand(machineListCmd)
	machineRemoteCmd.AddCommand(machineCreateCmd)
	machineRemoteCmd.AddCommand(machineGetCmd)
	machineRemoteCmd.AddCommand(machineEditCmd)
	machineRemoteCmd.AddCommand(machineStartCmd)
	machineRemoteCmd.AddCommand(machineStopCmd)
	machineRemoteCmd.AddCommand(machineTerminateCmd)
	machineRemoteCmd.AddCommand(machineActivityCmd)
	machineRemoteCmd.AddCommand(machineExecCmd)
	machineRemoteCmd.AddCommand(machineTmuxCmd)
	machineRemoteCmd.AddCommand(machineFileCmd)
	machineRemoteCmd.AddCommand(machineDirCmd)
	machineRemoteCmd.AddCommand(machineFwCmd)
	machineRemoteCmd.AddCommand(machineUserCmd)
	machineRemoteCmd.AddCommand(machinePortCmd)
}
