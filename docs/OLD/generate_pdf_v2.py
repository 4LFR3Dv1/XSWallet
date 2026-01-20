"""
XS Wallet - Technical Specification Document v2
Atualizado com Go Core, boltz-backend self-hosted, e feedback CTO
Design enterprise-grade focado em clareza e legibilidade
"""

from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch, mm
from reportlab.lib.enums import TA_CENTER, TA_JUSTIFY, TA_LEFT
from reportlab.platypus import (SimpleDocTemplate, Paragraph, Spacer, PageBreak, 
                                Table, TableStyle, Preformatted)
from reportlab.lib import colors
from reportlab.pdfgen import canvas
from datetime import datetime

OUTPUT_FILE = r"c:\Users\windows10\Downloads\XS WALLET\docs\XS_Wallet_Technical_Specification_v2.pdf"

# Design System - Foco em legibilidade
COLORS = {
    'text': colors.HexColor('#1a1a1a'),
    'text_secondary': colors.HexColor('#4a4a4a'),
    'heading': colors.HexColor('#0a0a0a'),
    'accent': colors.HexColor('#2563eb'),
    'border': colors.HexColor('#e5e5e5'),
    'bg_code': colors.HexColor('#f5f5f5'),
    'bg_table_header': colors.HexColor('#f0f0f0'),
    'go_blue': colors.HexColor('#00ADD8'),
}

def create_styles():
    """Estilos focados em legibilidade técnica"""
    styles = getSampleStyleSheet()
    
    styles['Normal'].fontSize = 10
    styles['Normal'].leading = 14
    styles['Normal'].textColor = COLORS['text']
    styles['Normal'].alignment = TA_JUSTIFY
    styles['Normal'].spaceAfter = 8
    
    styles.add(ParagraphStyle(
        'H1',
        parent=styles['Heading1'],
        fontSize=16,
        fontName='Helvetica-Bold',
        textColor=COLORS['heading'],
        spaceBefore=24,
        spaceAfter=12,
    ))
    
    styles.add(ParagraphStyle(
        'H2',
        parent=styles['Heading2'],
        fontSize=13,
        fontName='Helvetica-Bold',
        textColor=COLORS['heading'],
        spaceBefore=18,
        spaceAfter=8,
    ))
    
    styles.add(ParagraphStyle(
        'H3',
        parent=styles['Heading3'],
        fontSize=11,
        fontName='Helvetica-Bold',
        textColor=COLORS['text'],
        spaceBefore=12,
        spaceAfter=6,
    ))
    
    styles.add(ParagraphStyle(
        'CodeBlock',
        parent=styles['Normal'],
        fontSize=8,
        fontName='Courier',
        textColor=COLORS['text'],
        backColor=COLORS['bg_code'],
        leftIndent=10,
        rightIndent=10,
        spaceBefore=6,
        spaceAfter=6,
        leading=11,
    ))
    
    styles.add(ParagraphStyle(
        'Note',
        parent=styles['Normal'],
        fontSize=9,
        fontName='Helvetica-Oblique',
        textColor=COLORS['text_secondary'],
        leftIndent=20,
        rightIndent=20,
        spaceBefore=8,
        spaceAfter=8,
    ))
    
    styles.add(ParagraphStyle(
        'Caption',
        parent=styles['Normal'],
        fontSize=9,
        fontName='Helvetica-Oblique',
        textColor=COLORS['text_secondary'],
        alignment=TA_CENTER,
        spaceBefore=4,
        spaceAfter=12,
    ))
    
    return styles

