// Package server - Updated WalletService with vault integration
package server

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/adapters/liquid"
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
	liquid *liquid.Client
}

// NewWalletService creates WalletService
func NewWalletService(database *db.DB, cfg *config.Config, v *vault.Vault, btcClient *bitcoin.Client, liquidClient *liquid.Client) *WalletService {
	return &WalletService{
		db:     database,
		cfg:    cfg,
		vault:  v,
		btc:    btcClient,
		liquid: liquidClient,
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
		switch err {
		case vault.ErrInvalidPIN:
			failedAttempts, _, _ := s.vault.GetLockoutStatus()
			return &pb.UnlockVaultResponse{
				Success:           false,
				ErrorMessage:      "Invalid PIN",
				AttemptsRemaining: attemptsRemaining(failedAttempts),
			}, nil
		case vault.ErrTooManyAttempts:
			return &pb.UnlockVaultResponse{
				Success:           false,
				ErrorMessage:      "Vault locked. Try later.",
				AttemptsRemaining: 0,
			}, nil
		case vault.ErrPermanentLockout:
			return &pb.UnlockVaultResponse{
				Success:           false,
				ErrorMessage:      "Permanent lockout. Recovery required.",
				AttemptsRemaining: 0,
			}, nil
		default:
			return nil, status.Errorf(codes.Internal, "failed to unlock vault: %v", err)
		}
	}

	return &pb.UnlockVaultResponse{
		Success:           true,
		SessionId:         sessionID,
		AttemptsRemaining: int32(vault.MaxFailedAttempts),
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

	failedAttempts, lockedUntil, _ := s.vault.GetLockoutStatus()
	var lockoutUntil *timestamppb.Timestamp
	if lockedUntil != nil {
		lockoutUntil = timestamppb.New(*lockedUntil)
	}

	return &pb.VaultStatus{
		State:          state,
		FailedAttempts: int32(failedAttempts),
		LockoutUntil:   lockoutUntil,
	}, nil
}

func attemptsRemaining(failedAttempts int) int32 {
	remaining := vault.MaxFailedAttempts - failedAttempts
	if remaining < 0 {
		remaining = 0
	}
	return int32(remaining)
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
	if req.Chain == pb.Chain_CHAIN_LN {
		return &pb.BalanceResponse{
			Chain:          req.Chain,
			ConfirmedSat:   0,
			UnconfirmedSat: 0,
			PendingSwapSat: 0,
			TotalSat:       0,
		}, nil
	}

	utxosResp, err := s.ListUtxos(ctx, &pb.ListUtxosRequest{
		Chain:           req.Chain,
		IncludeReserved: true,
	})
	if err != nil {
		return nil, err
	}

	var confirmed uint64
	var unconfirmed uint64
	var reserved uint64

	for _, u := range utxosResp.Utxos {
		if u.Reserved {
			reserved += u.AmountSat
		}
		if u.Confirmations > 0 {
			confirmed += u.AmountSat
		} else {
			unconfirmed += u.AmountSat
		}
	}

	return &pb.BalanceResponse{
		Chain:          req.Chain,
		ConfirmedSat:   confirmed,
		UnconfirmedSat: unconfirmed,
		PendingSwapSat: reserved,
		TotalSat:       confirmed + unconfirmed,
	}, nil
}

// GetAllBalances returns all balances
func (s *WalletService) GetAllBalances(ctx context.Context, req *pb.GetAllBalancesRequest) (*pb.AllBalancesResponse, error) {
	btc, err := s.GetBalance(ctx, &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_BTC})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get BTC balance: %v", err)
	}
	liquid, err := s.GetBalance(ctx, &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_LIQUID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get Liquid balance: %v", err)
	}
	ln, err := s.GetBalance(ctx, &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_LN})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get LN balance: %v", err)
	}

	return &pb.AllBalancesResponse{
		Btc:    btc,
		Liquid: liquid,
		Ln:     ln,
	}, nil
}

