// Package server - Node service
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/adapters/liquid"
	"github.com/xs-wallet/xscore/internal/adapters/lnd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/xs-wallet/xscore/internal/config"
	pb "github.com/xs-wallet/xscore/proto"
)

// =============================================================================
// NODE SERVICE
// =============================================================================

// NodeService implementa pb.NodeServiceServer
type NodeService struct {
	pb.UnimplementedNodeServiceServer
	cfg *config.Config

	mu             sync.RWMutex
	lifecycle      map[pb.NodeType]*nodeLifecycleState
	manifestIssues map[pb.NodeType]string
	stopCh         chan struct{}
	stopOnce       sync.Once
}

// NewNodeService cria NodeService
func NewNodeService(cfg *config.Config) *NodeService {
	svc := &NodeService{
		cfg: cfg,
		lifecycle: map[pb.NodeType]*nodeLifecycleState{
			pb.NodeType_NODE_TYPE_BITCOIND:  {},
			pb.NodeType_NODE_TYPE_ELEMENTSD: {},
			pb.NodeType_NODE_TYPE_LND:       {},
		},
		manifestIssues: map[pb.NodeType]string{},
		stopCh:         make(chan struct{}),
	}
	svc.loadRuntimeManifestValidation()
	go svc.supervisorLoop()
	return svc
}

// Close stops background supervisor goroutine.
func (s *NodeService) Close() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

type nodeLifecycleState struct {
	desiredRunning bool
	startedAt      time.Time
	lastStartAt    time.Time
	nextStartAfter time.Time
	restartWindow  []time.Time
	lastError      string
	lastReasonCode string
	process        *exec.Cmd
}

type nodeRuntimeManifest struct {
	Version   string                     `json:"version"`
	Network   string                     `json:"network"`
	Nodes     map[string]nodeManifestRef `json:"nodes"`
	Bitcoind  *nodeManifestRef           `json:"bitcoind,omitempty"`
	Elementsd *nodeManifestRef           `json:"elementsd,omitempty"`
	LND       *nodeManifestRef           `json:"lnd,omitempty"`
}

type nodeManifestRef struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
	DataDir string `json:"data_dir,omitempty"`
	Binary  string `json:"binary,omitempty"`
}

type nodeHealthLevel string

const (
	healthUp       nodeHealthLevel = "UP"
	healthReady    nodeHealthLevel = "READY"
	healthDegraded nodeHealthLevel = "DEGRADED"
)

const (
	reasonRPCUnavailable      = "RPC_UNAVAILABLE"
	reasonSyncing             = "SYNCING"
	reasonBindUnsafe          = "RPC_BIND_UNSAFE"
	reasonConfigDisabled      = "CONFIG_DISABLED"
	reasonSecretPermsUnsafe   = "SECRET_PERMS_UNSAFE"
	reasonSecretMissing       = "SECRET_MISSING"
	reasonStartBackoff        = "START_BACKOFF"
	reasonErrorTransient      = "ERROR_TRANSIENT"
	reasonErrorHard           = "ERROR_HARD"
	reasonProcessExited       = "PROCESS_EXITED"
	reasonBinaryNotInstalled  = "BINARY_NOT_INSTALLED"
	reasonManifestMissing     = "MANIFEST_MISSING"
	reasonManifestInvalid     = "MANIFEST_INVALID"
	reasonManifestMismatch    = "MANIFEST_MISMATCH"
	reasonUnsupportedNodeType = "UNSUPPORTED_NODE_TYPE"
)

var (
	restartWindowDuration  = 5 * time.Minute
	restartMaxAttempts     = 5
	restartBaseBackoff     = 2 * time.Second
	restartMaxBackoff      = 60 * time.Second
	startingGracePeriod    = 45 * time.Second
	supervisorTickInterval = 5 * time.Second
)

func formatReason(reason, detail string) string {
	if strings.TrimSpace(detail) == "" {
		return fmt.Sprintf("reason_code=%s", reason)
	}
	return fmt.Sprintf("reason_code=%s detail=%s", reason, detail)
}

// GetPlatformCapabilities returns desktop capabilities for current runtime.
func (s *NodeService) GetPlatformCapabilities(ctx context.Context, req *pb.GetPlatformCapabilitiesRequest) (*pb.PlatformCapabilities, error) {
	return &pb.PlatformCapabilities{
		CanSpawnNodes:       true,
		CanDownloadBinaries: false,
		HasEmbeddedNeutrino: false,
		Platform:            "desktop",
	}, nil
}

