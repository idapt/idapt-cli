//go:build integration

package integration

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func TestIntegration_Secret_Lifecycle(t *testing.T) {
	skipIfNoServer(t)

	projectID := createProjectForTest(t)
	secretName := fmt.Sprintf("CLI_TEST_%d", rand.Intn(999999))

	// 1. Create secret
	status, result := rawPost(t, fmt.Sprintf("/api/projects/%s/secrets", projectID), map[string]interface{}{
		"name":  secretName,
		"value": "super-secret-value-12345",
		"type":  "generic",
	})
	if status != 201 {
		t.Fatalf("create secret returned %d, want 201; body: %v", status, result)
	}
	secret := getMap(result, "secret")
	if secret == nil {
		secret = result
	}
	secretID := getString(secret, "id")
	if secretID == "" {
		t.Fatalf("no secret ID in create response: %v", result)
	}
	t.Cleanup(func() {
		rawDelete(t, fmt.Sprintf("/api/secrets/%s", secretID))
	})

	// 2. List secrets
	status, result = rawGet(t, fmt.Sprintf("/api/projects/%s/secrets", projectID))
	if status != 200 {
		t.Fatalf("list secrets returned %d; body: %v", status, result)
	}
	secrets := getSlice(result, "secrets")
	if !containsID(secrets, secretID) {
		t.Fatalf("secret %s not found in list (%d items); response: %v", secretID, len(secrets), result)
	}

	// 3. Get secret detail
	status, result = rawGet(t, fmt.Sprintf("/api/secrets/%s", secretID))
	if status != 200 {
		t.Fatalf("get secret returned %d; body: %v", status, result)
	}
	got := getMap(result, "secret")
	if got == nil {
		got = result
	}
	if getString(got, "name") != secretName {
		t.Fatalf("secret name = %q, want %q", getString(got, "name"), secretName)
	}

	// 4. Update secret
	newDescription := "Updated description"
	status, result = rawPatch(t, fmt.Sprintf("/api/secrets/%s", secretID), map[string]interface{}{
		"description": newDescription,
	})
	if status != 200 {
		t.Fatalf("patch secret returned %d; body: %v", status, result)
	}

	// 5. Delete secret
	status, _ = rawDelete(t, fmt.Sprintf("/api/secrets/%s", secretID))
	if status != 204 {
		t.Fatalf("delete secret returned %d, want 204", status)
	}
}

func TestIntegration_Secret_Types(t *testing.T) {
	skipIfNoServer(t)

	projectID := createProjectForTest(t)

	types := []string{"generic", "password"}
	for _, secretType := range types {
		secretType := secretType // capture loop variable
		t.Run(secretType, func(t *testing.T) {
			name := fmt.Sprintf("CLI_TEST_%s_%d", strings.ToUpper(secretType), rand.Intn(999999))

			status, result := rawPost(t, fmt.Sprintf("/api/projects/%s/secrets", projectID), map[string]interface{}{
				"name":  name,
				"value": "test-value-for-" + secretType,
				"type":  secretType,
			})
			if status != 201 {
				t.Fatalf("create %s secret returned %d; body: %v", secretType, status, result)
			}
			secret := getMap(result, "secret")
			if secret == nil {
				secret = result
			}
			secretID := getString(secret, "id")
			if secretID == "" {
				t.Fatalf("no secret ID for %s type; response: %v", secretType, result)
			}
			t.Cleanup(func() {
				rawDelete(t, fmt.Sprintf("/api/secrets/%s", secretID))
			})

			// Verify the secret type is correct
			status, result = rawGet(t, fmt.Sprintf("/api/secrets/%s", secretID))
			if status != 200 {
				t.Fatalf("get %s secret returned %d; body: %v", secretType, status, result)
			}
			got := getMap(result, "secret")
			if got == nil {
				got = result
			}
			gotType := getString(got, "type")
			if gotType != secretType {
				t.Fatalf("secret type = %q, want %q", gotType, secretType)
			}
		})
	}
}