// GetNewAddress generates a new address
func (s *WalletService) GetNewAddress(ctx context.Context, req *pb.GetNewAddressRequest) (*pb.AddressResponse, error) {
	seed, err := s.vault.Seed()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "vault locked: %v", err)
	}

	switch req.Chain {
	case pb.Chain_CHAIN_BTC:
		addr, path, err := s.deriveAndStoreAddress(ctx, seed, "btc", req.Label, 0)
		if err != nil {
			return nil, err
		}
		return &pb.AddressResponse{
			Address:        addr.Address,
			Chain:          req.Chain,
			DerivationPath: path,
			Label:          req.Label,
		}, nil
	case pb.Chain_CHAIN_LIQUID:
		addr, path, err := s.deriveAndStoreAddress(ctx, seed, "liquid", req.Label, 0)
		if err != nil {
			return nil, err
		}
		return &pb.AddressResponse{
			Address:        addr.Address,
			Chain:          req.Chain,
			DerivationPath: path,
			Label:          req.Label,
		}, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported chain")
	}
}

// ListAddresses lists addresses
func (s *WalletService) ListAddresses(ctx context.Context, req *pb.ListAddressesRequest) (*pb.ListAddressesResponse, error) {
	chainKey, err := chainKeyFromProto(req.Chain)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	query := `
		SELECT address, derivation_path, label, used, account_index, change_index, address_index
		FROM wallet_addresses
		WHERE chain = ?
	`
	args := []interface{}{chainKey}
	if !req.IncludeUsed {
		query += " AND used = 0"
	}
	query += " ORDER BY id ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list addresses: %v", err)
	}
	defer rows.Close()

	addresses := []*pb.AddressInfo{}
	for rows.Next() {
		var address, path string
		var label sql.NullString
		var used int
		var accountIndex, changeIndex, addressIndex int64
		if err := rows.Scan(&address, &path, &label, &used, &accountIndex, &changeIndex, &addressIndex); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to read address: %v", err)
		}
		addresses = append(addresses, &pb.AddressInfo{
			Address:        address,
			Chain:          req.Chain,
			DerivationPath: path,
			Label:          label.String,
			Used:           used == 1,
			BalanceSat:     0,
		})
	}

	return &pb.ListAddressesResponse{Addresses: addresses}, nil
}

// ListUtxos lists UTXOs
func (s *WalletService) ListUtxos(ctx context.Context, req *pb.ListUtxosRequest) (*pb.ListUtxosResponse, error) {
	chainKey, err := chainKeyFromProto(req.Chain)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	addresses, err := s.loadAddresses(ctx, chainKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load addresses: %v", err)
	}
	if len(addresses) == 0 {
		return &pb.ListUtxosResponse{Utxos: []*pb.Utxo{}}, nil
	}

	reservedMap, err := s.loadUtxoReservations(ctx, chainKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load reservations: %v", err)
	}

	switch chainKey {
	case "btc":
		addrList := make([]string, 0, len(addresses))
		for _, addr := range addresses {
			addrList = append(addrList, addr.Address)
		}

		scan, err := s.btc.ScanTxOutSet(ctx, addrList)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "scantxoutset failed: %v", err)
		}

		var height int64
		if scan.Success {
			height = scan.Height
		}

		utxos := make([]*pb.Utxo, 0, len(scan.Unspents))
		for _, u := range scan.Unspents {
			confirmations := int32(0)
			if u.Height > 0 && height >= u.Height {
				confirmations = int32(height - u.Height + 1)
			}
			amountSat := uint64(math.Round(u.Amount * 100000000))
			key := fmt.Sprintf("%s:%d", u.TxID, u.Vout)
			reservation, reserved := reservedMap[key]
			if reserved && !req.IncludeReserved {
				continue
			}

			address := u.Address
			if address == "" && u.ScriptPubKey != "" {
				if addr, err := scriptToAddress(u.ScriptPubKey, btcParamsFromConfig(s.cfg.Network)); err == nil {
					address = addr
				}
			}

			if address != "" {
				_ = s.markAddressUsed(ctx, chainKey, address)
			}

			utxos = append(utxos, &pb.Utxo{
				Txid:              u.TxID,
				Vout:              u.Vout,
				AmountSat:         amountSat,
				Address:           address,
				Confirmations:     confirmations,
				Reserved:          reserved,
				ReservedForSwapId: reservation,
			})
		}

		_ = s.recordReceiveTransactions(ctx, chainKey, utxos)

		return &pb.ListUtxosResponse{Utxos: utxos}, nil
	case "liquid":
		if s.liquid == nil {
			return nil, status.Error(codes.Unavailable, "elementsd adapter not configured")
		}

		addrList := make([]string, 0, len(addresses))
		for _, addr := range addresses {
			if addr.AddressPlain.Valid {
				addrList = append(addrList, addr.AddressPlain.String)
			} else {
				addrList = append(addrList, addr.Address)
			}
		}

		utxosRaw, err := s.liquid.ListUnspent(ctx, 0, 9999999, addrList)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "listunspent failed: %v", err)
		}

		utxos := make([]*pb.Utxo, 0, len(utxosRaw))
		for _, u := range utxosRaw {
			key := fmt.Sprintf("%s:%d", u.TxID, u.Vout)
			reservation, reserved := reservedMap[key]
			if reserved && !req.IncludeReserved {
				continue
			}

			amountSat := uint64(u.AmountSat)
			if u.Address != "" {
				_ = s.markAddressUsed(ctx, chainKey, u.Address)
			}

			utxos = append(utxos, &pb.Utxo{
				Txid:              u.TxID,
				Vout:              u.Vout,
				AmountSat:         amountSat,
				Address:           u.Address,
				Confirmations:     int32(u.Confirmations),
				Reserved:          reserved,
				ReservedForSwapId: reservation,
			})
		}

		_ = s.recordReceiveTransactions(ctx, chainKey, utxos)

		return &pb.ListUtxosResponse{Utxos: utxos}, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported chain")
	}
}