// StartNode inicia node
func (s *NodeService) StartNode(ctx context.Context, req *pb.StartNodeRequest) (*pb.NodeStatus, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}
	if issue := s.manifestIssue(req.NodeType); issue != "" {
		return nil, status.Error(codes.FailedPrecondition, issue)
	}
	nodeCfg, err := s.nodeConfig(req.NodeType)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if !nodeCfg.Enabled {
		return nil, status.Error(codes.FailedPrecondition, formatReason(reasonConfigDisabled, "node disabled in config"))
	}
	if !isSafeLocalHost(nodeCfg.Host) {
		return nil, status.Error(codes.FailedPrecondition, formatReason(reasonBindUnsafe, "host must be localhost/loopback"))
	}
	if req.NodeType == pb.NodeType_NODE_TYPE_LND {
		if err := validateSecretFile(nodeCfg.TLSCert); err != nil {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if err := validateSecretFile(nodeCfg.Macaroon); err != nil {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
	}

	if _, err := binaryName(req.NodeType); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	binPath, err := findBinaryPathForNode(req.NodeType)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, formatReason(reasonBinaryNotInstalled, err.Error()))
	}

	s.mu.Lock()
	lc := s.ensureLifecycleLocked(req.NodeType)
	lc.desiredRunning = true
	cmd, startErr := s.attemptStartLocked(req.NodeType, nodeCfg, binPath, true)
	lastReason := lc.lastReasonCode
	lastErr := lc.lastError
	s.mu.Unlock()

	if cmd != nil {
		go s.waitForNodeExit(req.NodeType, cmd)
	}
	if startErr != nil {
		if lastReason == reasonStartBackoff || lastReason == reasonErrorHard {
			return nil, status.Error(codes.FailedPrecondition, lastErr)
		}
		return nil, status.Error(codes.Unavailable, lastErr)
	}

	return s.GetNodeStatus(ctx, &pb.GetNodeStatusRequest{NodeType: req.NodeType})
}

// StopNode para node
func (s *NodeService) StopNode(ctx context.Context, req *pb.StopNodeRequest) (*pb.NodeStatus, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}
	if _, err := s.nodeConfig(req.NodeType); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	s.mu.Lock()
	lc := s.ensureLifecycleLocked(req.NodeType)
	lc.desiredRunning = false
	cmd := lc.process
	lc.process = nil
	lc.startedAt = time.Time{}
	lc.lastStartAt = time.Time{}
	lc.nextStartAfter = time.Time{}
	lc.restartWindow = nil
	lc.lastError = ""
	lc.lastReasonCode = ""
	s.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		if req.GetGraceful() {
			_ = cmd.Process.Signal(os.Interrupt)
		} else {
			_ = cmd.Process.Kill()
		}
	}

	return &pb.NodeStatus{
		NodeType: req.NodeType,
		Network:  networkToPB(s.cfg.Network),
		State:    pb.NodeState_NODE_STATE_STOPPED,
	}, nil
}

// RestartNode reinicia node
func (s *NodeService) RestartNode(ctx context.Context, req *pb.RestartNodeRequest) (*pb.NodeStatus, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}
	if _, err := s.StopNode(ctx, &pb.StopNodeRequest{NodeType: req.NodeType, Graceful: true}); err != nil {
		return nil, err
	}
	return s.StartNode(ctx, &pb.StartNodeRequest{NodeType: req.NodeType, Network: networkToPB(s.cfg.Network)})
}

// GetNodeStatus retorna status do node
func (s *NodeService) GetNodeStatus(ctx context.Context, req *pb.GetNodeStatusRequest) (*pb.NodeStatus, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}
	st, _, err := s.statusForNode(ctx, req.NodeType)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return st, nil
}

// GetAllNodeStatuses retorna status de todos os nodes
func (s *NodeService) GetAllNodeStatuses(ctx context.Context, req *pb.GetAllNodeStatusesRequest) (*pb.AllNodeStatusesResponse, error) {
	btc, _, _ := s.statusForNode(ctx, pb.NodeType_NODE_TYPE_BITCOIND)
	liq, _, _ := s.statusForNode(ctx, pb.NodeType_NODE_TYPE_ELEMENTSD)
	lndSt, _, _ := s.statusForNode(ctx, pb.NodeType_NODE_TYPE_LND)
	return &pb.AllNodeStatusesResponse{
		Bitcoind:  btc,
		Elementsd: liq,
		Lnd:       lndSt,
	}, nil
}

// GetSyncProgress retorna progresso de sync
func (s *NodeService) GetSyncProgress(ctx context.Context, req *pb.GetSyncProgressRequest) (*pb.SyncProgress, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}
	_, progress, err := s.statusForNode(ctx, req.NodeType)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return progress, nil
}

