package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestMachineList(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/machines": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"machines": []map[string]interface{}{
					{"id": "m1", "name": "dev-box", "state": "running", "instanceType": "t3.micro"},
					{"id": "m2", "name": "staging", "state": "hibernated", "instanceType": "t3.small"},
				},
			})
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "list", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 machines, got %d", len(got))
	}
}

func TestMachineCreate(t *testing.T) {
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/projects": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"projects": []map[string]interface{}{{"id": "proj-1", "slug": "myproj"}},
			})
		},
		"POST /api/machines": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "my-machine" {
				t.Errorf("expected name=my-machine, got %v", body["name"])
			}
			if body["instanceType"] != "t3.micro" {
				t.Errorf("expected instanceType=t3.micro, got %v", body["instanceType"])
			}
			jsonResponse(w, 201, map[string]interface{}{
				"id": "m3", "name": "my-machine", "state": "provisioning",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "create", "--project", "myproj", "--name", "my-machine", "--instance-type", "t3.micro", "--storage", "20", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["state"] != "provisioning" {
		t.Errorf("expected state=provisioning, got %v", got["state"])
	}
}

func TestMachineGet(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/machines/" + machineID: func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"id": machineID, "name": "dev-box", "state": "running",
				"instanceType": "t3.micro", "rootVolumeSizeGb": 20,
				"publicIp": "1.2.3.4", "region": "us-east-1",
			})
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "get", machineID, "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if got["publicIp"] != "1.2.3.4" {
		t.Errorf("expected publicIp=1.2.3.4, got %v", got["publicIp"])
	}
}

func TestMachineEdit(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"PATCH /api/machines/" + machineID: func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "renamed-box" {
				t.Errorf("expected name=renamed-box, got %v", body["name"])
			}
			jsonResponse(w, 200, map[string]interface{}{
				"id": machineID, "name": "renamed-box", "state": "running",
			})
		},
	})
	_, _, err := runCmd(t, h, "machine", "edit", machineID, "--name", "renamed-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMachineStart(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/machines/" + machineID + "/action": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["action"] != "wake" {
				t.Errorf("expected action=wake, got %v", body["action"])
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "start", machineID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "starting") {
		t.Errorf("expected 'starting' in output, got: %s", stdout)
	}
}

func TestMachineStop(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/machines/" + machineID + "/action": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["action"] != "hibernate" {
				t.Errorf("expected action=hibernate, got %v", body["action"])
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "stop", machineID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "stopping") {
		t.Errorf("expected 'stopping' in output, got: %s", stdout)
	}
}

func TestMachineTerminate(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	t.Run("with_confirm", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
			"POST /api/machines/" + machineID + "/action": func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				if body["action"] != "terminate" {
					t.Errorf("expected action=terminate, got %v", body["action"])
				}
				w.WriteHeader(200)
			},
		})
		stdout, _, err := runCmd(t, h, "machine", "terminate", machineID, "--confirm")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout, "terminating") {
			t.Errorf("expected 'terminating' in output, got: %s", stdout)
		}
	})

	t.Run("without_confirm_aborts", func(t *testing.T) {
		h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){})
		_, _, err := runCmd(t, h, "machine", "terminate", machineID)
		if err == nil {
			t.Fatal("expected abort error without --confirm")
		}
		if !strings.Contains(err.Error(), "aborted") {
			t.Errorf("expected 'aborted' error, got: %v", err)
		}
	})
}

func TestMachineExec(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/machines/" + machineID + "/terminal": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["command"] != "ls -la" {
				t.Errorf("expected command='ls -la', got %v", body["command"])
			}
			jsonResponse(w, 200, map[string]interface{}{"output": "total 0\n", "exitCode": 0})
		},
	})
	// Use -- to prevent cobra from parsing command args as flags
	stdout, _, err := runCmd(t, h, "machine", "exec", machineID, "--", "ls", "-la")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "total 0") {
		t.Errorf("expected command output, got: %s", stdout)
	}
}

func TestMachineTmuxList(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/machines/" + machineID + "/terminal": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{"output": "main\t3\t1700000000"})
		},
	})
	_, _, err := runCmd(t, h, "machine", "tmux", "list", machineID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMachineTmuxRun(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/machines/" + machineID + "/terminal": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			cmd := body["command"].(string)
			if !strings.Contains(cmd, "tmux new-session") {
				t.Errorf("expected tmux new-session command, got: %s", cmd)
			}
			if !strings.Contains(cmd, "mysession") {
				t.Errorf("expected session name 'mysession' in command, got: %s", cmd)
			}
			w.WriteHeader(200)
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "tmux", "run", machineID, "mysession", "npm start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "started") {
		t.Errorf("expected 'started' message, got: %s", stdout)
	}
}