// ListTransactions lists transactions
func (s *WalletService) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
	chainKey, err := chainKeyFromProto(req.Chain)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	limit := int32(50)
	if req.Limit > 0 {
		limit = req.Limit
	}
	offset := int32(0)
	if req.Offset > 0 {
		offset = req.Offset
	}

	var total int32
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE chain = ?`, chainKey).Scan(&total); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to count transactions: %v", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT txid, direction, amount_sat, fee_sat, confirmations, label, created_at
		FROM wallet_transactions
		WHERE chain = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, chainKey, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list transactions: %v", err)
	}
	defer rows.Close()

	transactions := []*pb.Transaction{}
	for rows.Next() {
		var txid, direction, createdAt string
		var amountSat int64
		var feeSat sql.NullInt64
		var confirmations sql.NullInt64
		var label sql.NullString

		if err := rows.Scan(&txid, &direction, &amountSat, &feeSat, &confirmations, &label, &createdAt); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to read tx: %v", err)
		}

		timestamp := parseTime(createdAt)
		tx := &pb.Transaction{
			Txid:          txid,
			Chain:         req.Chain,
			AmountSat:     amountSat,
			FeeSat:        uint64(feeSat.Int64),
			Confirmations: int32(confirmations.Int64),
			Timestamp:     timestamp,
			Label:         label.String,
			SwapId:        "",
		}
		transactions = append(transactions, tx)
	}

	return &pb.ListTransactionsResponse{
		Transactions: transactions,
		TotalCount:   total,
	}, nil
}

// SendOnchain sends an on-chain transaction (BTC or Liquid).
func (s *WalletService) SendOnchain(ctx context.Context, req *pb.SendOnchainRequest) (*pb.SendOnchainResponse, error) {
	if req.AmountSat == 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be > 0")
	}
	if req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}
	if req.SubtractFee {
		return nil, status.Error(codes.Unimplemented, "subtract_fee not implemented")
	}

	seed, err := s.vault.Seed()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "vault locked: %v", err)
	}

	switch req.Chain {
	case pb.Chain_CHAIN_BTC:
		txid, feeSat, err := s.sendBTC(ctx, seed, req.Address, req.AmountSat, req.FeeRateSatVb, req.Label)
		if err != nil {
			return nil, err
		}
		return &pb.SendOnchainResponse{Success: true, Txid: txid, FeeSat: feeSat}, nil
	case pb.Chain_CHAIN_LIQUID:
		txid, feeSat, err := s.sendLiquid(ctx, seed, req.Address, req.AmountSat, req.FeeRateSatVb, req.Label)
		if err != nil {
			return nil, err
		}
		return &pb.SendOnchainResponse{Success: true, Txid: txid, FeeSat: feeSat}, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported chain")
	}
}

// WatchBalances streams balance updates
func (s *WalletService) WatchBalances(req *pb.WatchBalancesRequest, stream pb.WalletService_WatchBalancesServer) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

type storedAddress struct {
	Address        string
	AddressPlain   sql.NullString
	DerivationPath string
	AccountIndex   int64
	ChangeIndex    int64
	AddressIndex   int64
	BlindingPubKey sql.NullString
}

func (s *WalletService) deriveAndStoreAddress(ctx context.Context, seed []byte, chainKey string, label string, changeIndex uint32) (*storedAddress, string, error) {
	accountIndex := uint32(0)
	index, err := s.nextDerivationIndex(ctx, chainKey, changeIndex)
	if err != nil {
		return nil, "", status.Errorf(codes.Internal, "failed to get address index: %v", err)
	}

	network := vaultNetworkFromConfig(s.cfg.Network)

	switch chainKey {
	case "btc":
		params := btcParamsFromConfig(s.cfg.Network)
		coinType := btcCoinType(network)
		address, path, _, err := vault.DeriveBIP84AddressWithParams(seed, accountIndex, changeIndex, index, coinType, params)
		if err != nil {
			return nil, "", status.Errorf(codes.Internal, "failed to derive address: %v", err)
		}

		if err := s.insertAddress(ctx, chainKey, address, "", "", path, accountIndex, changeIndex, index, label); err != nil {
			return nil, "", status.Errorf(codes.Internal, "failed to store address: %v", err)
		}

		return &storedAddress{
			Address:        address,
			DerivationPath: path,
			AccountIndex:   int64(accountIndex),
			ChangeIndex:    int64(changeIndex),
			AddressIndex:   int64(index),
		}, path, nil
	case "liquid":
		if s.liquid == nil {
			return nil, "", status.Error(codes.Unavailable, "elementsd adapter not configured")
		}

		params := vault.LiquidParams(network)
		coinType := vault.LiquidCoinType(network)
		unconf, path, _, err := vault.DeriveBIP84AddressWithParams(seed, accountIndex, changeIndex, index, coinType, params)
		if err != nil {
			return nil, "", status.Errorf(codes.Internal, "failed to derive liquid address: %v", err)
		}

		scriptPubKey, err := addressToScript(unconf, params)
		if err != nil {
			return nil, "", status.Errorf(codes.Internal, "failed to build scriptPubKey: %v", err)
		}

		blindingPriv, blindingPub, err := vault.DeriveBlindingKey(seed, scriptPubKey)
		if err != nil {
			return nil, "", status.Errorf(codes.Internal, "failed to derive blinding key: %v", err)
		}

		blindingPubHex := hex.EncodeToString(blindingPub.SerializeCompressed())
		blindingPrivHex := hex.EncodeToString(blindingPriv)

		conf, err := s.liquid.CreateBlindedAddress(ctx, unconf, blindingPubHex)
		if err != nil {
			return nil, "", status.Errorf(codes.Internal, "failed to create confidential address: %v", err)
		}

		if err := s.insertAddress(ctx, chainKey, conf, unconf, blindingPubHex, path, accountIndex, changeIndex, index, label); err != nil {
			return nil, "", status.Errorf(codes.Internal, "failed to store address: %v", err)
		}

		// Import into elementsd wallet for unblinding
		_ = s.importLiquidAddress(ctx, unconf, label, blindingPrivHex)

		return &storedAddress{
			Address:        conf,
			AddressPlain:   sql.NullString{String: unconf, Valid: true},
			DerivationPath: path,
			AccountIndex:   int64(accountIndex),
			ChangeIndex:    int64(changeIndex),
			AddressIndex:   int64(index),
			BlindingPubKey: sql.NullString{String: blindingPubHex, Valid: true},
		}, path, nil
	default:
		return nil, "", status.Error(codes.InvalidArgument, "unsupported chain")
	}
}

func (s *WalletService) insertAddress(ctx context.Context, chainKey, address, addressPlain, blindingPubKey, path string, accountIndex, changeIndex, addressIndex uint32, label string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO wallet_addresses (
			chain, address, address_plain, blinding_pubkey, derivation_path,
			account_index, change_index, address_index, label, used
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
	`, chainKey, address, nullIfEmpty(addressPlain), nullIfEmpty(blindingPubKey), path, accountIndex, changeIndex, addressIndex, label)
	return err
}