// WatchNodeStatus streaming de status updates
func (s *NodeService) WatchNodeStatus(req *pb.WatchNodeStatusRequest, stream pb.NodeService_WatchNodeStatusServer) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	type watchSnapshot struct {
		state  pb.NodeState
		reason string
	}
	last := map[pb.NodeType]watchSnapshot{}
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			types := watchedNodeTypes(req.GetNodeType())
			for _, nt := range types {
				st, _, err := s.statusForNode(stream.Context(), nt)
				if err != nil || st == nil {
					continue
				}
				prev := last[nt]
				prevState := prev.state
				if prevState == 0 {
					prevState = st.State
				}
				reasonChanged := prev.reason != st.ErrorMessage
				stateChanged := st.State != prevState
				if stateChanged || reasonChanged {
					if err := stream.Send(&pb.NodeStatusUpdate{
						NodeType:      nt,
						PreviousState: prevState,
						CurrentState:  st.State,
						Timestamp:     timestamppb.Now(),
					}); err != nil {
						return err
					}
				}
				last[nt] = watchSnapshot{state: st.State, reason: st.ErrorMessage}
			}
		}
	}
}

// WatchLogs streaming de logs
func (s *NodeService) WatchLogs(req *pb.WatchLogsRequest, stream pb.NodeService_WatchLogsServer) error {
	return status.Error(codes.Unimplemented, "WatchLogs not implemented yet")
}

// CheckBinaryStatus verifica status do binário
func (s *NodeService) CheckBinaryStatus(ctx context.Context, req *pb.CheckBinaryStatusRequest) (*pb.BinaryStatus, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "missing request")
	}
	binName, err := binaryName(req.NodeType)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	path, lookErr := findBinaryPath(binName)
	return &pb.BinaryStatus{
		NodeType:         req.NodeType,
		Installed:        lookErr == nil,
		InstalledVersion: "",
		LatestVersion:    "",
		UpdateAvailable:  false,
		Verified:         false,
		BinaryPath:       path,
	}, nil
}