def create_table(data, col_widths=None, header=True):
    """Tabela técnica limpa"""
    if col_widths is None:
        col_widths = [1.5*inch] * len(data[0])
    
    table = Table(data, colWidths=col_widths)
    
    style_commands = [
        ('FONTNAME', (0, 0), (-1, -1), 'Helvetica'),
        ('FONTSIZE', (0, 0), (-1, -1), 9),
        ('TEXTCOLOR', (0, 0), (-1, -1), COLORS['text']),
        ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
        ('VALIGN', (0, 0), (-1, -1), 'TOP'),
        ('TOPPADDING', (0, 0), (-1, -1), 6),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 6),
        ('LEFTPADDING', (0, 0), (-1, -1), 8),
        ('RIGHTPADDING', (0, 0), (-1, -1), 8),
        ('GRID', (0, 0), (-1, -1), 0.5, COLORS['border']),
    ]
    
    if header:
        style_commands.extend([
            ('BACKGROUND', (0, 0), (-1, 0), COLORS['bg_table_header']),
            ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
        ])
    
    table.setStyle(TableStyle(style_commands))
    return table

def add_header_footer(canvas, doc):
    """Header e footer informativos"""
    canvas.saveState()
    
    canvas.setFont('Helvetica', 8)
    canvas.setFillColor(COLORS['text_secondary'])
    canvas.drawString(doc.leftMargin, doc.height + doc.topMargin - 10, 
                      "XS Wallet - Technical Specification v0.2.0")
    canvas.drawRightString(doc.width + doc.leftMargin, doc.height + doc.topMargin - 10,
                           "CONFIDENTIAL")
    
    canvas.drawString(doc.leftMargin, 20, 
                      f"Generated: {datetime.now().strftime('%Y-%m-%d %H:%M')}")
    canvas.drawRightString(doc.width + doc.leftMargin, 20, 
                           f"Page {doc.page}")
    
    canvas.setStrokeColor(COLORS['border'])
    canvas.line(doc.leftMargin, doc.height + doc.topMargin - 15, 
                doc.width + doc.leftMargin, doc.height + doc.topMargin - 15)
    canvas.line(doc.leftMargin, 30, doc.width + doc.leftMargin, 30)
    
    canvas.restoreState()