func (s *WalletService) loadAddresses(ctx context.Context, chainKey string) ([]storedAddress, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT address, address_plain, derivation_path, account_index, change_index, address_index, blinding_pubkey
		FROM wallet_addresses
		WHERE chain = ?
		ORDER BY id ASC
	`, chainKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	addresses := []storedAddress{}
	for rows.Next() {
		var addr storedAddress
		if err := rows.Scan(&addr.Address, &addr.AddressPlain, &addr.DerivationPath, &addr.AccountIndex, &addr.ChangeIndex, &addr.AddressIndex, &addr.BlindingPubKey); err != nil {
			return nil, err
		}
		addresses = append(addresses, addr)
	}
	return addresses, nil
}

func (s *WalletService) markAddressUsed(ctx context.Context, chainKey, address string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE wallet_addresses
		SET used = 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE chain = ? AND (address = ? OR address_plain = ?)
	`, chainKey, address, address)
	return err
}

func (s *WalletService) nextDerivationIndex(ctx context.Context, chainKey string, changeIndex uint32) (uint32, error) {
	branch := "external"
	if changeIndex == 1 {
		branch = "change"
	}
	key := fmt.Sprintf("wallet.%s.%s_index", chainKey, branch)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var current int64
	err = tx.QueryRowContext(ctx, `SELECT value FROM app_config WHERE key = ?`, key).Scan(&current)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		current = 0
	}

	next := current + 1
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO app_config (key, value, updated_at)
		VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, strconv.FormatInt(next, 10)); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uint32(current), nil
}

