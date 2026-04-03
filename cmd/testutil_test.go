package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/idapt/idapt-cli/internal/api"
	"github.com/idapt/idapt-cli/internal/cliconfig"
	"github.com/idapt/idapt-cli/internal/cmdutil"
	"github.com/idapt/idapt-cli/internal/credential"
	"github.com/idapt/idapt-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// testEnv holds the wiring for a single test invocation.
type testEnv struct {
	server  *httptest.Server
	factory *cmdutil.Factory
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
}

// newTestEnv creates a testEnv backed by the given HTTP handler.
// The server is closed automatically via t.Cleanup.
func newTestEnv(t *testing.T, handler http.Handler) *testEnv {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	client, err := api.NewClient(api.ClientConfig{
		BaseURL:    server.URL,
		APIKey:     "test-key",
		CLIVersion: "test",
	})
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}

	f := &cmdutil.Factory{
		Config: cliconfig.Config{
			APIURL:         server.URL,
			DefaultProject: "00000000-0000-0000-0000-000000000001",
		},
		Credentials: credential.Credentials{APIKey: "test-key"},
		Format:      output.FormatJSON,
		Out:         stdout,
		ErrOut:      stderr,
		In:          strings.NewReader(""),
	}
	f.SetClientFn(func() (*api.Client, error) { return client, nil })

	return &testEnv{
		server:  server,
		factory: f,
		stdout:  stdout,
		stderr:  stderr,
	}
}

// runCmd builds the root command tree with the test factory injected,
// then runs it with the given args. Returns captured stdout, stderr, and error.
func runCmd(t *testing.T, handler http.Handler, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	env := newTestEnv(t, handler)
	return runCmdWithEnv(t, env, args...)
}

// runCmdWithEnv executes the root command with a pre-built test environment.
func runCmdWithEnv(t *testing.T, env *testEnv, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	root := &cobra.Command{
		Use:           "idapt",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmdutil.SetFactory(cmd, env.factory)
			syncGlobalFlags(cmd)
			return nil
		},
	}
	root.SetOut(env.stdout)
	root.SetErr(env.stderr)
	root.SetIn(strings.NewReader(""))
	root.SetContext(context.Background())

	// Re-register global flags on this fresh root so child commands find them.
	gf := cmdutil.RegisterGlobalFlags(root)
	// Wire the package-level globalFlags pointer so existing commands can read it.
	savedFlags := globalFlags
	globalFlags = gf
	t.Cleanup(func() { globalFlags = savedFlags })

	// Add the same subcommands that init() registers on the real root.
	root.AddCommand(authCmd)
	root.AddCommand(projectCmd)
	root.AddCommand(agentCmd)
	root.AddCommand(chatCmd)
	root.AddCommand(fileCmd)
	root.AddCommand(machineRemoteCmd)
	root.AddCommand(scriptCmd)
	root.AddCommand(secretCmd)
	root.AddCommand(storeCmd)
	root.AddCommand(modelCmd)
	root.AddCommand(execCmd)
	root.AddCommand(webCmd)
	root.AddCommand(mediaCmd)
	root.AddCommand(settingsCmd)
	root.AddCommand(profileCmd)
	root.AddCommand(subscriptionCmd)
	root.AddCommand(apikeyCmd)
	root.AddCommand(shareCmd)
	root.AddCommand(notificationCmd)
	root.AddCommand(multiAgentCmd)
	root.AddCommand(versionCmd)

	// Reset flag state on all subcommands to prevent state leaking between
	// subtests. pflag does not reset the Changed bit between Parse() calls,
	// so flags set in a previous subtest would still appear as Changed.
	resetFlagState(root)

	root.SetArgs(args)
	execErr := root.Execute()
	return env.stdout.String(), env.stderr.String(), execErr
}

// resetFlagState walks the command tree and resets each flag's Changed bit
// and value to its default. This prevents state leaking between subtests
// when command objects are package-level singletons.
func resetFlagState(cmd *cobra.Command) {
	// Reset Changed bits and values on local flags so flags set by a
	// previous subtest's Execute don't appear as Changed in the next.
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue)
	})
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		f.Value.Set(f.DefValue)
	})
	for _, child := range cmd.Commands() {
		resetFlagState(child)
	}
}

// syncGlobalFlags copies parsed persistent flag values from the executing
// command into the package-level globalFlags struct. This is needed because
// cobra caches inherited persistent flags on child commands; when singleton
// commands are re-parented to a new root, the cached flags still point to
// the old root's bindings. Reading from cmd.Flags() gets the actual parsed
// values regardless of which binding was used.
func syncGlobalFlags(cmd *cobra.Command) {
	if v, err := cmd.Flags().GetBool("confirm"); err == nil {
		globalFlags.Confirm = v
	}
	if v, err := cmd.Flags().GetString("project"); err == nil && v != "" {
		globalFlags.Project = v
	}
	if v, err := cmd.Flags().GetString("api-key"); err == nil && v != "" {
		globalFlags.APIKey = v
	}
	if v, err := cmd.Flags().GetString("output"); err == nil && v != "" {
		globalFlags.Output = v
	}
	if v, err := cmd.Flags().GetBool("verbose"); err == nil {
		globalFlags.Verbose = v
	}
}

