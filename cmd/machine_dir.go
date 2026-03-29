package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

var machineDirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Upload/download directories on a machine",
}

var machineDirUploadCmd = &cobra.Command{
	Use:   "upload <machine-id-or-name> <local-dir> <remote-path>",
	Short: "Upload a directory (as tarball) to a machine",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		// Open the local directory as a tar for upload
		file, err := os.Open(args[1])
		if err != nil {
			return fmt.Errorf("opening directory: %w", err)
		}
		defer file.Close()

		fi, err := file.Stat()
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return fmt.Errorf("%s is not a directory", args[1])
		}

		fields := map[string]string{
			"path":     args[2],
			"isDir":    "true",
			"localDir": args[1],
		}

		resp, err := client.Upload(cmd.Context(), "/api/machines/"+id+"/sftp/upload", fi.Name(), file, fields)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		fmt.Fprintf(cmd.OutOrStdout(), "Directory uploaded to %s.\n", args[2])
		return nil
	},
}

var machineDirDownloadCmd = &cobra.Command{
	Use:   "download <machine-id-or-name> <remote-path> [local-dir]",
	Short: "Download a directory from a machine (as tarball)",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id, err := resolveMachine(cmd, f, args[0])
		if err != nil {
			return err
		}

		q := url.Values{"path": {args[1]}, "isDir": {"true"}}
		result, err := client.Download(cmd.Context(), "/api/machines/"+id+"/sftp/download?"+q.Encode())
		if err != nil {
			return err
		}
		defer result.Body.Close()

		outPath := "download.tar.gz"
		if len(args) > 2 {
			outPath = args[2]
		}

		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()

		n, err := io.Copy(out, result.Body)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Directory downloaded to %s (%d bytes).\n", outPath, n)
		return nil
	},
}

func init() {
	machineDirCmd.AddCommand(machineDirUploadCmd)
	machineDirCmd.AddCommand(machineDirDownloadCmd)
}