func (s *WalletService) loadUtxoReservations(ctx context.Context, chainKey string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT txid, vout, swap_id
		FROM utxo_reservations
		WHERE chain = ?
	`, chainKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reserved := make(map[string]string)
	for rows.Next() {
		var txid string
		var vout uint32
		var swapID string
		if err := rows.Scan(&txid, &vout, &swapID); err != nil {
			return nil, err
		}
		reserved[fmt.Sprintf("%s:%d", txid, vout)] = swapID
	}
	return reserved, nil
}

func (s *WalletService) recordReceiveTransactions(ctx context.Context, chainKey string, utxos []*pb.Utxo) error {
	if len(utxos) == 0 {
		return nil
	}

	type agg struct {
		amount        int64
		confirmations int32
	}
	aggMap := map[string]*agg{}
	for _, u := range utxos {
		entry := aggMap[u.Txid]
		if entry == nil {
			entry = &agg{}
			aggMap[u.Txid] = entry
		}
		entry.amount += int64(u.AmountSat)
		if u.Confirmations > entry.confirmations {
			entry.confirmations = u.Confirmations
		}
	}

	for txid, entry := range aggMap {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO wallet_transactions (
				txid, chain, direction, amount_sat, fee_sat, confirmations, label, created_at, updated_at
			) VALUES (?, ?, 'in', ?, NULL, ?, '', strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
			ON CONFLICT(txid, chain, direction) DO UPDATE SET
				amount_sat = excluded.amount_sat,
				confirmations = excluded.confirmations,
				updated_at = excluded.updated_at
		`, txid, chainKey, entry.amount, entry.confirmations)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *WalletService) importLiquidAddress(ctx context.Context, addressPlain, label, blindingPrivHex string) error {
	if s.liquid == nil {
		return errors.New("elementsd adapter not configured")
	}

	if err := s.liquid.ImportAddress(ctx, addressPlain, label, false); err != nil && !isAlreadyImported(err) {
		return err
	}

	if err := s.liquid.ImportBlindingKey(ctx, addressPlain, blindingPrivHex); err != nil && !isAlreadyImported(err) {
		return err
	}

	return nil
}

