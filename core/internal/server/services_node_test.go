package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xs-wallet/xscore/internal/config"
	pb "github.com/xs-wallet/xscore/proto"
)

func testNodeServiceConfig() *config.Config {
	return &config.Config{
		Network: "regtest",
		Bitcoind: config.NodeConfig{
			Enabled:  true,
			Host:     "127.0.0.1",
			Port:     18443,
			User:     "rpcuser",
			Password: "rpcpass",
		},
		Elementsd: config.NodeConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    7041,
		},
		LND: config.NodeConfig{
			Enabled:  true,
			Host:     "127.0.0.1",
			Port:     10009,
			TLSCert:  "/tmp/missing.cert",
			Macaroon: "/tmp/missing.macaroon",
		},
	}
}

func TestNodeService_GetPlatformCapabilities(t *testing.T) {
	svc := NewNodeService(testNodeServiceConfig())
	t.Cleanup(svc.Close)
	got, err := svc.GetPlatformCapabilities(context.Background(), &pb.GetPlatformCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("GetPlatformCapabilities: %v", err)
	}
	if !got.CanSpawnNodes {
		t.Fatalf("expected can_spawn_nodes=true")
	}
	if got.Platform == "" {
		t.Fatalf("expected non-empty platform")
	}
}

func TestNodeService_StartNodeRejectsUnsafeHost(t *testing.T) {
	cfg := testNodeServiceConfig()
	cfg.Bitcoind.Host = "0.0.0.0"
	svc := NewNodeService(cfg)
	t.Cleanup(svc.Close)

	_, err := svc.StartNode(context.Background(), &pb.StartNodeRequest{
		NodeType: pb.NodeType_NODE_TYPE_BITCOIND,
		Network:  pb.Network_NETWORK_REGTEST,
	})
	if err == nil {
		t.Fatalf("expected unsafe host error")
	}
	if !strings.Contains(err.Error(), reasonBindUnsafe) {
		t.Fatalf("expected reason code %s, got: %v", reasonBindUnsafe, err)
	}
}

func TestNodeService_StartNodeStartsManagedProcess(t *testing.T) {
	cfg := testNodeServiceConfig()
	svc := NewNodeService(cfg)
	t.Cleanup(svc.Close)

	prevLookPath := execLookPath
	prevExecCommand := execCommand
	execLookPath = func(file string) (string, error) { return "/bin/sh", nil }
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Keep process alive briefly; we only validate lifecycle wiring.
		return exec.Command("sh", "-c", "sleep 3")
	}
	t.Cleanup(func() {
		execLookPath = prevLookPath
		execCommand = prevExecCommand
	})

	st, err := svc.StartNode(context.Background(), &pb.StartNodeRequest{
		NodeType: pb.NodeType_NODE_TYPE_BITCOIND,
		Network:  pb.Network_NETWORK_REGTEST,
	})
	if err != nil {
		t.Fatalf("StartNode: %v", err)
	}
	if st.State != pb.NodeState_NODE_STATE_STARTING && st.State != pb.NodeState_NODE_STATE_SYNCING && st.State != pb.NodeState_NODE_STATE_RUNNING {
		t.Fatalf("unexpected state after start: %v", st.State)
	}

	_, _ = svc.StopNode(context.Background(), &pb.StopNodeRequest{
		NodeType: pb.NodeType_NODE_TYPE_BITCOIND,
		Graceful: false,
	})
}

func TestNodeService_StartNodeBackoffAfterExit(t *testing.T) {
	cfg := testNodeServiceConfig()
	svc := NewNodeService(cfg)
	t.Cleanup(svc.Close)

	prevLookPath := execLookPath
	prevExecCommand := execCommand
	execLookPath = func(file string) (string, error) { return "/bin/sh", nil }
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Exit immediately to trigger restart window/backoff logic.
		return exec.Command("sh", "-c", "exit 0")
	}
	t.Cleanup(func() {
		execLookPath = prevLookPath
		execCommand = prevExecCommand
	})

	if _, err := svc.StartNode(context.Background(), &pb.StartNodeRequest{
		NodeType: pb.NodeType_NODE_TYPE_BITCOIND,
		Network:  pb.Network_NETWORK_REGTEST,
	}); err != nil {
		t.Fatalf("first StartNode should pass: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	_, err := svc.StartNode(context.Background(), &pb.StartNodeRequest{
		NodeType: pb.NodeType_NODE_TYPE_BITCOIND,
		Network:  pb.Network_NETWORK_REGTEST,
	})
	if err == nil {
		t.Fatalf("expected backoff error on immediate second start")
	}
	if !strings.Contains(err.Error(), reasonStartBackoff) {
		t.Fatalf("expected reason %s, got: %v", reasonStartBackoff, err)
	}
}

