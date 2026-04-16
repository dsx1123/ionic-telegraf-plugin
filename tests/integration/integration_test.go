//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"golang.org/x/crypto/ssh"
)

func getSSHClient(t *testing.T) *ssh.Client {
	t.Helper()

	host := os.Getenv("INTEGRATION_HOST")
	if host == "" {
		t.Skip("INTEGRATION_HOST not set, skipping integration tests")
	}

	keyPath := os.Getenv("INTEGRATION_SSH_KEY")
	if keyPath == "" {
		keyPath = os.ExpandEnv("$HOME/.ssh/id_rsa")
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("unable to read SSH key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		t.Fatalf("unable to parse SSH key: %v", err)
	}

	user := os.Getenv("INTEGRATION_USER")
	if user == "" {
		user = "ubuntu"
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		t.Fatalf("failed to connect to %s: %v", host, err)
	}

	return client
}

func runRemote(t *testing.T, client *ssh.Client, cmd string) string {
	t.Helper()
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	if err != nil {
		t.Fatalf("command %q failed: %v\noutput: %s", cmd, err, string(out))
	}
	return string(out)
}

func TestSSHConnectivity(t *testing.T) {
	client := getSSHClient(t)
	defer client.Close()

	out := runRemote(t, client, "echo hello")
	if out != "hello\n" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestNicctlAvailable(t *testing.T) {
	client := getSSHClient(t)
	defer client.Close()

	runRemote(t, client, "which nicctl")
}

func TestPortStatisticsJSON(t *testing.T) {
	client := getSSHClient(t)
	defer client.Close()

	out := runRemote(t, client, "sudo nicctl show port statistics --json")
	var parsed interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("port statistics output is not valid JSON: %v\noutput: %s", err, out)
	}
	fmt.Printf("Port statistics keys: %v\n", jsonKeys(parsed))
}

func TestLifStatisticsJSON(t *testing.T) {
	client := getSSHClient(t)
	defer client.Close()

	out := runRemote(t, client, "sudo nicctl show lif statistics --json")
	var parsed interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("lif statistics output is not valid JSON: %v\noutput: %s", err, out)
	}
	fmt.Printf("LIF statistics keys: %v\n", jsonKeys(parsed))
}

func TestCardStatisticsJSON(t *testing.T) {
	client := getSSHClient(t)
	defer client.Close()

	out := runRemote(t, client, "sudo nicctl show card statistics packet-buffer --json")
	var parsed interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("card statistics output is not valid JSON: %v\noutput: %s", err, out)
	}
	fmt.Printf("Card statistics keys: %v\n", jsonKeys(parsed))
}

func jsonKeys(v interface{}) []string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