func (s *WalletService) sendBTC(ctx context.Context, seed []byte, destination string, amountSat uint64, feeRateSatVb uint64, label string) (string, uint64, error) {
	params := btcParamsFromConfig(s.cfg.Network)
	if _, err := btcutil.DecodeAddress(destination, params); err != nil {
		return "", 0, status.Errorf(codes.InvalidArgument, "invalid BTC address: %v", err)
	}

	utxosResp, err := s.ListUtxos(ctx, &pb.ListUtxosRequest{
		Chain:           pb.Chain_CHAIN_BTC,
		IncludeReserved: false,
	})
	if err != nil {
		return "", 0, err
	}
	if len(utxosResp.Utxos) == 0 {
		return "", 0, status.Error(codes.FailedPrecondition, "no UTXOs available")
	}

	addresses, err := s.loadAddresses(ctx, "btc")
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "failed to load addresses: %v", err)
	}
	addressMap := map[string]storedAddress{}
	for _, addr := range addresses {
		addressMap[addr.Address] = addr
		if addr.AddressPlain.Valid {
			addressMap[addr.AddressPlain.String] = addr
		}
	}

	utxos := make([]bitcoin.UTXO, 0, len(utxosResp.Utxos))
	for _, u := range utxosResp.Utxos {
		utxos = append(utxos, bitcoin.UTXO{
			TxID:          u.Txid,
			Vout:          u.Vout,
			Address:       u.Address,
			Amount:        float64(u.AmountSat) / 100000000,
			Confirmations: int64(u.Confirmations),
			Spendable:     true,
		})
	}

	feeRate := float64(feeRateSatVb)
	if feeRate <= 0 {
		if est, err := s.btc.EstimateFee(ctx, 6); err == nil && est > 0 {
			feeRate = est
		} else {
			feeRate = 1
		}
	}

	selected, err := bitcoin.SelectUTXOs(utxos, int64(amountSat), feeRate)
	if err != nil {
		return "", 0, status.Errorf(codes.FailedPrecondition, "insufficient funds: %v", err)
	}

	inputs := make([]bitcoin.FundingTxInput, 0, len(selected))
	totalInput := int64(0)
	network := vaultNetworkFromConfig(s.cfg.Network)
	coinType := btcCoinType(network)

	for _, utxo := range selected {
		addr, ok := addressMap[utxo.Address]
		if !ok {
			return "", 0, status.Errorf(codes.Internal, "missing address metadata for utxo %s:%d", utxo.TxID, utxo.Vout)
		}
		privKey, pubKey, _, err := vault.DeriveBIP84Key(seed, uint32(addr.AccountIndex), uint32(addr.ChangeIndex), uint32(addr.AddressIndex), coinType, params)
		if err != nil {
			return "", 0, status.Errorf(codes.Internal, "failed to derive key: %v", err)
		}
		inputs = append(inputs, bitcoin.FundingTxInput{
			UTXO:       utxo,
			PrivateKey: privKey,
			PubKey:     pubKey,
		})
		totalInput += int64(utxo.Amount * 100000000)
	}

	changeAddr, _, err := s.deriveAndStoreAddress(ctx, seed, "btc", "change", 1)
	if err != nil {
		return "", 0, err
	}

	builder := bitcoin.NewTxBuilder(params)
	tx, rawHex, err := builder.BuildFundingTx(inputs, destination, int64(amountSat), changeAddr.Address, feeRate)
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "failed to build tx: %v", err)
	}

	txid, err := s.btc.BroadcastTx(ctx, rawHex)
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "failed to broadcast tx: %v", err)
	}

	outputSum := int64(0)
	for _, out := range tx.TxOut {
		outputSum += out.Value
	}
	feeSat := uint64(totalInput - outputSum)
	_, _ = s.db.ExecContext(ctx, `
		INSERT INTO wallet_transactions (txid, chain, direction, amount_sat, fee_sat, confirmations, label)
		VALUES (?, 'btc', 'out', ?, ?, 0, ?)
	`, txid, -int64(amountSat), feeSat, label)

	return txid, feeSat, nil
}