// DownloadBinary faz download do binário
func (s *NodeService) DownloadBinary(req *pb.DownloadBinaryRequest, stream pb.NodeService_DownloadBinaryServer) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// VerifyBinary verifica integridade do binário
func (s *NodeService) VerifyBinary(ctx context.Context, req *pb.VerifyBinaryRequest) (*pb.VerifyBinaryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (s *NodeService) statusForNode(ctx context.Context, nodeType pb.NodeType) (*pb.NodeStatus, *pb.SyncProgress, error) {
	nodeCfg, err := s.nodeConfig(nodeType)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	s.mu.RLock()
	lc := s.lifecycle[nodeType]
	if lc == nil {
		lc = &nodeLifecycleState{}
	}
	startedAt := lc.startedAt
	lastStartAt := lc.lastStartAt
	desired := lc.desiredRunning
	lastErr := lc.lastError
	lastReason := lc.lastReasonCode
	nextStartAfter := lc.nextStartAfter
	processAlive := isProcessAlive(lc.process)
	s.mu.RUnlock()

	base := &pb.NodeStatus{
		NodeType: nodeType,
		Network:  networkToPB(s.cfg.Network),
		State:    pb.NodeState_NODE_STATE_STOPPED,
	}
	progress := &pb.SyncProgress{
		NodeType: nodeType,
		Synced:   false,
	}

	if !nodeCfg.Enabled {
		base.State = pb.NodeState_NODE_STATE_STOPPED
		base.ErrorMessage = formatReason(reasonConfigDisabled, "node disabled in config")
		return base, progress, nil
	}
	if !isSafeLocalHost(nodeCfg.Host) {
		base.State = pb.NodeState_NODE_STATE_ERROR
		base.ErrorMessage = formatReason(reasonBindUnsafe, "host must be localhost/loopback")
		return base, progress, nil
	}
	if issue := s.manifestIssue(nodeType); issue != "" {
		base.State = pb.NodeState_NODE_STATE_ERROR
		base.ErrorMessage = issue
		return base, progress, nil
	}
	if !desired {
		base.State = pb.NodeState_NODE_STATE_STOPPED
		if lastErr != "" {
			base.ErrorMessage = lastErr
		}
		return base, progress, nil
	}
	if !processAlive {
		if lastReason == reasonErrorHard && lastErr != "" {
			base.State = pb.NodeState_NODE_STATE_ERROR
			base.ErrorMessage = lastErr
			return base, progress, nil
		}
		if !nextStartAfter.IsZero() && now.Before(nextStartAfter) {
			base.State = pb.NodeState_NODE_STATE_ERROR
			base.ErrorMessage = formatReason(reasonStartBackoff, fmt.Sprintf("retry_after=%s", nextStartAfter.Sub(now).Round(time.Second)))
			return base, progress, nil
		}
		base.State = pb.NodeState_NODE_STATE_STARTING
		if lastErr != "" {
			base.State = pb.NodeState_NODE_STATE_ERROR
			base.ErrorMessage = lastErr
			return base, progress, nil
		}
		base.ErrorMessage = formatReason(string(healthUp), fmt.Sprintf("reason=%s process_not_running", lastReason))
		return base, progress, nil
	}

	switch nodeType {
	case pb.NodeType_NODE_TYPE_BITCOIND:
		return s.probeBitcoind(ctx, base, progress, now, startedAt, lastStartAt), progress, nil
	case pb.NodeType_NODE_TYPE_ELEMENTSD:
		return s.probeElements(ctx, base, progress, now, startedAt, lastStartAt), progress, nil
	case pb.NodeType_NODE_TYPE_LND:
		return s.probeLND(ctx, base, progress, now, startedAt, lastStartAt), progress, nil
	default:
		return nil, nil, fmt.Errorf("%s: %v", reasonUnsupportedNodeType, nodeType)
	}
}

func (s *NodeService) probeBitcoind(ctx context.Context, st *pb.NodeStatus, sync *pb.SyncProgress, now, startedAt, lastStartAt time.Time) *pb.NodeStatus {
	url := fmt.Sprintf("http://%s:%d", s.cfg.Bitcoind.Host, s.cfg.Bitcoind.Port)
	client := bitcoin.NewClient(url, s.cfg.Bitcoind.User, s.cfg.Bitcoind.Password)
	info, err := client.GetBlockchainInfo(ctx)
	if err != nil {
		if !lastStartAt.IsZero() && now.Sub(lastStartAt) <= startingGracePeriod {
			st.State = pb.NodeState_NODE_STATE_STARTING
			st.ErrorMessage = formatReason(string(healthUp), fmt.Sprintf("rpc warming-up: %v", err))
			return st
		}
		st.State = pb.NodeState_NODE_STATE_ERROR
		st.ErrorMessage = formatReason(reasonRPCUnavailable, err.Error())
		return st
	}
	st.Version = "bitcoind"
	sync.CurrentHeight = uint32(maxInt64(info.Blocks, 0))
	sync.TargetHeight = uint32(maxInt64(info.Headers, 0))
	sync.ProgressPercent = info.VerificationProgress * 100
	sync.Synced = !info.InitialBlockDownload && info.VerificationProgress >= 0.999
	st.PeerCount = int32(info.Connections)
	if startedAt.IsZero() {
		startedAt = now
	}
	st.StartedAt = timestamppb.New(startedAt)
	st.UptimeSeconds = int64(now.Sub(startedAt).Seconds())
	if sync.Synced {
		st.State = pb.NodeState_NODE_STATE_RUNNING
		st.ErrorMessage = formatReason(string(healthReady), "node ready for swaps")
	} else {
		st.State = pb.NodeState_NODE_STATE_SYNCING
		st.ErrorMessage = formatReason(reasonSyncing, fmt.Sprintf("health=%s progress=%.3f ibd=%t", healthDegraded, info.VerificationProgress, info.InitialBlockDownload))
	}
	return st
}

func (s *NodeService) probeElements(ctx context.Context, st *pb.NodeStatus, sync *pb.SyncProgress, now, startedAt, lastStartAt time.Time) *pb.NodeStatus {
	host := fmt.Sprintf("%s:%d", s.cfg.Elementsd.Host, s.cfg.Elementsd.Port)
	client := liquid.NewClient(liquid.Config{
		Host:     host,
		User:     s.cfg.Elementsd.User,
		Password: s.cfg.Elementsd.Password,
	})
	info, err := client.GetBlockchainInfo(ctx)
	if err != nil {
		if !lastStartAt.IsZero() && now.Sub(lastStartAt) <= startingGracePeriod {
			st.State = pb.NodeState_NODE_STATE_STARTING
			st.ErrorMessage = formatReason(string(healthUp), fmt.Sprintf("rpc warming-up: %v", err))
			return st
		}
		st.State = pb.NodeState_NODE_STATE_ERROR
		st.ErrorMessage = formatReason(reasonRPCUnavailable, err.Error())
		return st
	}
	st.Version = "elementsd"
	sync.CurrentHeight = uint32(maxInt64(info.Blocks, 0))
	sync.TargetHeight = uint32(maxInt64(info.Headers, 0))
	sync.ProgressPercent = info.Progress * 100
	sync.Synced = !info.InitialBlockDownload && info.Progress >= 0.999
	if startedAt.IsZero() {
		startedAt = now
	}
	st.StartedAt = timestamppb.New(startedAt)
	st.UptimeSeconds = int64(now.Sub(startedAt).Seconds())
	if sync.Synced {
		st.State = pb.NodeState_NODE_STATE_RUNNING
		st.ErrorMessage = formatReason(string(healthReady), "node ready for swaps")
	} else {
		st.State = pb.NodeState_NODE_STATE_SYNCING
		st.ErrorMessage = formatReason(reasonSyncing, fmt.Sprintf("health=%s progress=%.3f ibd=%t", healthDegraded, info.Progress, info.InitialBlockDownload))
	}
	return st
}

func (s *NodeService) probeLND(ctx context.Context, st *pb.NodeStatus, sync *pb.SyncProgress, now, startedAt, lastStartAt time.Time) *pb.NodeStatus {
	if err := validateSecretFile(s.cfg.LND.TLSCert); err != nil {
		st.State = pb.NodeState_NODE_STATE_ERROR
		st.ErrorMessage = err.Error()
		return st
	}
	if err := validateSecretFile(s.cfg.LND.Macaroon); err != nil {
		st.State = pb.NodeState_NODE_STATE_ERROR
		st.ErrorMessage = err.Error()
		return st
	}
	host := fmt.Sprintf("%s:%d", s.cfg.LND.Host, s.cfg.LND.Port)
	client, err := lnd.NewClient(lnd.Config{
		Host:         host,
		TLSCertPath:  s.cfg.LND.TLSCert,
		MacaroonPath: s.cfg.LND.Macaroon,
		Network:      s.cfg.Network,
	})
	if err != nil {
		if !lastStartAt.IsZero() && now.Sub(lastStartAt) <= startingGracePeriod {
			st.State = pb.NodeState_NODE_STATE_STARTING
			st.ErrorMessage = formatReason(string(healthUp), fmt.Sprintf("rpc warming-up: %v", err))
			return st
		}
		st.State = pb.NodeState_NODE_STATE_ERROR
		st.ErrorMessage = formatReason(reasonRPCUnavailable, err.Error())
		return st
	}
	defer client.Close()

	info, err := client.GetNodeInfo(ctx)
	if err != nil {
		st.State = pb.NodeState_NODE_STATE_ERROR
		st.ErrorMessage = formatReason(reasonRPCUnavailable, err.Error())
		return st
	}

	st.Version = info.Version
	st.PeerCount = int32(info.NumPeers)
	sync.CurrentHeight = uint32(maxInt64(info.BlockHeight, 0))
	sync.TargetHeight = uint32(maxInt64(info.BlockHeight, 0))
	sync.ProgressPercent = 100
	sync.Synced = info.SyncedToChain && info.SyncedToGraph
	if startedAt.IsZero() {
		startedAt = now
	}
	st.StartedAt = timestamppb.New(startedAt)
	st.UptimeSeconds = int64(now.Sub(startedAt).Seconds())
	if sync.Synced {
		st.State = pb.NodeState_NODE_STATE_RUNNING
		st.ErrorMessage = formatReason(string(healthReady), "node ready for swaps")
	} else if info.SyncedToChain || info.SyncedToGraph {
		st.State = pb.NodeState_NODE_STATE_SYNCING
		st.ErrorMessage = formatReason(reasonSyncing, fmt.Sprintf("health=%s synced_to_chain=%t synced_to_graph=%t", healthDegraded, info.SyncedToChain, info.SyncedToGraph))
	} else {
		st.State = pb.NodeState_NODE_STATE_STARTING
		st.ErrorMessage = formatReason(string(healthUp), "rpc up but not ready")
	}
	return st
}

func (s *NodeService) nodeConfig(nodeType pb.NodeType) (config.NodeConfig, error) {
	switch nodeType {
	case pb.NodeType_NODE_TYPE_BITCOIND:
		return s.cfg.Bitcoind, nil
	case pb.NodeType_NODE_TYPE_ELEMENTSD:
		return s.cfg.Elementsd, nil
	case pb.NodeType_NODE_TYPE_LND:
		return s.cfg.LND, nil
	default:
		return config.NodeConfig{}, fmt.Errorf("%s: %v", reasonUnsupportedNodeType, nodeType)
	}
}

func watchedNodeTypes(reqType pb.NodeType) []pb.NodeType {
	if reqType != pb.NodeType_NODE_TYPE_UNSPECIFIED {
		return []pb.NodeType{reqType}
	}
	return []pb.NodeType{
		pb.NodeType_NODE_TYPE_BITCOIND,
		pb.NodeType_NODE_TYPE_ELEMENTSD,
		pb.NodeType_NODE_TYPE_LND,
	}
}

func networkToPB(network string) pb.Network {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "mainnet":
		return pb.Network_NETWORK_MAINNET
	case "testnet":
		return pb.Network_NETWORK_TESTNET
	default:
		return pb.Network_NETWORK_REGTEST
	}
}

func isSafeLocalHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	if h == "" {
		return false
	}
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func validateSecretFile(path string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return fmt.Errorf("%s: empty path", reasonSecretMissing)
	}
	info, err := os.Stat(filepath.Clean(p))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: %s", reasonSecretMissing, p)
		}
		return fmt.Errorf("%s: %v", reasonSecretMissing, err)
	}
	if info.Mode()&0o077 != 0 {
		return fmt.Errorf("%s: %s mode=%o", reasonSecretPermsUnsafe, p, info.Mode().Perm())
	}
	return nil
}

func binaryName(nodeType pb.NodeType) (string, error) {
	switch nodeType {
	case pb.NodeType_NODE_TYPE_BITCOIND:
		return "bitcoind", nil
	case pb.NodeType_NODE_TYPE_ELEMENTSD:
		return "elementsd", nil
	case pb.NodeType_NODE_TYPE_LND:
		return "lnd", nil
	default:
		return "", fmt.Errorf("%s: %v", reasonUnsupportedNodeType, nodeType)
	}
}

func findBinaryPath(bin string) (string, error) {
	path, err := execLookPath(bin)
	if err != nil {
		return "", err
	}
	return path, nil
}

func findBinaryPathForNode(nodeType pb.NodeType) (string, error) {
	name, err := binaryName(nodeType)
	if err != nil {
		return "", err
	}
	return findBinaryPath(name)
}

