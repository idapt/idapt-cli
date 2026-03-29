package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSecretList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"GET /api/secrets": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"secrets": []map[string]interface{}{
					{"id": "sec-1", "name": "DB_PASSWORD", "type": "generic"},
					{"id": "sec-2", "name": "SSH_KEY", "type": "ssh_private_key"},
				},
			})
		},
	})
	stdout, _, err := runCmd(t, h, "secret", "list", "--project", "myproj", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if len(got) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(got))
	}
}

func TestSecretCreate(t *testing.T) {
	t.Run("generic", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
				})
			},
			"POST /api/secrets": func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["name"] != "API_TOKEN" {
					t.Errorf("expected name=API_TOKEN, got %v", body["name"])
				}
				if body["value"] != "secret123" {
					t.Errorf("expected value=secret123, got %v", body["value"])
				}
				if body["type"] != "generic" {
					t.Errorf("expected type=generic, got %v", body["type"])
				}
				jsonResponse(w, 201, map[string]interface{}{"id": "sec-3", "name": "API_TOKEN", "type": "generic"})
			},
		})
		_, _, err := runCmd(t, h, "secret", "create", "--project", "myproj", "--name", "API_TOKEN", "--value", "secret123", "--type", "generic")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ssh_key_type", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
				})
			},
			"POST /api/secrets": func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["type"] != "ssh_private_key" {
					t.Errorf("expected type=ssh_private_key, got %v", body["type"])
				}
				jsonResponse(w, 201, map[string]interface{}{"id": "sec-4", "name": "my-key", "type": "ssh_private_key"})
			},
		})
		_, _, err := runCmd(t, h, "secret", "create", "--project", "myproj", "--name", "my-key", "--value", "-----BEGIN PRIVATE KEY-----", "--type", "ssh_private_key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("password_type", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, map[string]interface{}{
					"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
				})
			},
			"POST /api/secrets": func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["type"] != "password" {
					t.Errorf("expected type=password, got %v", body["type"])
				}
				jsonResponse(w, 201, map[string]interface{}{"id": "sec-5", "name": "db-pass", "type": "password"})
			},
		})
		_, _, err := runCmd(t, h, "secret", "create", "--project", "myproj", "--name", "db-pass", "--value", "pw123", "--type", "password")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSecretGet(t *testing.T) {
	secretID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"GET /api/secrets/" + secretID: func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id": secretID, "name": "API_TOKEN", "type": "generic",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "secret", "get", secretID, "--project", "myproj", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["name"] != "API_TOKEN" {
		t.Errorf("expected name=API_TOKEN, got %v", got["name"])
	}
}

func TestSecretEdit(t *testing.T) {
	secretID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"PATCH /api/secrets/" + secretID: func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["value"] != "new-value" {
				t.Errorf("expected value=new-value, got %v", body["value"])
			}
			jsonResponse(w, 200, map[string]interface{}{"id": secretID})
		},
	})
	stdout, _, err := runCmd(t, h, "secret", "edit", secretID, "--project", "myproj", "--value", "new-value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "updated") {
		t.Errorf("expected 'updated' message, got: %s", stdout)
	}
}

func TestSecretDelete(t *testing.T) {
	secretID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"DELETE /api/secrets/" + secretID: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})
	stdout, _, err := runCmd(t, h, "secret", "delete", secretID, "--project", "myproj", "--confirm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected 'deleted' message, got: %s", stdout)
	}
}

func TestSecretGenerate(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/secrets": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["generate"] != true {
				t.Errorf("expected generate=true, got %v", body["generate"])
			}
			if body["name"] != "random-secret" {
				t.Errorf("expected name=random-secret, got %v", body["name"])
			}
			jsonResponse(w, 201, map[string]interface{}{
				"id": "sec-gen", "name": "random-secret", "value": "abc123xyz",
			})
		},
	})
	_, _, err := runCmd(t, h, "secret", "generate", "--project", "myproj", "--name", "random-secret", "--length", "64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSecretGenerateKeypair(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/projects/proj-1/secrets/ssh-keypair": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 201, map[string]interface{}{
				"privateKeyId": "pk-1", "publicKeyId": "pub-1", "publicKey": "ssh-ed25519 AAAA...",
			})
		},
	})
	_, _, err := runCmd(t, h, "secret", "generate-keypair", "--project", "myproj", "--name", "deploy-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSecretMount(t *testing.T) {
	machineID := "22222222-2222-2222-2222-222222222222"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/secrets/sec-1/mounts": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["machineId"] != machineID {
				t.Errorf("expected machineId=%s, got %v", machineID, body["machineId"])
			}
			if body["path"] != "/etc/secrets/token" {
				t.Errorf("expected path=/etc/secrets/token, got %v", body["path"])
			}
			jsonResponse(w, 201, map[string]interface{}{"id": "mount-1"})
		},
	})
	stdout, _, err := runCmd(t, h, "secret", "mount", "sec-1", machineID, "/etc/secrets/token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "mounted") {
		t.Errorf("expected 'mounted' message, got: %s", stdout)
	}
}

func TestSecretUnmount(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"DELETE /api/secrets/sec-1/mounts/mount-1": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})
	stdout, _, err := runCmd(t, h, "secret", "unmount", "sec-1", "mount-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "unmounted") {
		t.Errorf("expected 'unmounted' message, got: %s", stdout)
	}
}

func TestSecretMounts(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/secrets/sec-1/mounts": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"mounts": []map[string]interface{}{
					{"id": "mount-1", "machineId": "m1", "path": "/etc/secrets/token"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "secret", "mounts", "sec-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
