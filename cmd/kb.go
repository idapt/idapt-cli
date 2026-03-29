package cmd

import (
	"fmt"
	"net/url"

	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/input"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
)

var kbCmd = &cobra.Command{
	Use:     "kb",
	Aliases: []string{"knowledge-base"},
	Short:   "Manage knowledge bases",
}

var kbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List knowledge bases",
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

		var resp struct {
			KnowledgeBases []map[string]interface{} `json:"knowledgeBases"`
		}
		if err := client.Get(cmd.Context(), "/api/kb", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.KnowledgeBases, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "ICON", Field: "icon"},
			{Header: "NOTES", Field: "noteCount"},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var kbCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a knowledge base",
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
		if cmd.Flags().Changed("icon") {
			v, _ := cmd.Flags().GetString("icon")
			overrides["icon"] = v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			overrides["description"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/kb", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
		})
	},
}

var kbGetCmd = &cobra.Command{
	Use:   "get <id-or-name>",
	Short: "Get knowledge base details",
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

		id, err := resolveResource(cmd, f, "kb", args[0], projectID)
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/kb/"+id, nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
			{Header: "ICON", Field: "icon"},
			{Header: "NOTES", Field: "noteCount"},
			{Header: "DESCRIPTION", Field: "description", Width: 80},
			{Header: "CREATED", Field: "createdAt"},
		})
	},
}

var kbEditCmd = &cobra.Command{
	Use:   "edit <id-or-name>",
	Short: "Edit a knowledge base",
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

		id, err := resolveResource(cmd, f, "kb", args[0], projectID)
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
		if cmd.Flags().Changed("icon") {
			v, _ := cmd.Flags().GetString("icon")
			overrides["icon"] = v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			overrides["description"] = v
		}
		body = input.MergeFlags(body, overrides)

		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/kb/"+id, body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "NAME", Field: "name"},
		})
	},
}

var kbDeleteCmd = &cobra.Command{
	Use:   "delete <id-or-name>",
	Short: "Delete a knowledge base",
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

		id, err := resolveResource(cmd, f, "kb", args[0], projectID)
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete knowledge base %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/kb/"+id); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Knowledge base %s deleted.\n", args[0])
		return nil
	},
}

var kbAskCmd = &cobra.Command{
	Use:   "ask <id-or-name> <question>",
	Short: "Ask a question against a knowledge base",
	Args:  cobra.ExactArgs(2),
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

		id, err := resolveResource(cmd, f, "kb", args[0], projectID)
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"question": args[1],
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/kb/"+id+"/ask", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ANSWER", Field: "answer", Width: 120},
			{Header: "SOURCES", Field: "sources"},
		})
	},
}

var kbSearchCmd = &cobra.Command{
	Use:   "search <id-or-name> <query>",
	Short: "Search a knowledge base",
	Args:  cobra.ExactArgs(2),
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

		id, err := resolveResource(cmd, f, "kb", args[0], projectID)
		if err != nil {
			return err
		}

		q := url.Values{"q": {args[1]}}

		var resp struct {
			Results []map[string]interface{} `json:"results"`
		}
		if err := client.Get(cmd.Context(), "/api/kb/"+id+"/search", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Results, []output.Column{
			{Header: "NOTE", Field: "noteId"},
			{Header: "TITLE", Field: "title"},
			{Header: "SCORE", Field: "score"},
			{Header: "SNIPPET", Field: "snippet", Width: 80},
		})
	},
}

var kbIngestCmd = &cobra.Command{
	Use:   "ingest <id-or-name> <file-path>",
	Short: "Ingest a file into a knowledge base",
	Args:  cobra.ExactArgs(2),
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

		id, err := resolveResource(cmd, f, "kb", args[0], projectID)
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"filePath": args[1],
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/kb/"+id+"/ingest", body, &resp); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Ingestion started.")
		return nil
	},
}

// --- Note subcommands ---

var kbNoteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage knowledge base notes",
}

var kbNoteListCmd = &cobra.Command{
	Use:   "list <kb-id-or-name>",
	Short: "List notes in a knowledge base",
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

		kbID, err := resolveResource(cmd, f, "kb", args[0], projectID)
		if err != nil {
			return err
		}

		q := buildListQuery(cmd, nil)

		var resp struct {
			Notes []map[string]interface{} `json:"notes"`
		}
		if err := client.Get(cmd.Context(), "/api/kb/"+kbID+"/notes", q, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteList(resp.Notes, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TITLE", Field: "title"},
			{Header: "UPDATED", Field: "updatedAt"},
		})
	},
}