func (s *WalletService) sendLiquid(ctx context.Context, seed []byte, destination string, amountSat uint64, feeRateSatVb uint64, label string) (string, uint64, error) {
	if s.liquid == nil {
		return "", 0, status.Error(codes.Unavailable, "elementsd adapter not configured")
	}

	addresses, err := s.loadAddresses(ctx, "liquid")
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "failed to load addresses: %v", err)
	}
	addressMap := map[string]storedAddress{}
	for _, addr := range addresses {
		addressMap[addr.Address] = addr
		if addr.AddressPlain.Valid {
			addressMap[addr.AddressPlain.String] = addr
		}
	}

	feeRate := float64(feeRateSatVb)
	if feeRate <= 0 {
		if est, err := s.liquid.EstimateFee(ctx, 6); err == nil && est > 0 {
			feeRate = est
		} else {
			feeRate = 1
		}
	}

	addrList := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		if addr.AddressPlain.Valid {
			addrList = append(addrList, addr.AddressPlain.String)
		} else {
			addrList = append(addrList, addr.Address)
		}
	}
	if len(addrList) == 0 {
		return "", 0, status.Error(codes.FailedPrecondition, "no addresses available")
	}

	liquidUtxos, err := s.liquid.ListUnspent(ctx, 0, 9999999, addrList)
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "listunspent failed: %v", err)
	}

	reservedMap, err := s.loadUtxoReservations(ctx, "liquid")
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "failed to load reservations: %v", err)
	}

	filtered := make([]liquid.UTXO, 0, len(liquidUtxos))
	for _, u := range liquidUtxos {
		key := fmt.Sprintf("%s:%d", u.TxID, u.Vout)
		if _, reserved := reservedMap[key]; reserved {
			continue
		}
		filtered = append(filtered, u)
	}
	if len(filtered) == 0 {
		return "", 0, status.Error(codes.FailedPrecondition, "no UTXOs available")
	}

	selected, totalInput, err := selectLiquidUtxos(filtered, int64(amountSat), feeRate)
	if err != nil {
		return "", 0, status.Errorf(codes.FailedPrecondition, "insufficient funds: %v", err)
	}

	changeAddr, _, err := s.deriveAndStoreAddress(ctx, seed, "liquid", "change", 1)
	if err != nil {
		return "", 0, err
	}

	buildOnce := func(changeSat int64) (string, int64, error) {
		inputs := make([]liquid.RawInput, 0, len(selected))
		for _, u := range selected {
			inputs = append(inputs, liquid.RawInput{TxID: u.TxID, Vout: u.Vout})
		}

		outputs := map[string]float64{
			destination: satToBTC(amountSat),
		}
		if changeSat > 546 {
			outputs[changeAddr.Address] = satToBTC(uint64(changeSat))
		}

		rawHex, err := s.liquid.CreateRawTransaction(ctx, inputs, outputs)
		if err != nil {
			return "", 0, err
		}

		blinded, err := s.liquid.BlindRawTransaction(ctx, rawHex)
		if err != nil {
			return "", 0, err
		}

		vsize, err := s.liquid.DecodeRawTransaction(ctx, blinded)
		if err != nil {
			return "", 0, err
		}
		feeSat := int64(math.Ceil(float64(vsize) * feeRate))
		return blinded, feeSat, nil
	}

	estimatedFee := int64(math.Ceil(200.0 * feeRate))
	changeSat := totalInput - int64(amountSat) - estimatedFee
	if changeSat < 0 {
		return "", 0, status.Error(codes.FailedPrecondition, "insufficient funds for fee")
	}

	blindedHex, feeSat, err := buildOnce(changeSat)
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "failed to build tx: %v", err)
	}

	// Recompute change with actual fee
	actualChange := totalInput - int64(amountSat) - feeSat
	if (actualChange > 546 && actualChange != changeSat) || (actualChange <= 546 && changeSat > 546) {
		blindedHex, feeSat, err = buildOnce(actualChange)
		if err != nil {
			return "", 0, status.Errorf(codes.Internal, "failed to rebuild tx: %v", err)
		}
	}

	prevTxs := make([]liquid.PrevTx, 0, len(selected))
	privKeys := make([]string, 0, len(selected))
	network := vaultNetworkFromConfig(s.cfg.Network)
	params := vault.LiquidParams(network)
	coinType := vault.LiquidCoinType(network)

	for _, u := range selected {
		addr, ok := addressMap[u.Address]
		if !ok {
			return "", 0, status.Errorf(codes.Internal, "missing address metadata for utxo %s:%d", u.TxID, u.Vout)
		}
		privKey, _, _, err := vault.DeriveBIP84Key(seed, uint32(addr.AccountIndex), uint32(addr.ChangeIndex), uint32(addr.AddressIndex), coinType, params)
		if err != nil {
			return "", 0, status.Errorf(codes.Internal, "failed to derive key: %v", err)
		}
		wif, err := btcutil.NewWIF(privKey, params, true)
		if err != nil {
			return "", 0, status.Errorf(codes.Internal, "failed to encode WIF: %v", err)
		}
		privKeys = append(privKeys, wif.String())

		prevTxs = append(prevTxs, liquid.PrevTx{
			TxID:         u.TxID,
			Vout:         u.Vout,
			ScriptPubKey: u.ScriptPubKey,
			Amount:       satToBTC(uint64(u.AmountSat)),
		})
	}

	signedHex, complete, err := s.liquid.SignRawTransactionWithKey(ctx, blindedHex, privKeys, prevTxs)
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "failed to sign tx: %v", err)
	}
	if !complete {
		return "", 0, status.Error(codes.Internal, "transaction signing incomplete")
	}

	txid, err := s.liquid.SendRawTransaction(ctx, signedHex)
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, "failed to broadcast tx: %v", err)
	}

	_, _ = s.db.ExecContext(ctx, `
		INSERT INTO wallet_transactions (txid, chain, direction, amount_sat, fee_sat, confirmations, label)
		VALUES (?, 'liquid', 'out', ?, ?, 0, ?)
	`, txid, -int64(amountSat), feeSat, label)

	return txid, uint64(feeSat), nil
}

