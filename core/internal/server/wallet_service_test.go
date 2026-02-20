package server

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/xs-wallet/xscore/internal/adapters/lnd"
	"github.com/xs-wallet/xscore/internal/config"
	pb "github.com/xs-wallet/xscore/proto"
)

type fakeLNDBalanceClient struct {
	info      *lnd.NodeInfo
	infoErr   error
	walletSat int64
	walletErr error
	localSat  int64
	remoteSat int64
	chanErr   error
}

func (f *fakeLNDBalanceClient) GetNodeInfo(context.Context) (*lnd.NodeInfo, error) {
	return f.info, f.infoErr
}

func (f *fakeLNDBalanceClient) WalletBalance(context.Context) (int64, error) {
	return f.walletSat, f.walletErr
}

func (f *fakeLNDBalanceClient) ChannelBalance(context.Context) (int64, int64, error) {
	return f.localSat, f.remoteSat, f.chanErr
}

func (f *fakeLNDBalanceClient) Close() error { return nil }

func TestGetBalanceLNDisabledReturnsZero(t *testing.T) {
	service := NewWalletService(nil, &config.Config{
		Network: "testnet",
		LND:     config.NodeConfig{Enabled: false},
	}, nil, nil, nil)

	resp, err := service.GetBalance(context.Background(), &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_LN})
	if err != nil {
		t.Fatalf("GetBalance LN disabled: %v", err)
	}
	if resp.TotalSat != 0 || resp.ConfirmedSat != 0 {
		t.Fatalf("expected zero LN balance when disabled, got total=%d confirmed=%d", resp.TotalSat, resp.ConfirmedSat)
	}
}

func TestGetBalanceLNMissingSecretsFailsPrecondition(t *testing.T) {
	service := NewWalletService(nil, &config.Config{
		Network: "testnet",
		LND: config.NodeConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    10009,
		},
	}, nil, nil, nil)

	_, err := service.GetBalance(context.Background(), &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_LN})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v (%v)", status.Code(err), err)
	}
}

func TestGetBalanceLNUsesRealValuesWhenReady(t *testing.T) {
	service := NewWalletService(nil, &config.Config{
		Network: "testnet",
		LND: config.NodeConfig{
			Enabled:  true,
			Host:     "127.0.0.1",
			Port:     10009,
			TLSCert:  "/tmp/tls.cert",
			Macaroon: "/tmp/admin.macaroon",
		},
	}, nil, nil, nil)
	service.newLND = func(cfg lnd.Config) (lndBalanceClient, error) {
		return &fakeLNDBalanceClient{
			info: &lnd.NodeInfo{
				SyncedToChain: true,
				SyncedToGraph: true,
			},
			walletSat: 1200,
			localSat:  800,
		}, nil
	}

	resp, err := service.GetBalance(context.Background(), &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_LN})
	if err != nil {
		t.Fatalf("GetBalance LN ready: %v", err)
	}
	if resp.TotalSat != 2000 || resp.ConfirmedSat != 2000 {
		t.Fatalf("expected LN total/confirmed=2000, got total=%d confirmed=%d", resp.TotalSat, resp.ConfirmedSat)
	}
}

func TestGetBalanceLNSyncingFailsPrecondition(t *testing.T) {
	service := NewWalletService(nil, &config.Config{
		Network: "testnet",
		LND: config.NodeConfig{
			Enabled:  true,
			Host:     "127.0.0.1",
			Port:     10009,
			TLSCert:  "/tmp/tls.cert",
			Macaroon: "/tmp/admin.macaroon",
		},
	}, nil, nil, nil)
	service.newLND = func(cfg lnd.Config) (lndBalanceClient, error) {
		return &fakeLNDBalanceClient{
			info: &lnd.NodeInfo{
				SyncedToChain: true,
				SyncedToGraph: false,
			},
		}, nil
	}

	_, err := service.GetBalance(context.Background(), &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_LN})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v (%v)", status.Code(err), err)
	}
}