var kbNoteGetCmd = &cobra.Command{
	Use:   "get <note-id>",
	Short: "Get a note's content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		var resp map[string]interface{}
		if err := client.Get(cmd.Context(), "/api/kb/notes/"+args[0], nil, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TITLE", Field: "title"},
			{Header: "CONTENT", Field: "content"},
			{Header: "UPDATED", Field: "updatedAt"},
		})
	},
}

var kbNoteCreateCmd = &cobra.Command{
	Use:   "create <kb-id-or-name>",
	Short: "Create a note",
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

		kbID, err := resolveResource(cmd, f, "kb", args[0], projectID)
		if err != nil {
			return err
		}

		body := map[string]interface{}{}
		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			body["title"] = v
		}
		if cmd.Flags().Changed("content") {
			v, _ := cmd.Flags().GetString("content")
			body["content"] = v
		}
		if cmd.Flags().Changed("content-file") {
			path, _ := cmd.Flags().GetString("content-file")
			content, err := input.ReadFileFlag(path)
			if err != nil {
				return err
			}
			body["content"] = content
		}

		var resp map[string]interface{}
		if err := client.Post(cmd.Context(), "/api/kb/"+kbID+"/notes", body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TITLE", Field: "title"},
		})
	},
}

var kbNoteEditCmd = &cobra.Command{
	Use:   "edit <note-id>",
	Short: "Edit a note",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		body := map[string]interface{}{}
		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			body["title"] = v
		}
		if cmd.Flags().Changed("content") {
			v, _ := cmd.Flags().GetString("content")
			body["content"] = v
		}
		if cmd.Flags().Changed("content-file") {
			path, _ := cmd.Flags().GetString("content-file")
			content, err := input.ReadFileFlag(path)
			if err != nil {
				return err
			}
			body["content"] = content
		}

		var resp map[string]interface{}
		if err := client.Patch(cmd.Context(), "/api/kb/notes/"+args[0], body, &resp); err != nil {
			return err
		}

		formatter := f.Formatter()
		return formatter.WriteItem(resp, []output.Column{
			{Header: "ID", Field: "id"},
			{Header: "TITLE", Field: "title"},
		})
	},
}

var kbNoteDeleteCmd = &cobra.Command{
	Use:   "delete <note-id>",
	Short: "Delete a note",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmdutil.FactoryFromCmd(cmd)
		client, err := f.APIClient()
		if err != nil {
			return err
		}

		if !globalFlags.Confirm {
			if !cmdutil.ConfirmAction(f, fmt.Sprintf("Delete note %s?", args[0])) {
				return fmt.Errorf("aborted")
			}
		}

		if err := client.Delete(cmd.Context(), "/api/kb/notes/"+args[0]); err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Note deleted.")
		return nil
	},
}

func init() {
	cmdutil.AddListFlags(kbListCmd)

	kbCreateCmd.Flags().String("name", "", "Knowledge base name")
	kbCreateCmd.Flags().String("icon", "", "Icon emoji")
	kbCreateCmd.Flags().String("description", "", "Description")
	cmdutil.AddJSONInput(kbCreateCmd)

	kbEditCmd.Flags().String("name", "", "Knowledge base name")
	kbEditCmd.Flags().String("icon", "", "Icon emoji")
	kbEditCmd.Flags().String("description", "", "Description")
	cmdutil.AddJSONInput(kbEditCmd)

	cmdutil.AddListFlags(kbNoteListCmd)

	kbNoteCreateCmd.Flags().String("title", "", "Note title")
	kbNoteCreateCmd.Flags().String("content", "", "Note content")
	kbNoteCreateCmd.Flags().String("content-file", "", "Path to content file")

	kbNoteEditCmd.Flags().String("title", "", "Note title")
	kbNoteEditCmd.Flags().String("content", "", "Note content")
	kbNoteEditCmd.Flags().String("content-file", "", "Path to content file")

	kbNoteCmd.AddCommand(kbNoteListCmd)
	kbNoteCmd.AddCommand(kbNoteGetCmd)
	kbNoteCmd.AddCommand(kbNoteCreateCmd)
	kbNoteCmd.AddCommand(kbNoteEditCmd)
	kbNoteCmd.AddCommand(kbNoteDeleteCmd)

	kbCmd.AddCommand(kbListCmd)
	kbCmd.AddCommand(kbCreateCmd)
	kbCmd.AddCommand(kbGetCmd)
	kbCmd.AddCommand(kbEditCmd)
	kbCmd.AddCommand(kbDeleteCmd)
	kbCmd.AddCommand(kbAskCmd)
	kbCmd.AddCommand(kbSearchCmd)
	kbCmd.AddCommand(kbIngestCmd)
	kbCmd.AddCommand(kbNoteCmd)
}