func TestNodeService_StopNodeSetsStopped(t *testing.T) {
	cfg := testNodeServiceConfig()
	cfg.Bitcoind.Enabled = false
	svc := NewNodeService(cfg)
	t.Cleanup(svc.Close)

	st, err := svc.StopNode(context.Background(), &pb.StopNodeRequest{
		NodeType: pb.NodeType_NODE_TYPE_BITCOIND,
		Graceful: true,
	})
	if err != nil {
		t.Fatalf("StopNode: %v", err)
	}
	if st.State != pb.NodeState_NODE_STATE_STOPPED {
		t.Fatalf("expected stopped, got %v", st.State)
	}
}

func TestValidateSecretFileRejectsInsecurePerms(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "admin.macaroon")
	if err := os.WriteFile(p, []byte("test"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	err := validateSecretFile(p)
	if err == nil {
		t.Fatalf("expected permission validation error")
	}
	if !strings.Contains(err.Error(), reasonSecretPermsUnsafe) {
		t.Fatalf("expected reason code %s, got: %v", reasonSecretPermsUnsafe, err)
	}
}

func TestNodeService_SuperviseOnceRestartsDesiredProcess(t *testing.T) {
	cfg := testNodeServiceConfig()
	svc := NewNodeService(cfg)
	t.Cleanup(svc.Close)

	prevLookPath := execLookPath
	prevExecCommand := execCommand
	execLookPath = func(file string) (string, error) { return "/bin/sh", nil }
	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "sleep 2")
	}
	t.Cleanup(func() {
		execLookPath = prevLookPath
		execCommand = prevExecCommand
	})

	svc.mu.Lock()
	lc := svc.ensureLifecycleLocked(pb.NodeType_NODE_TYPE_BITCOIND)
	lc.desiredRunning = true
	lc.process = nil
	svc.mu.Unlock()

	svc.superviseOnce()
	t.Cleanup(func() {
		_, _ = svc.StopNode(context.Background(), &pb.StopNodeRequest{
			NodeType: pb.NodeType_NODE_TYPE_BITCOIND,
			Graceful: false,
		})
	})

	svc.mu.RLock()
	defer svc.mu.RUnlock()
	lc = svc.lifecycle[pb.NodeType_NODE_TYPE_BITCOIND]
	if lc == nil || !isProcessAlive(lc.process) {
		t.Fatalf("expected supervised process to be running")
	}
}

func TestNodeService_StartNodeBlockedByManifestMismatch(t *testing.T) {
	dir := t.TempDir()
	cfg := testNodeServiceConfig()
	cfg.DataDir = dir

	manifestPath := filepath.Join(dir, "nodemanager.manifest.json")
	manifest := `{"network":"regtest","nodes":{"bitcoind":{"host":"127.0.0.2","port":18443}}}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	svc := NewNodeService(cfg)
	t.Cleanup(svc.Close)

	_, err := svc.StartNode(context.Background(), &pb.StartNodeRequest{
		NodeType: pb.NodeType_NODE_TYPE_BITCOIND,
		Network:  pb.Network_NETWORK_REGTEST,
	})
	if err == nil {
		t.Fatalf("expected manifest mismatch to block start")
	}
	if !strings.Contains(err.Error(), reasonManifestMismatch) {
		t.Fatalf("expected reason %s, got: %v", reasonManifestMismatch, err)
	}
}

func TestNodeService_ManifestRequiredMissingSetsErrorState(t *testing.T) {
	dir := t.TempDir()
	cfg := testNodeServiceConfig()
	cfg.DataDir = dir
	t.Setenv("XS_NODEMANAGER_MANIFEST_REQUIRED", "1")

	svc := NewNodeService(cfg)
	t.Cleanup(svc.Close)

	st, err := svc.GetNodeStatus(context.Background(), &pb.GetNodeStatusRequest{
		NodeType: pb.NodeType_NODE_TYPE_BITCOIND,
	})
	if err != nil {
		t.Fatalf("GetNodeStatus: %v", err)
	}
	if st.State != pb.NodeState_NODE_STATE_ERROR {
		t.Fatalf("expected error state, got: %v", st.State)
	}
	if !strings.Contains(st.ErrorMessage, reasonManifestMissing) {
		t.Fatalf("expected reason %s, got: %s", reasonManifestMissing, st.ErrorMessage)
	}
}