func (s *NodeService) buildNodeCommand(nodeType pb.NodeType, cfg config.NodeConfig, binPath string) (*exec.Cmd, error) {
	override := strings.TrimSpace(nodeCommandOverride(nodeType))
	var parts []string
	if override != "" {
		parts = strings.Fields(override)
		if len(parts) == 0 {
			return nil, fmt.Errorf("invalid override command")
		}
	} else {
		parts = []string{binPath}
	}
	cmd := execCommand(parts[0], parts[1:]...)
	cmd.Dir = cfg.DataDir
	return cmd, nil
}

func nodeCommandOverride(nodeType pb.NodeType) string {
	switch nodeType {
	case pb.NodeType_NODE_TYPE_BITCOIND:
		return os.Getenv("XS_NODEMANAGER_CMD_BITCOIND")
	case pb.NodeType_NODE_TYPE_ELEMENTSD:
		return os.Getenv("XS_NODEMANAGER_CMD_ELEMENTSD")
	case pb.NodeType_NODE_TYPE_LND:
		return os.Getenv("XS_NODEMANAGER_CMD_LND")
	default:
		return ""
	}
}

func (s *NodeService) manifestIssue(nodeType pb.NodeType) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.manifestIssues[nodeType]
}

func (s *NodeService) loadRuntimeManifestValidation() {
	path := strings.TrimSpace(os.Getenv("XS_NODEMANAGER_MANIFEST_PATH"))
	if path == "" {
		path = filepath.Join(s.cfg.DataDir, "nodemanager.manifest.json")
	}
	required := envBool("XS_NODEMANAGER_MANIFEST_REQUIRED")

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) && !required {
			return
		}
		s.setManifestIssueAll(formatReason(reasonManifestMissing, fmt.Sprintf("path=%s err=%v", path, err)))
		return
	}

	var manifest nodeRuntimeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		s.setManifestIssueAll(formatReason(reasonManifestInvalid, err.Error()))
		return
	}
	s.applyManifestValidation(path, manifest)
}

func (s *NodeService) applyManifestValidation(path string, manifest nodeRuntimeManifest) {
	normalizedNetwork := strings.ToLower(strings.TrimSpace(s.cfg.Network))
	manifestNetwork := strings.ToLower(strings.TrimSpace(manifest.Network))
	if manifestNetwork != "" && manifestNetwork != normalizedNetwork {
		s.setManifestIssueAll(formatReason(
			reasonManifestMismatch,
			fmt.Sprintf("path=%s field=network expected=%s actual=%s", path, normalizedNetwork, manifestNetwork),
		))
		return
	}

	nodes := manifest.Nodes
	if len(nodes) == 0 {
		nodes = map[string]nodeManifestRef{}
		if manifest.Bitcoind != nil {
			nodes["bitcoind"] = *manifest.Bitcoind
		}
		if manifest.Elementsd != nil {
			nodes["elementsd"] = *manifest.Elementsd
		}
		if manifest.LND != nil {
			nodes["lnd"] = *manifest.LND
		}
	}

	s.validateManifestNode(path, pb.NodeType_NODE_TYPE_BITCOIND, nodes["bitcoind"], true)
	s.validateManifestNode(path, pb.NodeType_NODE_TYPE_ELEMENTSD, nodes["elementsd"], true)
	s.validateManifestNode(path, pb.NodeType_NODE_TYPE_LND, nodes["lnd"], true)
}

