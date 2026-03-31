// Package resolve provides resource name-to-ID resolution.
package resolve

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	"github.com/idapt/idapt-cli/internal/api"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID returns true if the string looks like a UUID.
func IsUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

type cacheKey struct {
	resourceType string
	name         string
	projectID    string
}

// Resolver resolves short names/slugs to UUIDs.
type Resolver struct {
	client *api.Client
	cache  map[cacheKey]string
}

// New creates a new Resolver.
func New(client *api.Client) *Resolver {
	return &Resolver{
		client: client,
		cache:  make(map[cacheKey]string),
	}
}

// ResolveProject resolves a project slug to a project ID.
func (r *Resolver) ResolveProject(ctx context.Context, slug string) (string, error) {
	if IsUUID(slug) {
		return slug, nil
	}
	if slug == "" {
		return "", fmt.Errorf("project is required; use --project or `idapt config set defaultProject <slug>`")
	}

	key := cacheKey{resourceType: "project", name: slug}
	if id, ok := r.cache[key]; ok {
		return id, nil
	}

	var resp struct {
		Projects []struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"projects"`
	}
	q := url.Values{"slug": {slug}}
	if err := r.client.Get(ctx, "/api/projects", q, &resp); err != nil {
		return "", err
	}

	for _, p := range resp.Projects {
		if p.Slug == slug {
			r.cache[key] = p.ID
			return p.ID, nil
		}
	}

	return "", fmt.Errorf("project %q not found", slug)
}

// Resolve resolves a resource name to an ID within a project.
func (r *Resolver) Resolve(ctx context.Context, resourceType, name, projectID string) (string, error) {
	if IsUUID(name) {
		return name, nil
	}
	if name == "" {
		return "", fmt.Errorf("%s name cannot be empty", resourceType)
	}
	if projectID == "" {
		return "", fmt.Errorf("project is required to resolve %s %q", resourceType, name)
	}

	key := cacheKey{resourceType: resourceType, name: name, projectID: projectID}
	if id, ok := r.cache[key]; ok {
		return id, nil
	}

	// Map resource type to API path and field names
	apiPath, nameField := resourceAPIPath(resourceType)
	q := url.Values{"projectId": {projectID}, nameField: {name}}

	var resp map[string]interface{}
	if err := r.client.Get(ctx, apiPath, q, &resp); err != nil {
		return "", err
	}

	// Extract items from response (varies by resource type)
	items := extractItems(resp, resourceType)
	if len(items) == 0 {
		return "", fmt.Errorf("%s %q not found in project", resourceType, name)
	}
	if len(items) > 1 {
		return "", fmt.Errorf("%s %q is ambiguous (%d matches)", resourceType, name, len(items))
	}

	id, ok := items[0]["id"].(string)
	if !ok {
		return "", fmt.Errorf("unexpected response format for %s %q", resourceType, name)
	}

	r.cache[key] = id
	return id, nil
}

func resourceAPIPath(resourceType string) (path, nameField string) {
	switch resourceType {
	case "agent":
		return "/api/agents", "name"
	case "machine":
		return "/api/machines", "name"
	case "script":
		return "/api/scripts", "name"
	case "secret":
		return "/api/secrets", "name"
	case "skill":
		return "/api/skills", "name"
	case "chat":
		return "/api/chat", "title"
	default:
		return "/api/" + resourceType + "s", "name"
	}
}

func extractItems(resp map[string]interface{}, resourceType string) []map[string]interface{} {
	// Try common response keys
	for _, key := range []string{resourceType + "s", "agents", "machines", "scripts", "secrets", "skills", "chats", "data"} {
		if items, ok := resp[key]; ok {
			if arr, ok := items.([]interface{}); ok {
				result := make([]map[string]interface{}, 0, len(arr))
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						result = append(result, m)
					}
				}
				return result
			}
		}
	}
	return nil
}
