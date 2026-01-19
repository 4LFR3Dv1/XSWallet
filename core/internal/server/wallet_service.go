// Package server - Updated WalletService with vault integration
package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/config"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/vault"
	pb "github.com/xs-wallet/xscore/proto"
)

// WalletService implements pb.WalletServiceServer
type WalletService struct {
	pb.UnimplementedWalletServiceServer
	db     *db.DB
	cfg    *config.Config
	vault  *vault.Vault
	btc    *bitcoin.Client
}

// NewWalletService creates WalletService
func NewWalletService(database *db.DB, cfg *config.Config, v *vault.Vault, btcClient *bitcoin.Client) *WalletService {
	return &WalletService{
		db:    database,
		cfg:   cfg,
		vault: v,
		btc:   btcClient,
	}
}

// InitializeVault initializes the vault with a new or imported seed
func (s *WalletService) InitializeVault(ctx context.Context, req *pb.InitializeVaultRequest) (*pb.InitializeVaultResponse, error) {
	var mnemonic string
	var err error

	// Check if generating or importing
	if gen := req.GetGenerate(); gen != nil {
		// Generate new mnemonic
		mnemonic, err = s.vault.Initialize("", "", req.Pin)
	} else if imp := req.GetImport(); imp != nil {
		// Import existing mnemonic
		mnemonic, err = s.vault.Initialize(imp.Mnemonic, imp.Passphrase, req.Pin)
	} else {
		return nil, status.Error(codes.InvalidArgument, "must specify generate or import")
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initialize vault: %v", err)
	}

	// Unlock immediately after initialization
	sessionID, err := s.vault.Unlock(req.Pin)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unlock vault: %v", err)
	}

	return &pb.InitializeVaultResponse{
		Success:   true,
		Mnemonic:  mnemonic,
		SessionId: sessionID,
	}, nil
}

// UnlockVault unlocks the vault with a PIN
func (s *WalletService) UnlockVault(ctx context.Context, req *pb.UnlockVaultRequest) (*pb.UnlockVaultResponse, error) {
	sessionID, err := s.vault.Unlock(req.Pin)
	if err != nil {
		if err == vault.ErrInvalidPIN {
			return &pb.UnlockVaultResponse{
				Success:      false,
				ErrorMessage: "Invalid PIN",
			}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to unlock vault: %v", err)
	}

	return &pb.UnlockVaultResponse{
		Success:   true,
		SessionId: sessionID,
	}, nil
}

// LockVault locks the vault
func (s *WalletService) LockVault(ctx context.Context, req *pb.LockVaultRequest) (*pb.LockVaultResponse, error) {
	if err := s.vault.Lock(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lock vault: %v", err)
	}

	return &pb.LockVaultResponse{
		Success: true,
	}, nil
}

// GetVaultStatus returns the vault status
func (s *WalletService) GetVaultStatus(ctx context.Context, req *pb.GetVaultStatusRequest) (*pb.VaultStatus, error) {
	vaultStatus := s.vault.Status()

	var state pb.VaultStatus_State
	switch vaultStatus {
	case vault.StatusNotInitialized:
		state = pb.VaultStatus_STATE_NOT_INITIALIZED
	case vault.StatusLocked:
		state = pb.VaultStatus_STATE_LOCKED
	case vault.StatusUnlocked:
		state = pb.VaultStatus_STATE_UNLOCKED
	case vault.StatusLockedOut:
		state = pb.VaultStatus_STATE_LOCKED_OUT
	default:
		state = pb.VaultStatus_STATE_UNSPECIFIED
	}

	return &pb.VaultStatus{
		State: state,
	}, nil
}

// GetBackupStatus returns backup status
func (s *WalletService) GetBackupStatus(ctx context.Context, req *pb.GetBackupStatusRequest) (*pb.BackupStatus, error) {
	// For MVP Sprint 1 (Submarine only), no critical backup requirements
	return &pb.BackupStatus{
		HasPendingSwapsRequiringBackup: false,
		PendingSwapCount:               0,
		BackupRecommended:              false,
	}, nil
}

// MarkBackupComplete marks a backup as complete
func (s *WalletService) MarkBackupComplete(ctx context.Context, req *pb.MarkBackupCompleteRequest) (*pb.BackupStatus, error) {
	// TODO: Implement backup tracking
	return s.GetBackupStatus(ctx, &pb.GetBackupStatusRequest{})
}

// GetBalance returns balance for a chain
func (s *WalletService) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.BalanceResponse, error) {
	// TODO: Implement balance query via adapters
	return &pb.BalanceResponse{
		Chain:         req.Chain,
		ConfirmedSat:  0,
		UnconfirmedSat: 0,
		PendingSwapSat: 0,
		TotalSat:      0,
	}, nil
}

// GetAllBalances returns all balances
func (s *WalletService) GetAllBalances(ctx context.Context, req *pb.GetAllBalancesRequest) (*pb.AllBalancesResponse, error) {
	btc, _ := s.GetBalance(ctx, &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_BTC})
	liquid, _ := s.GetBalance(ctx, &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_LIQUID})
	ln, _ := s.GetBalance(ctx, &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_LN})

	return &pb.AllBalancesResponse{
		Btc:    btc,
		Liquid: liquid,
		Ln:     ln,
	}, nil
}

// GetNewAddress generates a new address
func (s *WalletService) GetNewAddress(ctx context.Context, req *pb.GetNewAddressRequest) (*pb.AddressResponse, error) {
	// Get seed from vault
	seed, err := s.vault.Seed()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "vault locked: %v", err)
	}

	// Derive address
	address, path, err := vault.DeriveBIP84Address(seed, 0, 0, 0, vault.NetworkRegtest)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to derive address: %v", err)
	}

	return &pb.AddressResponse{
		Address:        address,
		Chain:          req.Chain,
		DerivationPath: path,
		Label:          req.Label,
	}, nil
}

// ListAddresses lists addresses
func (s *WalletService) ListAddresses(ctx context.Context, req *pb.ListAddressesRequest) (*pb.ListAddressesResponse, error) {
	return &pb.ListAddressesResponse{Addresses: []*pb.AddressInfo{}}, nil
}

// ListUtxos lists UTXOs
func (s *WalletService) ListUtxos(ctx context.Context, req *pb.ListUtxosRequest) (*pb.ListUtxosResponse, error) {
	return &pb.ListUtxosResponse{Utxos: []*pb.Utxo{}}, nil
}

// ListTransactions lists transactions
func (s *WalletService) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
	return &pb.ListTransactionsResponse{Transactions: []*pb.Transaction{}}, nil
}

// WatchBalances streams balance updates
func (s *WalletService) WatchBalances(req *pb.WatchBalancesRequest, stream pb.WalletService_WatchBalancesServer) error {
	return status.Error(codes.Unimplemented, "not implemented")
}
