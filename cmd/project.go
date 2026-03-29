package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/idapt/idapt-cli/internal/resolve"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:     "project",
	Aliases: []string{"proj"},
	Short:   "Manage projects",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp struct {
			Projects []map[string]interface{} `json:"projects"`
		}
		if err := client.Get(cmd.Context(), "/api/projects", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Projects, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "SLUG", Field: "slug"},
			{Header: "ROLE", Field: "role"},
			{Header: "ICON", Field: "icon"},
		})
	},
}

var projectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{}
		if cmd.Flags().Changed("json") {
			raw, _ := cmd.Flags().GetString("json")
			parsed, err := input.ParseJSONFlag(raw, f.In)
			if err != nil {
				return err
			}
			body = parsed
		}

		overrides := map[string]interface{}{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			overrides["name"] = v
		}
		if cmd.Flags().Changed("slug") {
			v, _ := cmd.Flags().GetString("slug")
			overrides["slug"] = v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			overrides["description"] = v
		}
		if cmd.Flags().Changed("icon") {
			v, _ := cmd.Flags().GetString("icon")
			overrides["icon"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/projects", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "SLUG", Field: "slug"},
		})
	},
}

var projectGetCmd = &cobra.Command{
	Use:   "get <id-or-slug>",
	Short: "Get project details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id := args[0]
		if !resolve.IsUUID(id) {
			resolver, err := f.Resolver()
			if err != nil {
				return err
			}
			id, err = resolver.ResolveProject(cmd.Context(), id)
			if err != nil {
				return err
			}
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/projects/"+id, nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "SLUG", Field: "slug"},
			{Header: "DESCRIPTION", Field: "description"},
			{Header: "ICON", Field: "icon"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var projectEditCmd = &cobra.Command{
	Use:   "edit <id-or-slug>",
	Short: "Edit a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id := args[0]
		if !resolve.IsUUID(id) {
			resolver, err := f.Resolver()
			if err != nil {
				return err
			}
			id, err = resolver.ResolveProject(cmd.Context(), id)
			if err != nil {
				return err
			}
		}

		body := map[string]interface{}{}
		if cmd.Flags().Changed("json") {
			raw, _ := cmd.Flags().GetString("json")
			parsed, err := input.ParseJSONFlag(raw, f.In)
			if err != nil {
				return err
			}
			body = parsed
		}

		overrides := map[string]interface{}{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			overrides["name"] = v
		}
		if cmd.Flags().Changed("slug") {
			v, _ := cmd.Flags().GetString("slug")
			overrides["slug"] = v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			overrides["description"] = v
		}
		if cmd.Flags().Changed("icon") {
			v, _ := cmd.Flags().GetString("icon")
			overrides["icon"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/projects/"+id, body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "SLUG", Field: "slug"},
		})
	},
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <id-or-slug>",
	Short: "Delete a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id := args[0]
		if !resolve.IsUUID(id) {
			resolver, err := f.Resolver()
			if err != nil {
				return err
			}
			id, err = resolver.ResolveProject(cmd.Context(), id)
			if err != nil {
				return err
			}
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete project %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/projects/"+id); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Project %s deleted.\n", args[0])
		return nil
	},
}

var projectForkCmd = &cobra.Command{
	Use:   "fork <id-or-slug>",
	Short: "Fork a project (deep copy)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id := args[0]
		if !resolve.IsUUID(id) {
			resolver, err := f.Resolver()
			if err != nil {
				return err
			}
			id, err = resolver.ResolveProject(cmd.Context(), id)
			if err != nil {
				return err
			}
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/projects/"+id+"/fork", nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "SLUG", Field: "slug"},
		})
	},
}

// --- Member subcommands ---

var projectMemberCmd = &cobra.Command{
	Use:   "member",
	Short: "Manage project members",
}

var projectMemberListCmd = &cobra.Command{
	Use:   "list <project-id-or-slug>",
	Short: "List project members",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id := args[0]
		if !resolve.IsUUID(id) {
			resolver, err := f.Resolver()
			if err != nil {
				return err
			}
			id, err = resolver.ResolveProject(cmd.Context(), id)
			if err != nil {
				return err
			}
		}

		var resp struct {
			Members []map[string]interface{} `json:"members"`
		}
		if err := client.Get(cmd.Context(), "/api/projects/"+id, nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Members, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "EMAIL", Field: "email"},
			{Header: "ROLE", Field: "role"},
			{Header: "JOINED", Field: "createdAt"},
		})
	},
}