func selectLiquidUtxos(utxos []liquid.UTXO, targetAmount int64, feeRate float64) ([]liquid.UTXO, int64, error) {
	selected := []liquid.UTXO{}
	total := int64(0)

	for _, u := range utxos {
		selected = append(selected, u)
		total += int64(u.AmountSat)
		estimatedFee := int64(math.Ceil(float64(len(selected))*200.0*feeRate + 200.0*feeRate))
		if total >= targetAmount+estimatedFee {
			return selected, total, nil
		}
	}

	return nil, total, fmt.Errorf("insufficient funds: need %d sat, have %d sat", targetAmount, total)
}

func scriptToAddress(scriptHex string, params *chaincfg.Params) (string, error) {
	scriptBytes, err := hex.DecodeString(scriptHex)
	if err != nil {
		return "", err
	}
	_, addresses, _, err := txscript.ExtractPkScriptAddrs(scriptBytes, params)
	if err != nil {
		return "", err
	}
	if len(addresses) == 0 {
		return "", errors.New("no address found")
	}
	return addresses[0].EncodeAddress(), nil
}

func addressToScript(address string, params *chaincfg.Params) ([]byte, error) {
	addr, err := btcutil.DecodeAddress(address, params)
	if err != nil {
		return nil, err
	}
	return txscript.PayToAddrScript(addr)
}

func chainKeyFromProto(chain pb.Chain) (string, error) {
	switch chain {
	case pb.Chain_CHAIN_BTC:
		return "btc", nil
	case pb.Chain_CHAIN_LIQUID:
		return "liquid", nil
	default:
		return "", errors.New("unsupported chain")
	}
}

func vaultNetworkFromConfig(network string) vault.Network {
	switch network {
	case "mainnet":
		return vault.NetworkMainnet
	case "testnet":
		return vault.NetworkTestnet
	default:
		return vault.NetworkRegtest
	}
}

func btcParamsFromConfig(network string) *chaincfg.Params {
	switch network {
	case "mainnet":
		return &chaincfg.MainNetParams
	case "testnet":
		return &chaincfg.TestNet3Params
	default:
		return &chaincfg.RegressionNetParams
	}
}

func btcCoinType(network vault.Network) uint32 {
	if network == vault.NetworkMainnet {
		return 0
	}
	return 1
}

func nullIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func parseTime(value string) *timestamppb.Timestamp {
	if value == "" {
		return nil
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return timestamppb.New(ts)
}

func satToBTC(amountSat uint64) float64 {
	return float64(amountSat) / 100000000
}

func isAlreadyImported(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "already") || strings.Contains(msg, "exists")
}