// runCmdConfirm runs a command with globalFlags.Confirm pre-set to true.
// Use this for delete/destructive command tests instead of passing --confirm
// as an argument (cobra's cached persistent flag bindings prevent --confirm
// from reaching globalFlags when child commands are singleton objects).
func runCmdConfirm(t *testing.T, handler http.Handler, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	env := newTestEnv(t, handler)
	return runCmdWithEnvConfirm(t, env, args...)
}

func runCmdWithEnvConfirm(t *testing.T, env *testEnv, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	root := &cobra.Command{
		Use:           "idapt",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmdutil.SetFactory(cmd, env.factory)
			return nil
		},
	}
	root.SetOut(env.stdout)
	root.SetErr(env.stderr)
	root.SetIn(strings.NewReader(""))
	root.SetContext(context.Background())

	gf := cmdutil.RegisterGlobalFlags(root)
	savedFlags := globalFlags
	globalFlags = gf
	t.Cleanup(func() { globalFlags = savedFlags })

	root.AddCommand(authCmd)
	root.AddCommand(projectCmd)
	root.AddCommand(agentCmd)
	root.AddCommand(chatCmd)
	root.AddCommand(fileCmd)
	root.AddCommand(machineRemoteCmd)
	root.AddCommand(scriptCmd)
	root.AddCommand(secretCmd)
	root.AddCommand(storeCmd)
	root.AddCommand(modelCmd)
	root.AddCommand(execCmd)
	root.AddCommand(webCmd)
	root.AddCommand(mediaCmd)
	root.AddCommand(settingsCmd)
	root.AddCommand(profileCmd)
	root.AddCommand(subscriptionCmd)
	root.AddCommand(apikeyCmd)
	root.AddCommand(shareCmd)
	root.AddCommand(notificationCmd)
	root.AddCommand(multiAgentCmd)
	root.AddCommand(versionCmd)

	resetFlagState(root)

	// Pre-set confirm AFTER resetFlagState to avoid being undone.
	gf.Confirm = true

	root.SetArgs(args)
	execErr := root.Execute()
	return env.stdout.String(), env.stderr.String(), execErr
}

// mockHandler creates an http.Handler from a map of "METHOD path" -> handler func.
func mockHandler(routes map[string]func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if fn, ok := routes[key]; ok {
			fn(w, r)
			return
		}
		// Fallback: try path-only match (any method)
		if fn, ok := routes[r.URL.Path]; ok {
			fn(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

// jsonResponse writes a JSON response with the given status code.
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// jsonErrorResponse writes a JSON error response matching the API error format.
func jsonErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
		},
	})
}

// readJSONBody reads and parses a JSON request body.
func readJSONBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading request body: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parsing request body: %v (raw: %s)", err, string(data))
	}
	return result
}

// parseJSONOutput parses captured stdout as a JSON object.
func parseJSONOutput(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		t.Fatalf("parsing JSON output: %v\nraw: %s", err, s)
	}
	return result
}

// setupTestCmd creates a root command wired to a test server and returns
// the root command plus its stdout and stderr buffers. Useful when a test
// needs to tweak the command (e.g. inject custom stdin) before executing.
func setupTestCmd(t *testing.T, handler http.Handler) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	env := newTestEnv(t, handler)

	root := &cobra.Command{
		Use:           "idapt",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmdutil.SetFactory(cmd, env.factory)
			syncGlobalFlags(cmd)
			return nil
		},
	}
	root.SetOut(env.stdout)
	root.SetErr(env.stderr)
	root.SetIn(strings.NewReader(""))
	root.SetContext(context.Background())

	gf := cmdutil.RegisterGlobalFlags(root)
	savedFlags := globalFlags
	globalFlags = gf
	t.Cleanup(func() { globalFlags = savedFlags })

	root.AddCommand(authCmd)
	root.AddCommand(projectCmd)
	root.AddCommand(agentCmd)
	root.AddCommand(chatCmd)
	root.AddCommand(fileCmd)
	root.AddCommand(machineRemoteCmd)
	root.AddCommand(scriptCmd)
	root.AddCommand(secretCmd)
	root.AddCommand(storeCmd)
	root.AddCommand(modelCmd)
	root.AddCommand(execCmd)
	root.AddCommand(webCmd)
	root.AddCommand(mediaCmd)
	root.AddCommand(settingsCmd)
	root.AddCommand(profileCmd)
	root.AddCommand(subscriptionCmd)
	root.AddCommand(apikeyCmd)
	root.AddCommand(shareCmd)
	root.AddCommand(notificationCmd)
	root.AddCommand(multiAgentCmd)
	root.AddCommand(versionCmd)

	resetFlagState(root)

	return root, env.stdout, env.stderr
}

// parseJSONArrayOutput parses captured stdout as a JSON array.
func parseJSONArrayOutput(t *testing.T, s string) []map[string]interface{} {
	t.Helper()
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		t.Fatalf("parsing JSON array output: %v\nraw: %s", err, s)
	}
	return result
}