var projectMemberAddCmd = &cobra.Command{
	Use:   "add <project-id-or-slug>",
	Short: "Invite a member to a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		id := args[0]
		if !resolve.IsUUID(id) {
			resolver, err := f.Resolver()
			if err != nil {
				return err
			}
			id, err = resolver.ResolveProject(cmd.Context(), id)
			if err != nil {
				return err
			}
		}

		email, _ := cmd.Flags().GetString("email")
		role, _ := cmd.Flags().GetString("role")
		if email == "" {
			return fmt.Errorf("--email is required")
		}

		body := map[string]interface{}{
			"email": email,
		}
		if role != "" {
			body["role"] = role
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/projects/"+id+"/invitations", body, &resp); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Invitation sent to %s.\n", email)
		return nil
	},
}

var projectMemberRemoveCmd = &cobra.Command{
	Use:   "remove <project-id-or-slug> <member-id>",
	Short: "Remove a member from a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID := args[0]
		if !resolve.IsUUID(projectID) {
			resolver, err := f.Resolver()
			if err != nil {
				return err
			}
			projectID, err = resolver.ResolveProject(cmd.Context(), projectID)
			if err != nil {
				return err
			}
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Remove member %s?", args[1])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/projects/"+projectID+"/members/"+args[1]); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Member removed.")
		return nil
	},
}

var projectMemberEditCmd = &cobra.Command{
	Use:   "edit <project-id-or-slug> <member-id>",
	Short: "Edit a member's role",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		projectID := args[0]
		if !resolve.IsUUID(projectID) {
			resolver, err := f.Resolver()
			if err != nil {
				return err
			}
			projectID, err = resolver.ResolveProject(cmd.Context(), projectID)
			if err != nil {
				return err
			}
		}

		role, _ := cmd.Flags().GetString("role")
		if role == "" {
			return fmt.Errorf("--role is required")
		}

		body := map[string]interface{}{"role": role}
		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/projects/"+projectID+"/members/"+args[1], body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Member role updated.")
		return nil
	},
}

func resolveProjectFlag(cmd *cobra.Command, f *cmdutil.Factory) (string, error) {
	project := globalFlags.Project
	if project == "" {
		project = f.Config.DefaultProject
	}
	if project == "" {
		return "", fmt.Errorf("--project is required (or set defaultProject via `idapt config set defaultProject <slug>`)")
	}
	if resolve.IsUUID(project) {
		return project, nil
	}
	resolver, err := f.Resolver()
	if err != nil {
		return "", err
	}
	return resolver.ResolveProject(cmd.Context(), project)
}

func resolveResource(cmd *cobra.Command, f *cmdutil.Factory, resourceType, nameOrID, projectID string) (string, error) {
	if resolve.IsUUID(nameOrID) {
		return nameOrID, nil
	}
	resolver, err := f.Resolver()
	if err != nil {
		return "", err
	}
	return resolver.Resolve(cmd.Context(), resourceType, nameOrID, projectID)
}

func buildListQuery(cmd *cobra.Command, extra url.Values) url.Values {
	q := url.Values{}
	limit, _ := cmd.Flags().GetInt("limit")
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	after, _ := cmd.Flags().GetString("starting-after")
	if after != "" {
		q.Set("starting_after", after)
	}
	for k, vs := range extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	return q
}

func init() {
	projectCreateCmd.Flags().String("name", "", "Project name")
	projectCreateCmd.Flags().String("slug", "", "Project slug")
	projectCreateCmd.Flags().String("description", "", "Project description")
	projectCreateCmd.Flags().String("icon", "", "Project icon emoji")
	cmdutil.AddJSONInput(projectCreateCmd)

	projectEditCmd.Flags().String("name", "", "Project name")
	projectEditCmd.Flags().String("slug", "", "Project slug")
	projectEditCmd.Flags().String("description", "", "Project description")
	projectEditCmd.Flags().String("icon", "", "Project icon emoji")
	cmdutil.AddJSONInput(projectEditCmd)

	projectMemberAddCmd.Flags().String("email", "", "Member email address")
	projectMemberAddCmd.Flags().String("role", "viewer", "Member role (viewer, editor, admin)")
	projectMemberEditCmd.Flags().String("role", "", "New role (viewer, editor, admin)")

	projectMemberCmd.AddCommand(projectMemberListCmd)
	projectMemberCmd.AddCommand(projectMemberAddCmd)
	projectMemberCmd.AddCommand(projectMemberRemoveCmd)
	projectMemberCmd.AddCommand(projectMemberEditCmd)

	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectEditCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectForkCmd)
	projectCmd.AddCommand(projectMemberCmd)
}