func TestMachineTmuxCapture(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/machines/" + machineID + "/terminal": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{"output": "captured output line 1\nline 2\n"})
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "tmux", "capture", machineID, "mysession")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "captured output") {
		t.Errorf("expected captured output, got: %s", stdout)
	}
}

func TestMachineFileList(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/machines/" + machineID + "/sftp": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("action") != "list" {
				t.Errorf("expected action=list, got %v", r.URL.Query().Get("action"))
			}
			jsonResponse(w, 200, map[string]interface{}{
				"files": []map[string]interface{}{
					{"name": "readme.md", "type": "file", "size": 1024},
					{"name": "src", "type": "directory", "size": 4096},
				},
			})
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "file", "list", machineID, "/home/user", "-o", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []map[string]interface{}
	json.Unmarshal([]byte(stdout), &got)
	if len(got) != 2 {
		t.Errorf("expected 2 files, got %d", len(got))
	}
}

func TestMachineFileWrite(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/machines/" + machineID + "/sftp": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["action"] != "write" {
				t.Errorf("expected action=write, got %v", body["action"])
			}
			if body["path"] != "/tmp/test.txt" {
				t.Errorf("expected path=/tmp/test.txt, got %v", body["path"])
			}
			if body["content"] != "hello world" {
				t.Errorf("expected content='hello world', got %v", body["content"])
			}
			w.WriteHeader(200)
		},
	})
	// Create a custom env with stdin content, since the write command reads from f.In
	env := newTestEnv(t, h)
	env.factory.In = strings.NewReader("hello world")
	stdout, _, err := runCmdWithEnv(t, env, "machine", "file", "write", machineID, "/tmp/test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "File written") {
		t.Errorf("expected 'File written' message, got: %s", stdout)
	}
}

func TestMachineFwList(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/machines/" + machineID + "/firewall": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"rules": []map[string]interface{}{
					{"id": "r1", "port": 22, "protocol": "tcp", "source": "0.0.0.0/0"},
					{"id": "r2", "port": 443, "protocol": "tcp", "source": "0.0.0.0/0"},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "machine", "firewall", "list", machineID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMachineFwAdd(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/machines/" + machineID + "/firewall": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["port"].(float64) != 8080 {
				t.Errorf("expected port=8080, got %v", body["port"])
			}
			if body["protocol"] != "tcp" {
				t.Errorf("expected protocol=tcp, got %v", body["protocol"])
			}
			jsonResponse(w, 201, map[string]interface{}{"id": "r3"})
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "firewall", "add", machineID, "--port", "8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "8080") {
		t.Errorf("expected port in output, got: %s", stdout)
	}
}

func TestMachineFwRemove(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"DELETE /api/machines/" + machineID + "/firewall/r1": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "firewall", "remove", machineID, "r1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "removed") {
		t.Errorf("expected 'removed' message, got: %s", stdout)
	}
}

func TestMachineUserList(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"GET /api/machines/" + machineID + "/users": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"users": []map[string]interface{}{
					{"username": "admin", "shell": "/bin/bash", "home": "/home/admin", "sudo": true},
				},
			})
		},
	})
	_, _, err := runCmd(t, h, "machine", "user", "list", machineID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMachineUserCreate(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"POST /api/machines/" + machineID + "/users": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["username"] != "deploy" {
				t.Errorf("expected username=deploy, got %v", body["username"])
			}
			jsonResponse(w, 201, map[string]interface{}{"username": "deploy"})
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "user", "create", machineID, "--username", "deploy", "--password", "secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "User created") {
		t.Errorf("expected 'User created' message, got: %s", stdout)
	}
}

func TestMachineUserDelete(t *testing.T) {
	machineID := "11111111-1111-1111-1111-111111111111"
	h := mockHandler(map[string]func(w http.ResponseWriter, r *http.Request){
		"DELETE /api/machines/" + machineID + "/users/deploy": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})
	stdout, _, err := runCmd(t, h, "machine", "user", "delete", machineID, "deploy", "--confirm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected 'deleted' message, got: %s", stdout)
	}
}
