#!/bin/sh
# LND Entrypoint Script - Simplified
# Manual wallet creation required on first run

set -e

echo "Starting LND..."
echo "==============================================="
echo "First time setup:"
echo "1. Wait for LND to start"
echo "2. Run: docker exec -it xs-lnd lncli --network=regtest create"
echo "3. Follow the prompts to create wallet"
echo "==============================================="

# Start LND with all arguments passed from docker-compose
exec lnd "$@"
