module domniwallet/payout-service

go 1.25.6

require (
	domniwallet/reuse/xs_wallet v0.0.0
	github.com/btcsuite/btcd v0.25.0
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0
	github.com/go-chi/chi/v5 v5.0.12
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.1
)

require (
	github.com/btcsuite/btcd/btcec/v2 v2.3.6 // indirect
	github.com/btcsuite/btcd/btcutil v1.1.6 // indirect
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f // indirect
	github.com/decred/dcrd/crypto/blake256 v1.0.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.27.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
)

replace domniwallet/reuse/xs_wallet => ../reuse/xs_wallet