def create_pdf():
    """Gera documento técnico v2 completo"""
    doc = SimpleDocTemplate(
        OUTPUT_FILE,
        pagesize=A4,
        rightMargin=0.75*inch,
        leftMargin=0.75*inch,
        topMargin=0.75*inch,
        bottomMargin=0.6*inch
    )
    
    styles = create_styles()
    story = []
    
    # =========================================================================
    # CAPA
    # =========================================================================
    story.append(Spacer(1, 2*inch))
    
    story.append(Paragraph("XS WALLET", ParagraphStyle(
        'Title', parent=styles['H1'], fontSize=28, alignment=TA_CENTER, spaceAfter=20
    )))
    
    story.append(Paragraph("Technical Specification Document v2", ParagraphStyle(
        'Subtitle', parent=styles['Normal'], fontSize=14, alignment=TA_CENTER, 
        textColor=COLORS['text_secondary'], spaceAfter=40
    )))
    
    story.append(Paragraph("Go Core + Atomic Swaps Desktop Application", ParagraphStyle(
        'Desc', parent=styles['Normal'], fontSize=12, alignment=TA_CENTER, spaceAfter=60
    )))
    
    meta_data = [
        ['Document', 'Value'],
        ['Version', '0.2.0'],
        ['Status', 'Phase 1 Complete - Boltz Client Production-Ready'],
        ['Date', 'January 20, 2026'],
        ['Classification', 'Technical Specification'],
        ['Target Audience', 'Developers, Architects, Auditors, CTO'],
        ['Backend', 'Go Core (xscore) with gRPC'],
        ['Swap Provider', 'boltz-backend (self-hosted, our liquidity)'],
    ]
    story.append(create_table(meta_data, [2*inch, 3.5*inch]))
    
    story.append(PageBreak())
    
    # =========================================================================
    # ÍNDICE
    # =========================================================================
    story.append(Paragraph("Table of Contents", styles['H1']))
    story.append(Spacer(1, 12))
    
    toc_items = [
        "1. Executive Summary",
        "2. System Architecture (Updated)",
        "   2.1 Go Core + gRPC",
        "   2.2 boltz-backend Self-Hosted",
        "   2.3 2 RPC Primary Interfaces",
        "3. Boltz Client Implementation",
        "   3.1 HTTP Client (Production-Ready)",
        "   3.2 WebSocket Client (Production-Ready)",
        "   3.3 Status Normalization",
        "4. Database Specification",
        "5. Atomic Swap Protocol",
        "6. Cryptographic Specifications",
        "7. Security Model",
        "8. Implementation Status",
        "9. Roadmap",
    ]
    
    for item in toc_items:
        indent = 0 if not item.startswith("   ") else 20
        story.append(Paragraph(item.strip(), ParagraphStyle(
            'TOC', parent=styles['Normal'], leftIndent=indent, fontSize=10, spaceAfter=4
        )))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 1. EXECUTIVE SUMMARY
    # =========================================================================
    story.append(Paragraph("1. Executive Summary", styles['H1']))
    
    story.append(Paragraph("""
    XS Wallet is a self-custody desktop application enabling atomic swaps between Bitcoin (BTC), 
    Liquid (L-BTC), and Lightning Network (LN) using Taproot and <b>boltz-backend (self-hosted)</b> 
    as swap orchestrator with <b>our liquidity</b>. The system maintains true self-custody while 
    providing seamless cross-chain and cross-layer swaps.
    """, styles['Normal']))
    
    story.append(Paragraph("""
    <b>Key Architecture Change (CTO Feedback)</b>: The system now uses boltz-backend self-hosted 
    instead of Boltz public API, with 2 primary RPC interfaces (LND gRPC + elementsd JSON-RPC), 
    and bitcoind as runtime dependency only.
    """, styles['Note']))
    
    story.append(Paragraph("1.1 Core Capabilities", styles['H2']))
    
    capabilities = [
        ['Capability', 'Implementation', 'Status'],
        ['Go Core (xscore)', 'Authoritative backend in Go with gRPC', 'Implemented'],
        ['Boltz HTTP Client', 'Retry/backoff, Retry-After, typed errors', 'Production-Ready'],
        ['Boltz WebSocket', 'Single-writer, gap-recovery, connCtx', 'Production-Ready'],
        ['Status Normalization', 'Table-driven per swap type', 'Implemented'],
        ['Swap Engine', 'CAS/optimistic locking, idempotency', 'Base Implemented'],
        ['HD Wallet', 'BIP39/32/84/85 derivation', 'Partial'],
        ['Encryption', 'Argon2id + AES-256-GCM', 'Designed'],
    ]
    story.append(create_table(capabilities, [1.8*inch, 2.5*inch, 1.7*inch]))
    story.append(Paragraph("Table 1.1: Core Capabilities Matrix", styles['Caption']))
    
    story.append(Paragraph("1.2 Design Principles", styles['H2']))
    
    principles = """
    <b>Self-Custody</b>: User controls all private keys; no server holds funds or signing authority.<br/><br/>
    <b>Zero Trust</b>: All external data (boltz-backend responses, node data) is verified locally.<br/><br/>
    <b>Crash Recovery</b>: All swap states are persisted; swaps can resume after any failure.<br/><br/>
    <b>Authoritative State Machine</b>: Go Core is the single source of truth for swap state (CTO endorsed as "coisa de gente cuidadosa").
    """
    story.append(Paragraph(principles, styles['Normal']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 2. SYSTEM ARCHITECTURE
    # =========================================================================
    story.append(Paragraph("2. System Architecture (Updated)", styles['H1']))
    
    story.append(Paragraph("2.1 Go Core + gRPC", styles['H2']))
    
    story.append(Paragraph("""
    The backend has been refactored from Node.js to <b>Go Core (xscore)</b>, providing superior 
    performance for cryptographic operations and native cross-platform support. Communication 
    with frontend uses <b>gRPC</b> instead of Electron IPC, enabling future mobile clients.
    """, styles['Normal']))
    
    arch_diagram = """
    ┌─────────────────────────────────────────────────────────────────────────┐
    │                         XS Wallet Desktop Application                    │
    ├─────────────────────────────────────────────────────────────────────────┤
    │                                                                          │
    │  ┌──────────────────┐              ┌──────────────────────────────────┐ │
    │  │  Electron        │              │     Go Core (xscore)             │ │
    │  │  React UI        │◄────gRPC────►│                                   │ │
    │  │                  │              │  ┌─────────────────────────────┐ │ │
    │  │  • Dashboard     │              │  │      Swap Engine            │ │ │
    │  │  • Swap UI       │              │  │  • State Machine (CAS)      │ │ │
    │  │  • Settings      │              │  │  • Idempotent Operations    │ │ │
    │  └──────────────────┘              │  │  • Crash Recovery           │ │ │
    │                                    │  └─────────────────────────────┘ │ │
    │                                    │                                   │ │
    │                                    │  ┌─────────────────────────────┐ │ │
    │                                    │  │      Boltz Client           │ │ │
    │                                    │  │  • HTTP + Retry/Backoff     │ │ │
    │                                    │  │  • WebSocket Single-Writer  │ │ │
    │                                    │  │  • Status Normalization     │ │ │
    │                                    │  └─────────────────────────────┘ │ │
    │                                    │                                   │ │
    │                                    │  ┌─────────────────────────────┐ │ │
    │                                    │  │   Adapters (2 RPC Primary)  │ │ │
    │                                    │  │  • LND (gRPC)               │ │ │
    │                                    │  │  • elementsd (JSON-RPC)     │ │ │
    │                                    │  └─────────────────────────────┘ │ │
    │                                    └──────────────────────────────────┘ │
    └─────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼ HTTPS/WSS
                              ┌─────────────────────────┐
                              │    boltz-backend        │
                              │    (self-hosted)        │
                              │    Our Liquidity        │
                              │    REST compat. v2      │
                              └─────────────────────────┘
                                         │
                    ┌────────────────────┼────────────────────┐
                    ▼                    ▼                    ▼
               ┌────────┐          ┌──────────┐         ┌─────────┐
               │ LND    │          │elementsd │         │bitcoind │
               │(gRPC)  │          │(JSON-RPC)│         │(runtime)│
               │PRIMARY │          │ PRIMARY  │         │DEPENDENCY│
               └────────┘          └──────────┘         └─────────┘
    """
    
    story.append(Preformatted(arch_diagram, styles['CodeBlock']))
    story.append(Paragraph("Figure 2.1: Updated System Architecture", styles['Caption']))
    
    story.append(Paragraph("2.2 boltz-backend Self-Hosted", styles['H2']))
    
    story.append(Paragraph("""
    <b>Key Change</b>: Instead of using Boltz public API as liquidity provider, the system 
    uses <b>boltz-backend self-hosted</b> with our own liquidity. This provides:
    """, styles['Normal']))
    
    boltz_benefits = [
        ['Benefit', 'Description'],
        ['Internal Control', 'Full control over liquidity and swap orchestration'],
        ['Reduced Dependencies', 'No external API availability concerns'],
        ['API Compatibility', 'REST interface compatible with Boltz v2 spec'],
        ['WebSocket Support', 'Real-time swap status updates'],
    ]
    story.append(create_table(boltz_benefits, [2*inch, 4*inch]))
    story.append(Paragraph("Table 2.1: boltz-backend Benefits", styles['Caption']))
    
    story.append(Paragraph("2.3 Two Primary RPC Interfaces (CTO Directive)", styles['H2']))
    
    story.append(Paragraph("""
    Per CTO feedback, the integration model has been simplified to 2 primary RPC interfaces:
    """, styles['Normal']))
    
    rpc_interfaces = [
        ['Interface', 'Protocol', 'Purpose', 'Status'],
        ['LND', 'gRPC', 'Lightning (invoices, payments, channels)', 'Primary'],
        ['elementsd', 'JSON-RPC', 'Liquid (addresses, txs, blinding)', 'Primary'],
        ['bitcoind', '-', 'Runtime dependency (not direct RPC)', 'Dependency'],
    ]
    story.append(create_table(rpc_interfaces, [1.3*inch, 1.2*inch, 2.5*inch, 1*inch]))
    story.append(Paragraph("Table 2.2: RPC Interfaces", styles['Caption']))
    
    story.append(Paragraph("""
    <b>Rationale</b>: "o lnd em si já faz manejo on-chain e lightning" (CTO) - LND already 
    handles on-chain operations, eliminating the need for direct bitcoind RPC from the wallet.
    """, styles['Note']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 3. BOLTZ CLIENT IMPLEMENTATION
    # =========================================================================
    story.append(Paragraph("3. Boltz Client Implementation", styles['H1']))
    
    story.append(Paragraph("""
    The Boltz Client has been implemented with production-grade reliability standards. All 
    components are tested and ready for integration with boltz-backend.
    """, styles['Normal']))
    
    story.append(Paragraph("3.1 HTTP Client (Production-Ready)", styles['H2']))
    
    http_features = [
        ['Feature', 'Implementation', 'File'],
        ['Exponential Backoff', 'initialBackoff * 2^attempt (max 10s)', 'client.go'],
        ['Retry-After Header', 'Parses and respects 429 header', 'client.go'],
        ['Resource Leak Prevention', 'Body closed in closure, not defer in loop', 'client.go'],
        ['Typed Errors', 'ErrSwapNotFound, ErrPairHashMismatch, etc', 'errors.go'],
        ['All v2 Endpoints', '/v2/swap/submarine, /reverse, /chain, etc', 'client.go'],
    ]
    story.append(create_table(http_features, [1.8*inch, 2.5*inch, 1.7*inch]))
    story.append(Paragraph("Table 3.1: HTTP Client Features", styles['Caption']))
    
    http_code = """
    // Body closed in closure - prevents leak in retry loop
    respBody, statusCode, err := func() ([]byte, int, error) {
        resp, err := c.httpClient.Do(req)
        if err != nil { return nil, 0, err }
        defer resp.Body.Close() // Closed HERE, not in outer loop!
        return io.ReadAll(resp.Body)
    }()
    """
    story.append(Preformatted(http_code, styles['CodeBlock']))
    story.append(Paragraph("Listing 3.1: Resource Leak Prevention Pattern", styles['Caption']))
    
    story.append(Paragraph("3.2 WebSocket Client (Production-Ready)", styles['H2']))
    
    ws_features = [
        ['Feature', 'Implementation', 'Rationale'],
        ['Single-Writer', 'writeMu sync.Mutex', 'Prevents concurrent write races'],
        ['connCtx per Connection', 'Context canceled on disconnect', 'Prevents goroutine duplication'],
        ['Gap Recovery', 'REST poll on reconnect', 'Catches missed updates'],
        ['Channel swap.update', 'Correct v2 WebSocket format', 'Per Boltz spec'],
        ['Ping/Pong Heartbeat', '30s interval', 'Connection keepalive'],
    ]
    story.append(create_table(ws_features, [1.6*inch, 2*inch, 2.4*inch]))
    story.append(Paragraph("Table 3.2: WebSocket Client Features", styles['Caption']))
    
    story.append(Paragraph("3.3 Status Normalization", styles['H2']))
    
    story.append(Paragraph("""
    Status normalization is table-driven per swap type, mapping Boltz status strings to 
    internal state machine states with appropriate actions.
    """, styles['Normal']))
    
    status_map = [
        ['Boltz Status', 'Swap Type', 'Internal State', 'Action'],
        ['transaction.claim.pending', 'Submarine', 'StateSigningMusig2Partial', 'Sign'],
        ['invoice.settled', 'Reverse', 'StateCompleted', 'None'],
        ['transaction.mempool', 'Reverse', 'StateSigningMusig2Partial', 'Sign'],
        ['invoice.failedToPay', 'Submarine', 'StateFallbackScriptReady', 'Refund'],
        ['swap.expired', 'All', 'StateFailed', 'None'],
    ]
    story.append(create_table(status_map, [1.8*inch, 1.2*inch, 1.8*inch, 1.2*inch]))
    story.append(Paragraph("Table 3.3: Status Normalization Examples", styles['Caption']))
    
    story.append(Paragraph("""
    <b>Critical Fix</b>: transaction.claim.pending (not invoice.paid) is the correct trigger 
    for MuSig2 partial signature in submarine swaps.
    """, styles['Note']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 4. DATABASE SPECIFICATION
    # =========================================================================
    story.append(Paragraph("4. Database Specification", styles['H1']))
    
    story.append(Paragraph("4.1 Schema Design", styles['H2']))
    
    story.append(Paragraph("""
    The database uses SQLite with Write-Ahead Logging (WAL) for optimal read/write concurrency. 
    All swap state mutations use Compare-And-Swap (CAS) with version-based optimistic locking.
    """, styles['Normal']))
    
    tables_summary = [
        ['Table', 'Primary Key', 'Purpose'],
        ['swaps', 'id (TEXT)', 'Authoritative swap state with CAS version'],
        ['swap_events', 'seq (AUTOINCREMENT)', 'Immutable audit log for debugging'],
        ['swap_ops', '(swap_id, op_key)', 'Idempotent operation ledger'],
        ['utxo_reservations', '(chain, txid, vout)', 'Prevents UTXO double-spend'],
        ['ln_reservations', 'payment_hash_hex', 'Prevents duplicate LN payments'],
        ['app_config', 'key', 'User configuration (snapshot in locked_intent)'],
    ]
    story.append(create_table(tables_summary, [1.5*inch, 1.8*inch, 2.7*inch]))
    story.append(Paragraph("Table 4.1: Database Tables", styles['Caption']))
    
    story.append(Paragraph("4.2 CTO-Endorsed Design Patterns", styles['H2']))
    
    story.append(Paragraph("""
    The following patterns were specifically praised by CTO as "coisa de gente cuidadosa":
    """, styles['Normal']))
    
    patterns = [
        ['Pattern', 'Implementation', 'Benefit'],
        ['locked_intent', 'Immutable JSON snapshot of quote/fees', 'Audit trail; prevents mutation'],
        ['CAS with version', 'UPDATE ... WHERE version = ?', 'Prevents race conditions'],
        ['swap_ops ledger', 'INSERT OR IGNORE + SELECT', 'Safe retries; no duplicate actions'],
        ['utxo_reservations', 'Reserve before broadcast', 'No double-spend across swaps'],
        ['ln_reservations', 'Reserve before pay', 'No duplicate LN payments'],
    ]
    story.append(create_table(patterns, [1.5*inch, 2.3*inch, 2.2*inch]))
    story.append(Paragraph("Table 4.2: Production-Ready Patterns", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 5. ATOMIC SWAP PROTOCOL
    # =========================================================================
    story.append(Paragraph("5. Atomic Swap Protocol", styles['H1']))
    
    story.append(Paragraph("5.1 Responsibility Split: Wallet vs boltz-backend", styles['H2']))
    
    story.append(Paragraph("""
    The responsibility split between wallet and boltz-backend is being documented in the 
    current spike. Key points:
    """, styles['Normal']))
    
    responsibilities = [
        ['Operation', 'Wallet', 'boltz-backend'],
        ['Preimage generation (reverse/chain)', '✓', ''],
        ['HTLC script verification', '✓', ''],
        ['Funding HTLCs', '✓', ''],
        ['Claim HTLCs (reveal preimage)', '✓ (reverse/chain)', '✓ (submarine)'],
        ['LND payment', '✓', '✓'],
        ['Liquidity management', '', '✓'],
        ['WebSocket status updates', 'Consumer', 'Producer'],
    ]
    story.append(create_table(responsibilities, [2.5*inch, 1.5*inch, 2*inch]))
    story.append(Paragraph("Table 5.1: Responsibility Split", styles['Caption']))
    
    story.append(Paragraph("5.2 State Machine", styles['H2']))
    
    states = [
        ['State', 'Description', 'Transitions'],
        ['open', 'Quote accepted, not locked', 'locked | canceled'],
        ['locked', 'Parameters locked, intent immutable', 'commit_started | canceled'],
        ['commit_started', 'Funding broadcast', 'waiting | failed'],
        ['waiting', 'Awaiting counterparty', 'waiting_claim_details | refund_coop_waiting'],
        ['signing_musig2_partial', 'Creating partial sig', 'sent_partial_to_provider'],
        ['waiting_provider_broadcast', 'Provider broadcasting', 'completed | fallback_script_ready'],
        ['fallback_script_ready', 'Script-path fallback', 'refunding | completed'],
        ['completed', 'Terminal success', '(terminal)'],
        ['failed', 'Terminal failure', '(terminal)'],
    ]
    story.append(create_table(states, [1.8*inch, 2*inch, 2.2*inch]))
    story.append(Paragraph("Table 5.2: Swap State Machine", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 6. CRYPTOGRAPHIC SPECIFICATIONS
    # =========================================================================
    story.append(Paragraph("6. Cryptographic Specifications", styles['H1']))
    
    story.append(Paragraph("6.1 Vault Encryption", styles['H2']))
    
    vault_spec = [
        ['Parameter', 'Value', 'Rationale'],
        ['KDF', 'Argon2id', 'Memory-hard; GPU/ASIC resistant'],
        ['Memory', '64 MB', 'Desktop-appropriate security/performance'],
        ['Iterations', '3', 'Standard recommendation'],
        ['Cipher', 'AES-256-GCM', 'Authenticated encryption'],
        ['IV Length', '12 bytes', 'GCM standard'],
    ]
    story.append(create_table(vault_spec, [1.5*inch, 1.5*inch, 3*inch]))
    story.append(Paragraph("Table 6.1: Vault Encryption Parameters", styles['Caption']))
    
    story.append(Paragraph("6.2 MuSig2 + Taproot", styles['H2']))
    
    musig_spec = [
        ['Component', 'Specification'],
        ['Key Order', 'Provider pubkey first, then user (deterministic)'],
        ['Nonce Generation', 'Deterministic from session + private key'],
        ['Session Persistence', 'musig_* fields in swaps table'],
        ['Key-Path Spend', 'MuSig2 aggregated signature (cooperative)'],
        ['Script-Path Fallback', 'HTLC conditions if cooperation fails'],
    ]
    story.append(create_table(musig_spec, [2*inch, 4*inch]))
    story.append(Paragraph("Table 6.2: MuSig2 Specification", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 7. SECURITY MODEL
    # =========================================================================
    story.append(Paragraph("7. Security Model", styles['H1']))
    
    threats = [
        ['Threat', 'In Scope', 'Mitigation'],
        ['Seed theft (offline)', 'Yes', 'Argon2id + device secret (v1.1)'],
        ['PIN brute force', 'Yes', 'Exponential backoff; lockout'],
        ['Swap manipulation', 'Yes', 'Local script verification'],
        ['Race conditions', 'Yes', 'CAS + idempotency ledger'],
        ['Double broadcast', 'Yes', 'swap_ops table'],
        ['Duplicate LN payment', 'Yes', 'ln_reservations table'],
    ]
    story.append(create_table(threats, [1.8*inch, 1*inch, 3.2*inch]))
    story.append(Paragraph("Table 7.1: Threat Model", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 8. IMPLEMENTATION STATUS
    # =========================================================================
    story.append(Paragraph("8. Implementation Status", styles['H1']))
    
    story.append(Paragraph("""
    Current progress: <b>~40% of MVP complete</b>. Phase 1 (Boltz Client) is production-ready.
    """, styles['Normal']))
    
    impl_status = [
        ['Component', 'Files', 'Status'],
        ['Go Core (xscore)', 'cmd/xscore/main.go', 'Implemented'],
        ['Boltz HTTP Client', 'internal/boltz/client.go', 'Production-Ready'],
        ['Boltz WebSocket', 'internal/boltz/ws.go', 'Production-Ready'],
        ['Status Normalization', 'internal/boltz/status.go', 'Production-Ready'],
        ['Types (MinerFeesAny)', 'internal/boltz/types.go', 'Production-Ready'],
        ['Errors', 'internal/boltz/errors.go', 'Production-Ready'],
        ['Provider Interface', 'internal/boltz/boltz.go', 'Production-Ready'],
        ['Swap Engine', 'internal/swap/engine.go', 'Base Implemented'],
        ['Database Layer', 'internal/db/db.go', 'Implemented'],
        ['LND Adapter', 'internal/adapters/lnd/', 'Next Sprint'],
        ['Liquid Adapter', 'internal/adapters/liquid/', 'Next Sprint'],
    ]
    story.append(create_table(impl_status, [1.8*inch, 2.5*inch, 1.7*inch]))
    story.append(Paragraph("Table 8.1: Implementation Status", styles['Caption']))
    
    story.append(Paragraph("8.1 Test Results", styles['H2']))
    
    story.append(Paragraph("""
    All Boltz Client tests passing:
    """, styles['Normal']))
    
    test_code = """
    $ go test -v ./internal/boltz/...
    
    === RUN   TestMinerFeesAny_UnmarshalNumber
    --- PASS: TestMinerFeesAny_UnmarshalNumber (0.00s)
    === RUN   TestMinerFeesAny_UnmarshalObject
    --- PASS: TestMinerFeesAny_UnmarshalObject (0.00s)
    === RUN   TestNormalizeSubmarine_ClaimPending
    --- PASS: TestNormalizeSubmarine_ClaimPending (0.00s)
    === RUN   TestAllSubmarineStatuses
    --- PASS: TestAllSubmarineStatuses (0.00s)
    
    ok  github.com/xs-wallet/xscore/internal/boltz  0.563s
    """
    story.append(Preformatted(test_code, styles['CodeBlock']))
    story.append(Paragraph("Listing 8.1: Test Results", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 9. ROADMAP
    # =========================================================================
    story.append(Paragraph("9. Roadmap", styles['H1']))
    
    roadmap = [
        ['Phase', 'Duration', 'Deliverables', 'Status'],
        ['Boltz Client', 'Complete', 'HTTP, WebSocket, Status Normalization', '✓ Done'],
        ['Spike boltz-backend', 'Current', 'Responsibility Split, Integration Test', 'In Progress'],
        ['Adapters', '1 week', 'LND gRPC, elementsd JSON-RPC', 'Next'],
        ['Wiring', '1 week', 'xscore → boltz-backend, E2E tests', 'Pending'],
        ['Frontend', '2 weeks', 'Migrate BRLN devdash, Electron integration', 'Pending'],
    ]
    story.append(create_table(roadmap, [1.5*inch, 1*inch, 2.7*inch, 0.8*inch]))
    story.append(Paragraph("Table 9.1: Implementation Roadmap", styles['Caption']))
    
    story.append(Paragraph("9.1 Post-MVP Roadmap", styles['H2']))
    
    post_mvp = [
        ['Version', 'Target', 'Features'],
        ['v1.1', 'Q2 2026', 'Hardware wallet, SQLCipher, device secret'],
        ['v1.2', 'Q3 2026', 'Coinjoin, Payjoin, LNURL'],
        ['v2.0', 'Q4 2026', 'EVM Module (MetaMask + LI.FI) - isolated'],
    ]
    story.append(create_table(post_mvp, [1*inch, 1*inch, 4*inch]))
    story.append(Paragraph("Table 9.2: Post-MVP Roadmap", styles['Caption']))
    
    story.append(Paragraph("""
    <b>Note on EVM Module</b>: Per CTO direction, MetaMask + LI.FI integration will be 
    implemented as an isolated module, separate from the UTXO/LN core, to avoid contaminating 
    the atomic swap architecture.
    """, styles['Note']))
    
    # =========================================================================
    # BUILD
    # =========================================================================
    doc.build(story, onFirstPage=add_header_footer, onLaterPages=add_header_footer)
    print(f"✅ Technical Specification v2 generated: {OUTPUT_FILE}")

if __name__ == "__main__":
    create_pdf()
