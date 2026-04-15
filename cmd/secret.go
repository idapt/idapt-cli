package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secrets",
}

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secrets",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		q := buildListQuery(cmd, url.Values{"projectId": {projectID}})
		if cmd.Flags().Changed("type") {
			v, _ := cmd.Flags().GetString("type")
			q.Set("type", v)
		}

		var resp struct {
			Secrets []map[string]interface{} `json:"secrets"`
		}
		if err := client.Get(cmd.Context(), "/api/secrets", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Secrets, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "type"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var secretCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a secret",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		body := map[string]interface{}{"projectId": projectID}
		if cmd.Flags().Changed("json") {
			raw, _ := cmd.Flags().GetString("json")
			parsed, err := input.ParseJSONFlag(raw, f.In)
			if err != nil {
				return err
			}
			body = input.MergeFlags(parsed, map[string]interface{}{"projectId": projectID})
		}

		overrides := map[string]interface{}{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			overrides["name"] = v
		}
		if cmd.Flags().Changed("value") {
			v, _ := cmd.Flags().GetString("value")
			overrides["value"] = v
		}
		if cmd.Flags().Changed("value-file") {
			path, _ := cmd.Flags().GetString("value-file")
			content, err := input.ReadFileFlag(path)
			if err != nil {
				return err
			}
			overrides["value"] = content
		}
		if cmd.Flags().Changed("type") {
			v, _ := cmd.Flags().GetString("type")
			overrides["type"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/secrets", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "type"},
		})
	},
}

var secretGetCmd = &cobra.Command{
	Use:   "get <id-or-name>",
	Short: "Get secret details (value is masked)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		id, err := resolveResource(cmd, f, "secret", args[0], projectID)
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/secrets/"+id, nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "TYPE", Field: "type"},
			{Header: "CREATED", Field: "createdAt"},
			{Header: "UPDATED", Field: "updatedAt"},
		})
	},
}

var secretEditCmd = &cobra.Command{
	Use:   "edit <id-or-name>",
	Short: "Edit a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		id, err := resolveResource(cmd, f, "secret", args[0], projectID)
		if err != nil {
			return err
		}

		body := map[string]interface{}{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			body["name"] = v
		}
		if cmd.Flags().Changed("value") {
			v, _ := cmd.Flags().GetString("value")
			body["value"] = v
		}
		if cmd.Flags().Changed("value-file") {
			path, _ := cmd.Flags().GetString("value-file")
			content, err := input.ReadFileFlag(path)
			if err != nil {
				return err
			}
			body["value"] = content
		}

		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/secrets/"+id, body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Secret updated.")
		return nil
	},
}

var secretDeleteCmd = &cobra.Command{
	Use:   "delete <id-or-name>",
	Short: "Delete a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		id, err := resolveResource(cmd, f, "secret", args[0], projectID)
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete secret %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/secrets/"+id); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Secret %s deleted.\n", args[0])
		return nil
	},
}

var secretGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a random secret",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		length, _ := cmd.Flags().GetInt("length")

		body := map[string]interface{}{
			"projectId": projectID,
			"type":      "generic",
			"generate":  true,
		}
		if name != "" {
			body["name"] = name
		}
		if length > 0 {
			body["length"] = length
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/secrets", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "VALUE", Field: "value"},
		})
	},
}

var secretGenerateKeypairCmd = &cobra.Command{
	Use:   "generate-keypair",
	Short: "Generate an SSH keypair",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")

		body := map[string]interface{}{
			"projectId": projectID,
		}
		if name != "" {
			body["name"] = name
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/projects/"+projectID+"/secrets/ssh-keypair", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "PRIVATE KEY ID", Field: "privateKeyId"},
			{Header: "PUBLIC KEY ID", Field: "publicKeyId"},
			{Header: "PUBLIC KEY", Field: "publicKey"},
		})
	},
}

var secretMountCmd = &cobra.Command{
	Use:   "mount <secret-id> <machine-id-or-name> <path>",
	Short: "Mount a secret to a path on a machine",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		machineID, err := resolveMachine(cmd, f, args[1])
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"machineId": machineID,
			"path":      args[2],
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/secrets/"+args[0]+"/mounts", body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Secret mounted.")
		return nil
	},
}

var secretUnmountCmd = &cobra.Command{
	Use:   "unmount <secret-id> <mount-id>",
	Short: "Unmount a secret from a machine path",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if err := client.Delete(cmd.Context(), "/api/secrets/"+args[0]+"/mounts/"+args[1]); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Secret unmounted.")
		return nil
	},
}

var secretMountsCmd = &cobra.Command{
	Use:   "mounts <secret-id>",
	Short: "List mounts for a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp struct {
			Mounts []map[string]interface{} `json:"mounts"`
		}
		if err := client.Get(cmd.Context(), "/api/secrets/"+args[0]+"/mounts", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Mounts, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "MACHINE", Field: "machineId"},
			{Header: "PATH", Field: "path"},
		})
	},
}

var secretComposeEnvCmd = &cobra.Command{
	Use:   "compose-env",
	Short: "Output secrets as .env format for docker-compose",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectFlag(cmd, f)
		if err != nil {
			return err
		}

		q := url.Values{"projectId": {projectID}, "format": {"env"}}

		var resp struct {
			Secrets []map[string]interface{} `json:"secrets"`
		}
		if err := client.Get(cmd.Context(), "/api/secrets", q, &resp); err != nil {
			return err
		}

		for _, s := range resp.Secrets {
			name, _ := s["name"].(string)
			value, _ := s["value"].(string)
			if name != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", name, value)
			}
		}
		return nil
	},
}

func init() {
	cmdutil.AddListFlags(secretListCmd)
	secretListCmd.Flags().String("type", "", "Filter by type (generic, ssh_private_key, ssh_public_key, password)")

	secretCreateCmd.Flags().String("name", "", "Secret name")
	secretCreateCmd.Flags().String("value", "", "Secret value")
	secretCreateCmd.Flags().String("value-file", "", "Path to file containing secret value")
	secretCreateCmd.Flags().String("type", "generic", "Secret type (generic, ssh_private_key, ssh_public_key, password)")
	cmdutil.AddJSONInput(secretCreateCmd)

	secretEditCmd.Flags().String("name", "", "Secret name")
	secretEditCmd.Flags().String("value", "", "New secret value")
	secretEditCmd.Flags().String("value-file", "", "Path to file containing new secret value")

	secretGenerateCmd.Flags().String("name", "", "Secret name")
	secretGenerateCmd.Flags().Int("length", 32, "Secret length")

	secretGenerateKeypairCmd.Flags().String("name", "", "Keypair name prefix")

	secretCmd.AddCommand(secretListCmd)
	secretCmd.AddCommand(secretCreateCmd)
	secretCmd.AddCommand(secretGetCmd)
	secretCmd.AddCommand(secretEditCmd)
	secretCmd.AddCommand(secretDeleteCmd)
	secretCmd.AddCommand(secretGenerateCmd)
	secretCmd.AddCommand(secretGenerateKeypairCmd)
	secretCmd.AddCommand(secretMountCmd)
	secretCmd.AddCommand(secretUnmountCmd)
	secretCmd.AddCommand(secretMountsCmd)
	secretCmd.AddCommand(secretComposeEnvCmd)
}