func (s *NodeService) validateManifestNode(path string, nodeType pb.NodeType, node nodeManifestRef, present bool) {
	if !present {
		return
	}
	cfg, err := s.nodeConfig(nodeType)
	if err != nil {
		s.setManifestIssue(nodeType, formatReason(reasonManifestInvalid, err.Error()))
		return
	}
	if node.Enabled != nil && cfg.Enabled != *node.Enabled {
		s.setManifestIssue(nodeType, formatReason(
			reasonManifestMismatch,
			fmt.Sprintf("path=%s field=enabled expected=%t actual=%t", path, *node.Enabled, cfg.Enabled),
		))
		return
	}
	if node.Host != "" && !strings.EqualFold(strings.TrimSpace(node.Host), strings.TrimSpace(cfg.Host)) {
		s.setManifestIssue(nodeType, formatReason(
			reasonManifestMismatch,
			fmt.Sprintf("path=%s field=host expected=%s actual=%s", path, strings.TrimSpace(node.Host), strings.TrimSpace(cfg.Host)),
		))
		return
	}
	if node.Port != 0 && node.Port != cfg.Port {
		s.setManifestIssue(nodeType, formatReason(
			reasonManifestMismatch,
			fmt.Sprintf("path=%s field=port expected=%d actual=%d", path, node.Port, cfg.Port),
		))
		return
	}
	if strings.TrimSpace(node.DataDir) != "" && filepath.Clean(node.DataDir) != filepath.Clean(cfg.DataDir) {
		s.setManifestIssue(nodeType, formatReason(
			reasonManifestMismatch,
			fmt.Sprintf("path=%s field=data_dir expected=%s actual=%s", path, filepath.Clean(node.DataDir), filepath.Clean(cfg.DataDir)),
		))
		return
	}
	if strings.TrimSpace(node.Binary) != "" {
		expected, err := binaryName(nodeType)
		if err != nil {
			s.setManifestIssue(nodeType, formatReason(reasonManifestInvalid, err.Error()))
			return
		}
		if !strings.EqualFold(strings.TrimSpace(node.Binary), expected) {
			s.setManifestIssue(nodeType, formatReason(
				reasonManifestMismatch,
				fmt.Sprintf("path=%s field=binary expected=%s actual=%s", path, strings.TrimSpace(node.Binary), expected),
			))
			return
		}
	}
	s.setManifestIssue(nodeType, "")
}

func (s *NodeService) setManifestIssue(nodeType pb.NodeType, issue string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(issue) == "" {
		delete(s.manifestIssues, nodeType)
		return
	}
	s.manifestIssues[nodeType] = issue
}

func (s *NodeService) setManifestIssueAll(issue string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	types := []pb.NodeType{
		pb.NodeType_NODE_TYPE_BITCOIND,
		pb.NodeType_NODE_TYPE_ELEMENTSD,
		pb.NodeType_NODE_TYPE_LND,
	}
	for _, nt := range types {
		if strings.TrimSpace(issue) == "" {
			delete(s.manifestIssues, nt)
			continue
		}
		s.manifestIssues[nt] = issue
	}
}

func envBool(key string) bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

func (s *NodeService) supervisorLoop() {
	ticker := time.NewTicker(supervisorTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.superviseOnce()
		}
	}
}

func (s *NodeService) superviseOnce() {
	nodeTypes := []pb.NodeType{
		pb.NodeType_NODE_TYPE_BITCOIND,
		pb.NodeType_NODE_TYPE_ELEMENTSD,
		pb.NodeType_NODE_TYPE_LND,
	}
	for _, nodeType := range nodeTypes {
		nodeCfg, err := s.nodeConfig(nodeType)
		if err != nil {
			continue
		}

		s.mu.RLock()
		lc := s.lifecycle[nodeType]
		desired := lc != nil && lc.desiredRunning
		alive := lc != nil && isProcessAlive(lc.process)
		s.mu.RUnlock()
		if !desired || alive {
			continue
		}
		if !nodeCfg.Enabled || !isSafeLocalHost(nodeCfg.Host) {
			continue
		}
		if s.manifestIssue(nodeType) != "" {
			continue
		}

		binPath, binErr := findBinaryPathForNode(nodeType)
		if binErr != nil {
			s.mu.Lock()
			lc = s.ensureLifecycleLocked(nodeType)
			lc.lastReasonCode = reasonErrorHard
			lc.lastError = formatReason(reasonBinaryNotInstalled, binErr.Error())
			s.mu.Unlock()
			continue
		}

		s.mu.Lock()
		lc = s.ensureLifecycleLocked(nodeType)
		lc.desiredRunning = true
		cmd, startErr := s.attemptStartLocked(nodeType, nodeCfg, binPath, false)
		s.mu.Unlock()
		if startErr != nil {
			continue
		}
		if cmd != nil {
			go s.waitForNodeExit(nodeType, cmd)
		}
	}
}

