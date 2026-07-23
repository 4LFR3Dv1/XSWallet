# BRLN-OS DevDash

Experimental developer dashboard for blockchain development featuring atomic swaps, HD wallets, and multi-chain signatures.

## Features

### Core Pages
1. **Home/Overview** - System status, quick actions, metrics, and activity feed
2. **Wallet Studio** - Generate, derive, encrypt, and restore HD wallets
3. **HTLC Laboratory** - Build, decode, and simulate Hash Time-Locked Contracts
4. **Swap Center** - Create and manage atomic swaps (BTC ↔ LN ↔ L-BTC)
5. **Network** - Monitor node health, channels, and mempool status
6. **API Explorer** - Test and debug API endpoints with request builder
7. **Settings** - Configure theme, network, and developer options

### Outlier Patterns
- **Observability Ribbon** - Real-time metrics (latency, error rate, throughput) with sparklines
- **Inspector Drawer** - Collapsible side panel for detailed item inspection
- **Timeline Dock** - Universal event feed with filtering by category and severity

### Design System
- **Colors**: BTC Orange (#F7931A), LN Purple (#9945FF), Liquid Cyan (#00B4E6)
- **Typography**: Inter (UI), JetBrains Mono (code)
- **Spacing**: 4px scale (4, 8, 12, 16, 24, 32, 48, 64)
- **Radius**: Cards 12px, Buttons 8px, Inputs 6px, Badges 4px
- **Themes**: Dark (default) and Light mode

### Component Library
- StatusCard - Status indicators with icons and health states
- CodeBlock - Syntax-highlighted code with line numbers, copy, and wrap toggle
- CopyButton - One-click copy with visual feedback
- NetworkBadge - Colored badges for BTC/LN/L-BTC networks
- AddressDisplay - Truncated addresses with expand and copy
- StateMachine - Interactive state flow visualization
- TreeView - Collapsible hierarchical data (e.g., BIP44 derivation paths)
- ObservabilityRibbon - System metrics with mini sparklines
- TimelineDock - Filterable event stream
- InspectorDrawer - Detailed item inspector

## Navigation
- Collapsible sidebar with icons and labels
- Global search in topbar
- Network selector (Mainnet/Testnet/Regtest)
- Theme toggle (Dark/Light)

## Mock Data
All data is mocked for demonstration purposes. Real implementation would connect to:
- Bitcoin Core RPC
- LND gRPC API
- Elements/Liquid daemon
- Custom backend API for HTLC and swap management

## Technology Stack
- React 18.3+ with TypeScript
- Tailwind CSS v4
- Lucide React icons
- Radix UI primitives
- Design system: BRLN-OS specification