func (s *NodeService) ensureLifecycleLocked(nodeType pb.NodeType) *nodeLifecycleState {
	lc := s.lifecycle[nodeType]
	if lc == nil {
		lc = &nodeLifecycleState{}
		s.lifecycle[nodeType] = lc
	}
	return lc
}

func (s *NodeService) attemptStartLocked(nodeType pb.NodeType, nodeCfg config.NodeConfig, binPath string, blockOnBackoff bool) (*exec.Cmd, error) {
	lc := s.ensureLifecycleLocked(nodeType)
	if isProcessAlive(lc.process) {
		return nil, nil
	}
	now := time.Now().UTC()
	lc.restartWindow = pruneRestartWindow(lc.restartWindow, now)
	if lc.lastReasonCode == reasonErrorHard {
		if lc.lastError == "" {
			lc.lastError = formatReason(reasonErrorHard, "manual reset required")
		}
		return nil, errors.New(lc.lastError)
	}
	if len(lc.restartWindow) >= restartMaxAttempts {
		lc.lastReasonCode = reasonErrorHard
		lc.lastError = formatReason(reasonErrorHard, "restart attempts exceeded in window")
		return nil, errors.New(lc.lastError)
	}
	if !lc.nextStartAfter.IsZero() && now.Before(lc.nextStartAfter) {
		if blockOnBackoff {
			wait := lc.nextStartAfter.Sub(now).Round(time.Second)
			lc.lastReasonCode = reasonStartBackoff
			lc.lastError = formatReason(reasonStartBackoff, fmt.Sprintf("wait=%s", wait))
			return nil, errors.New(lc.lastError)
		}
		return nil, fmt.Errorf("backoff pending")
	}

	cmd, cmdErr := s.buildNodeCommand(nodeType, nodeCfg, binPath)
	if cmdErr != nil {
		lc.lastReasonCode = reasonErrorHard
		lc.lastError = formatReason(reasonErrorHard, cmdErr.Error())
		return nil, cmdErr
	}
	if startErr := cmd.Start(); startErr != nil {
		lc.restartWindow = append(lc.restartWindow, now)
		lc.lastReasonCode = reasonErrorTransient
		lc.lastError = formatReason(reasonErrorTransient, fmt.Sprintf("start failed: %v", startErr))
		lc.nextStartAfter = now.Add(computeBackoff(len(lc.restartWindow)))
		return nil, startErr
	}
	lc.process = cmd
	lc.startedAt = now
	lc.lastStartAt = now
	lc.lastReasonCode = ""
	lc.lastError = ""
	lc.nextStartAfter = time.Time{}
	return cmd, nil
}

func (s *NodeService) waitForNodeExit(nodeType pb.NodeType, cmd *exec.Cmd) {
	err := cmd.Wait()
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	lc := s.lifecycle[nodeType]
	if lc == nil {
		return
	}
	if lc.process == cmd {
		lc.process = nil
		lc.startedAt = time.Time{}
	}
	if lc.desiredRunning {
		lc.restartWindow = pruneRestartWindow(lc.restartWindow, now)
		lc.restartWindow = append(lc.restartWindow, now)
		if len(lc.restartWindow) >= restartMaxAttempts {
			lc.lastReasonCode = reasonErrorHard
			lc.lastError = formatReason(reasonErrorHard, "restart attempts exceeded in window")
			return
		}
		lc.lastReasonCode = reasonErrorTransient
		if err != nil {
			lc.lastError = formatReason(reasonProcessExited, err.Error())
		} else {
			lc.lastError = formatReason(reasonProcessExited, "process exited")
		}
		lc.nextStartAfter = now.Add(computeBackoff(len(lc.restartWindow)))
	}
}

func pruneRestartWindow(attempts []time.Time, now time.Time) []time.Time {
	if len(attempts) == 0 {
		return attempts
	}
	cut := now.Add(-restartWindowDuration)
	out := attempts[:0]
	for _, t := range attempts {
		if t.After(cut) {
			out = append(out, t)
		}
	}
	return out
}

func computeBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return restartBaseBackoff
	}
	backoff := restartBaseBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= restartMaxBackoff {
			return restartMaxBackoff
		}
	}
	if backoff > restartMaxBackoff {
		return restartMaxBackoff
	}
	return backoff
}

func isProcessAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return false
	}
	return true
}

var execLookPath = func(file string) (string, error) {
	return exec.LookPath(file)
}

var execCommand = func(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

func maxInt64(v int64, min int64) int64 {
	if v < min {
		return min
	}
	return v
}
