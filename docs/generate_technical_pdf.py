"""
XS Wallet - Technical Specification Document Generator
Documento técnico denso, informativo, enterprise-grade
Design focado em clareza e legibilidade, não estética
"""

from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch, mm
from reportlab.lib.enums import TA_CENTER, TA_JUSTIFY, TA_LEFT
from reportlab.platypus import (SimpleDocTemplate, Paragraph, Spacer, PageBreak, 
                                Table, TableStyle, Preformatted, ListFlowable, ListItem)
from reportlab.lib import colors
from reportlab.platypus.tableofcontents import TableOfContents
from reportlab.pdfgen import canvas
from datetime import datetime

OUTPUT_FILE = r"c:\Users\windows10\Downloads\XS WALLET\docs\XS_Wallet_Technical_Specification.pdf"

# Design System - Foco em legibilidade
COLORS = {
    'text': colors.HexColor('#1a1a1a'),
    'text_secondary': colors.HexColor('#4a4a4a'),
    'heading': colors.HexColor('#0a0a0a'),
    'accent': colors.HexColor('#2563eb'),
    'border': colors.HexColor('#e5e5e5'),
    'bg_code': colors.HexColor('#f5f5f5'),
    'bg_table_header': colors.HexColor('#f0f0f0'),
}

def create_styles():
    """Estilos focados em legibilidade técnica"""
    styles = getSampleStyleSheet()
    
    # Texto base - legível
    styles['Normal'].fontSize = 10
    styles['Normal'].leading = 14
    styles['Normal'].textColor = COLORS['text']
    styles['Normal'].alignment = TA_JUSTIFY
    styles['Normal'].spaceAfter = 8
    
    # Headings hierárquicos
    styles.add(ParagraphStyle(
        'H1',
        parent=styles['Heading1'],
        fontSize=16,
        fontName='Helvetica-Bold',
        textColor=COLORS['heading'],
        spaceBefore=24,
        spaceAfter=12,
        borderWidth=0,
        borderPadding=0,
        borderColor=COLORS['border'],
        borderRadius=0,
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
    
    # Código
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
    
    # Nota técnica
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
    
    # Caption
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
    
    # Header
    canvas.setFont('Helvetica', 8)
    canvas.setFillColor(COLORS['text_secondary'])
    canvas.drawString(doc.leftMargin, doc.height + doc.topMargin - 10, 
                      "XS Wallet - Technical Specification v0.1.0")
    canvas.drawRightString(doc.width + doc.leftMargin, doc.height + doc.topMargin - 10,
                           "CONFIDENTIAL")
    
    # Footer
    canvas.drawString(doc.leftMargin, 20, 
                      f"Generated: {datetime.now().strftime('%Y-%m-%d %H:%M')}")
    canvas.drawRightString(doc.width + doc.leftMargin, 20, 
                           f"Page {doc.page}")
    
    # Linha separadora
    canvas.setStrokeColor(COLORS['border'])
    canvas.line(doc.leftMargin, doc.height + doc.topMargin - 15, 
                doc.width + doc.leftMargin, doc.height + doc.topMargin - 15)
    canvas.line(doc.leftMargin, 30, doc.width + doc.leftMargin, 30)
    
    canvas.restoreState()

def create_pdf():
    """Gera documento técnico completo"""
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
    
    story.append(Paragraph("Technical Specification Document", ParagraphStyle(
        'Subtitle', parent=styles['Normal'], fontSize=14, alignment=TA_CENTER, 
        textColor=COLORS['text_secondary'], spaceAfter=40
    )))
    
    story.append(Paragraph("HD Wallet + Atomic Swaps Desktop Application", ParagraphStyle(
        'Desc', parent=styles['Normal'], fontSize=12, alignment=TA_CENTER, spaceAfter=60
    )))
    
    # Metadados
    meta_data = [
        ['Document', 'Value'],
        ['Version', '0.1.0'],
        ['Status', 'Draft - Foundation Complete'],
        ['Date', 'January 2026'],
        ['Classification', 'Technical Specification'],
        ['Target Audience', 'Developers, Architects, Auditors'],
    ]
    story.append(create_table(meta_data, [2*inch, 3*inch]))
    
    story.append(PageBreak())
    
    # =========================================================================
    # ÍNDICE
    # =========================================================================
    story.append(Paragraph("Table of Contents", styles['H1']))
    story.append(Spacer(1, 12))
    
    toc_items = [
        "1. Executive Summary",
        "2. System Architecture",
        "   2.1 Component Overview",
        "   2.2 Process Model",
        "   2.3 Data Flow",
        "3. Database Specification",
        "   3.1 Schema Design",
        "   3.2 Table Definitions",
        "   3.3 Concurrency Model",
        "4. Atomic Swap Protocol",
        "   4.1 Submarine Swap",
        "   4.2 Reverse Swap",
        "   4.3 Chain Swap",
        "   4.4 State Machine",
        "5. Cryptographic Specifications",
        "   5.1 Key Derivation (BIP39/32/84/85)",
        "   5.2 Vault Encryption",
        "   5.3 Taproot + MuSig2",
        "6. Node Management",
        "   6.1 Lifecycle Management",
        "   6.2 Binary Verification",
        "7. Security Model",
        "8. API Specifications",
        "9. Implementation Roadmap",
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
    Liquid (L-BTC), and Lightning Network (LN) using Taproot and the Boltz API v2 as liquidity 
    provider. The system maintains true self-custody while providing seamless cross-chain and 
    cross-layer swaps without requiring users to operate their own swap infrastructure.
    """, styles['Normal']))
    
    story.append(Paragraph("1.1 Core Capabilities", styles['H2']))
    
    capabilities = [
        ['Capability', 'Implementation', 'Standard/Protocol'],
        ['HD Wallet', 'Hierarchical deterministic key derivation', 'BIP39, BIP32, BIP84, BIP85'],
        ['Atomic Swaps', 'Hash Time-Locked Contracts (HTLC)', 'Boltz API v2, Taproot P2TR'],
        ['MuSig2 Signing', 'Aggregated Schnorr signatures', 'BIP340, MuSig2 draft'],
        ['Encryption', 'Memory-hard KDF + authenticated encryption', 'Argon2id, AES-256-GCM'],
        ['Persistence', 'Embedded SQL database with WAL', 'SQLite 3.31+'],
        ['Node Integration', 'Embedded full nodes with verified binaries', 'Bitcoin Core, Elements, LND'],
    ]
    story.append(create_table(capabilities, [1.5*inch, 2.5*inch, 2*inch]))
    story.append(Paragraph("Table 1.1: Core Capabilities Matrix", styles['Caption']))
    
    story.append(Paragraph("1.2 Design Principles", styles['H2']))
    
    principles = """
    <b>Self-Custody</b>: User controls all private keys; no server holds funds or signing authority.<br/><br/>
    <b>Zero Trust</b>: All external data (Boltz responses, node data) is verified locally before use.<br/><br/>
    <b>Crash Recovery</b>: All swap states are persisted with idempotent operations; swaps can resume after any failure.<br/><br/>
    <b>Deterministic Restoration</b>: Pending swaps can be restored from mnemonic alone using BIP85 child seeds.
    """
    story.append(Paragraph(principles, styles['Normal']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 2. SYSTEM ARCHITECTURE
    # =========================================================================
    story.append(Paragraph("2. System Architecture", styles['H1']))
    
    story.append(Paragraph("2.1 Component Overview", styles['H2']))
    
    story.append(Paragraph("""
    The system is structured as a multi-process Electron application with crash isolation between 
    UI, backend logic, and embedded nodes. This architecture ensures that a crash in any component 
    does not affect the others, and swap operations can continue even if the UI restarts.
    """, styles['Normal']))
    
    # Diagrama ASCII
    arch_diagram = """
    ┌─────────────────────────────────────────────────────────────────────────┐
    │                         XS Wallet Desktop Application                    │
    ├─────────────────────────────────────────────────────────────────────────┤
    │                                                                          │
    │  ┌──────────────────┐              ┌──────────────────────────────────┐ │
    │  │  Electron Main   │◄────IPC─────►│     Backend Process (Node.js)    │ │
    │  │                  │              │                                   │ │
    │  │  • Window Mgmt   │              │  ┌─────────────────────────────┐ │ │
    │  │  • Auto-Updater  │              │  │      Swap Engine            │ │ │
    │  │  • Node Manager  │              │  │  • State Machine (CAS)      │ │ │
    │  │  • IPC Bridge    │              │  │  • Watchdog + Recovery      │ │ │
    │  └──────────────────┘              │  │  • Idempotent Operations    │ │ │
    │           │                        │  └─────────────────────────────┘ │ │
    │           │ spawn                  │                                   │ │
    │           ▼                        │  ┌─────────────────────────────┐ │ │
    │  ┌──────────────────┐              │  │      Database (SQLite)      │ │ │
    │  │   React UI       │◄────IPC─────►│  │  • WAL Mode                 │ │ │
    │  │   (Renderer)     │              │  │  • CAS/Version Control      │ │ │
    │  │                  │              │  │  • Event Log                │ │ │
    │  │  • Dashboard     │              │  └─────────────────────────────┘ │ │
    │  │  • Swap UI       │              │                                   │ │
    │  │  • Settings      │              │  ┌─────────────────────────────┐ │ │
    │  └──────────────────┘              │  │      Chain Adapters         │ │ │
    │                                    │  │  • BTC (bitcoind RPC)       │ │ │
    │  ┌──────────────────────────────┐  │  │  • Liquid (elementsd RPC)   │ │ │
    │  │    Embedded Nodes            │  │  │  • LN (LND gRPC)            │ │ │
    │  │  bitcoind │ elementsd │ LND  │◄─┼──│                             │ │ │
    │  └──────────────────────────────┘  │  └─────────────────────────────┘ │ │
    │                                    └──────────────────────────────────┘ │
    └─────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼ HTTPS/WSS
                              ┌─────────────────────┐
                              │    Boltz API v2     │
                              │  (Swap Provider)    │
                              └─────────────────────┘
    """
    
    story.append(Preformatted(arch_diagram, styles['CodeBlock']))
    story.append(Paragraph("Figure 2.1: System Architecture Diagram", styles['Caption']))
    
    story.append(Paragraph("2.2 Process Model", styles['H2']))
    
    processes = [
        ['Process', 'Role', 'Isolation Benefit'],
        ['Main (Electron)', 'Window management, IPC routing, node spawning', 'Core stability; manages lifecycle'],
        ['Backend (Node.js)', 'Swap engine, database, adapters, Boltz client', 'UI crash does not affect swaps'],
        ['Renderer (Chromium)', 'React UI, user interactions', 'Sandboxed; minimal privileges'],
        ['bitcoind', 'Bitcoin full node (RPC server)', 'Separate process; own data dir'],
        ['elementsd', 'Liquid/Elements node (RPC server)', 'Separate process; own data dir'],
        ['LND', 'Lightning Network daemon (gRPC server)', 'Separate process; macaroon auth'],
    ]
    story.append(create_table(processes, [1.3*inch, 2.5*inch, 2.2*inch]))
    story.append(Paragraph("Table 2.1: Process Model", styles['Caption']))
    
    story.append(Paragraph("2.3 Data Flow", styles['H2']))
    
    story.append(Paragraph("""
    All user-initiated operations flow through the IPC bridge to the backend process. The backend 
    maintains authoritative state in SQLite and coordinates with chain adapters and Boltz API. 
    Critical invariant: <b>WebSocket events from Boltz are triggers only; on-chain/LND state is 
    always verified before acting.</b>
    """, styles['Normal']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 3. DATABASE SPECIFICATION
    # =========================================================================
    story.append(Paragraph("3. Database Specification", styles['H1']))
    
    story.append(Paragraph("3.1 Schema Design", styles['H2']))
    
    story.append(Paragraph("""
    The database uses SQLite with Write-Ahead Logging (WAL) for optimal read/write concurrency. 
    All swap state mutations use Compare-And-Swap (CAS) with version-based optimistic locking to 
    prevent race conditions without blocking queries.
    """, styles['Normal']))
    
    # Pragmas
    story.append(Paragraph("3.1.1 Critical Pragmas", styles['H3']))
    
    pragmas = [
        ['Pragma', 'Value', 'Rationale'],
        ['journal_mode', 'WAL', 'Concurrent reads during writes; crash recovery'],
        ['synchronous', 'NORMAL', 'Balance durability/performance (safe with WAL)'],
        ['busy_timeout', '5000', 'Wait 5s for locks; prevents "database is locked"'],
        ['foreign_keys', 'ON', 'Enforce referential integrity'],
        ['temp_store', 'MEMORY', 'Temp tables in RAM for performance'],
        ['cache_size', '-20000', '~20MB page cache'],
    ]
    story.append(create_table(pragmas, [1.3*inch, 1.2*inch, 3.5*inch]))
    story.append(Paragraph("Table 3.1: SQLite Pragmas", styles['Caption']))
    
    story.append(Paragraph("3.2 Table Definitions", styles['H2']))
    
    # swaps table
    story.append(Paragraph("3.2.1 swaps - Authoritative Swap State", styles['H3']))
    
    swaps_cols = [
        ['Column', 'Type', 'Constraints', 'Description'],
        ['id', 'TEXT', 'PRIMARY KEY', 'Unique swap identifier (UUID)'],
        ['kind', 'TEXT', 'CHECK(IN submarine,reverse,chain)', 'Swap type'],
        ['env', 'TEXT', 'CHECK(IN regtest,testnet,mainnet)', 'Network environment'],
        ['version', 'INTEGER', 'NOT NULL DEFAULT 0', 'CAS version for optimistic locking'],
        ['state', 'TEXT', 'CHECK(IN open,locked,...)', 'Current state machine state'],
        ['locked_intent', 'TEXT (JSON)', 'NOT NULL when state != open', 'Immutable quote/fees snapshot'],
        ['swap_key_index', 'INTEGER', 'NOT NULL', 'BIP85 derivation index'],
        ['preimage_hash_hex', 'TEXT', 'CHECK(length=64)', 'SHA256 hash of preimage R'],
        ['claim_pubkey_hex', 'TEXT', 'CHECK(length IN 64,66)', 'User claim public key'],
        ['refund_pubkey_hex', 'TEXT', 'CHECK(length IN 64,66)', 'User refund public key'],
        ['musig_session_id', 'TEXT', '', 'MuSig2 session identifier'],
        ['lockup_txid', 'TEXT', 'CHECK(length=64)', 'Funding transaction ID'],
        ['lockup_amount_sat', 'TEXT', 'CHECK(GLOB [0-9]*)', 'Amount in satoshis (bigint as string)'],
        ['created_at', 'TEXT', 'DEFAULT ISO8601', 'Creation timestamp'],
        ['updated_at', 'TEXT', 'DEFAULT ISO8601', 'Last update timestamp'],
    ]
    story.append(create_table(swaps_cols, [1.3*inch, 1*inch, 1.5*inch, 2.2*inch]))
    story.append(Paragraph("Table 3.2: swaps Table (partial - 40+ columns total)", styles['Caption']))
    
    story.append(Paragraph("""
    <b>Invariant</b>: When state is not in (open, failed, canceled), the locked_intent field MUST 
    be non-null. This is enforced via CHECK constraint and ensures every active swap has an 
    immutable record of agreed parameters.
    """, styles['Note']))
    
    # Other tables summary
    story.append(Paragraph("3.2.2 Supporting Tables", styles['H3']))
    
    tables_summary = [
        ['Table', 'Primary Key', 'Purpose'],
        ['swap_events', 'seq (AUTOINCREMENT)', 'Immutable audit log; enables replay/debug'],
        ['swap_ops', '(swap_id, op_key)', 'Idempotent operation ledger; prevents duplicate broadcasts'],
        ['utxo_reservations', '(chain, txid, vout)', 'Prevents UTXO double-spend across swaps'],
        ['ln_reservations', 'payment_hash_hex', 'Prevents duplicate LN payments'],
        ['app_config', 'key', 'User configuration; snapshot captured in locked_intent'],
    ]
    story.append(create_table(tables_summary, [1.5*inch, 1.8*inch, 2.7*inch]))
    story.append(Paragraph("Table 3.3: Supporting Tables", styles['Caption']))
    
    story.append(Paragraph("3.3 Concurrency Model", styles['H2']))
    
    story.append(Paragraph("""
    All state transitions use Compare-And-Swap (CAS) with atomic transactions:
    """, styles['Normal']))
    
    cas_code = """
    -- CAS Update Pattern
    UPDATE swaps
    SET state = 'locked',
        version = version + 1,
        updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
        locked_intent = ?
    WHERE id = ? AND version = ? AND state = 'open'
    RETURNING version, state;
    
    -- If RETURNING is empty: version mismatch (concurrent modification)
    -- Application must reload and retry or abort
    """
    story.append(Preformatted(cas_code, styles['CodeBlock']))
    story.append(Paragraph("Listing 3.1: CAS Update Pattern", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 4. ATOMIC SWAP PROTOCOL
    # =========================================================================
    story.append(Paragraph("4. Atomic Swap Protocol", styles['H1']))
    
    story.append(Paragraph("4.1 Submarine Swap (On-chain → Lightning)", styles['H2']))
    
    story.append(Paragraph("""
    Submarine swaps convert on-chain funds (BTC or L-BTC) to Lightning capacity. The user locks 
    funds in a P2TR HTLC; Boltz pays the user's LN invoice and claims the HTLC using the revealed 
    preimage.
    """, styles['Normal']))
    
    sub_flow = """
    User                           Boltz                          Chain
      │                              │                              │
      │ 1. POST /v2/swap/submarine   │                              │
      │    {invoice, refundPubkey}   │                              │
      ├─────────────────────────────►│                              │
      │                              │                              │
      │ 2. Response:                 │                              │
      │    {address, swapTree,       │                              │
      │     claimPubkey, timeouts}   │                              │
      │◄─────────────────────────────┤                              │
      │                              │                              │
      │ 3. VERIFY: Rebuild P2TR      │                              │
      │    address from swapTree,    │                              │
      │    assert match              │                              │
      │                              │                              │
      │ 4. Fund HTLC                 │                              │
      ├──────────────────────────────┼─────────────────────────────►│
      │                              │                              │
      │                              │ 5. Detect funding            │
      │                              │◄─────────────────────────────┤
      │                              │                              │
      │                              │ 6. Pay LN invoice            │
      │                              │    (obtains preimage R)      │
      │                              │                              │
      │                              │ 7. Claim HTLC (reveal R)     │
      │                              ├─────────────────────────────►│
      │                              │                              │
      │ 8. WS: swap.update           │                              │
      │    {status: completed}       │                              │
      │◄─────────────────────────────┤                              │
    """
    story.append(Preformatted(sub_flow, styles['CodeBlock']))
    story.append(Paragraph("Figure 4.1: Submarine Swap Sequence", styles['Caption']))
    
    story.append(Paragraph("4.2 Reverse Swap (Lightning → On-chain)", styles['H2']))
    
    story.append(Paragraph("""
    Reverse swaps convert Lightning balance to on-chain funds. The user generates preimage R and 
    hash H=SHA256(R), pays a Boltz hold invoice, and claims the on-chain HTLC by revealing R.
    """, styles['Normal']))
    
    rev_flow = """
    User                           Boltz                          Chain
      │                              │                              │
      │ 1. Generate R, H=SHA256(R)   │                              │
      │                              │                              │
      │ 2. POST /v2/swap/reverse     │                              │
      │    {preimageHash, claimPub}  │                              │
      ├─────────────────────────────►│                              │
      │                              │                              │
      │ 3. Response:                 │                              │
      │    {invoice, lockupAddress,  │                              │
      │     swapTree, timeouts}      │                              │
      │◄─────────────────────────────┤                              │
      │                              │                              │
      │ 4. VERIFY: invoice hash == H │                              │
      │    VERIFY: P2TR address      │                              │
      │                              │                              │
      │ 5. Pay hold invoice          │                              │
      ├─────────────────────────────►│                              │
      │                              │                              │
      │                              │ 6. Fund on-chain HTLC        │
      │                              ├─────────────────────────────►│
      │                              │                              │
      │ 7. WS: lockup confirmed      │                              │
      │◄─────────────────────────────┤                              │
      │                              │                              │
      │ 8. Claim HTLC (reveal R)     │                              │
      ├──────────────────────────────┼─────────────────────────────►│
      │                              │                              │
      │                              │ 9. Settle invoice (use R)    │
      │                              │◄─────────────────────────────┤
    """
    story.append(Preformatted(rev_flow, styles['CodeBlock']))
    story.append(Paragraph("Figure 4.2: Reverse Swap Sequence", styles['Caption']))
    
    story.append(Paragraph("4.3 Chain Swap (BTC ↔ Liquid)", styles['H2']))
    
    story.append(Paragraph("""
    Chain swaps are atomic cross-chain swaps between Bitcoin mainchain and Liquid sidechain. 
    Timeouts are staggered (T_liquid < T_btc) to prevent race conditions.
    """, styles['Normal']))
    
    story.append(Paragraph("4.4 State Machine", styles['H2']))
    
    states = [
        ['State', 'Description', 'Transitions'],
        ['open', 'Initial state; quote accepted but not locked', 'locked | canceled'],
        ['locked', 'Parameters locked; intent immutable', 'commit_started | canceled'],
        ['commit_started', 'Funding transaction broadcast', 'waiting | failed'],
        ['waiting', 'Awaiting counterparty action', 'waiting_claim_details | refund_coop_waiting'],
        ['waiting_claim_details', 'Awaiting claim transaction details', 'signing_musig2_partial'],
        ['signing_musig2_partial', 'Creating MuSig2 partial signature', 'sent_partial_to_provider'],
        ['sent_partial_to_provider', 'Partial sig sent; awaiting broadcast', 'waiting_provider_broadcast'],
        ['waiting_provider_broadcast', 'Provider broadcasting claim tx', 'completed | fallback_script_ready'],
        ['refund_coop_waiting', 'Attempting cooperative refund', 'completed | fallback_script_ready'],
        ['fallback_script_ready', 'Preparing script-path claim/refund', 'refunding | completed'],
        ['refunding', 'Refund tx broadcast; awaiting confirmation', 'completed | failed'],
        ['completed', 'Terminal success state', '(terminal)'],
        ['failed', 'Terminal failure state', '(terminal)'],
        ['canceled', 'Canceled before funding', '(terminal)'],
    ]
    story.append(create_table(states, [1.6*inch, 2.5*inch, 2*inch]))
    story.append(Paragraph("Table 4.1: Swap State Machine", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 5. CRYPTOGRAPHIC SPECIFICATIONS
    # =========================================================================
    story.append(Paragraph("5. Cryptographic Specifications", styles['H1']))
    
    story.append(Paragraph("5.1 Key Derivation Hierarchy", styles['H2']))
    
    key_deriv = """
    Master Seed (BIP39 - 24 words)
    │
    ├── BIP32 Master Key (m)
    │   │
    │   ├── BIP84 (Native SegWit Wallet)
    │   │   └── m/84'/0'/0'/...  (regular wallet addresses)
    │   │
    │   └── BIP85 Child Seed (Swap Subtree)
    │       │
    │       └── m/0/index  (swap key pairs)
    │           │
    │           └── preimage R = SHA256(privateKey)
    │
    Restoration: Given mnemonic → derive swap keys → reconstruct pending swaps
    """
    story.append(Preformatted(key_deriv, styles['CodeBlock']))
    story.append(Paragraph("Figure 5.1: Key Derivation Hierarchy", styles['Caption']))
    
    story.append(Paragraph("5.2 Vault Encryption", styles['H2']))
    
    vault_spec = [
        ['Parameter', 'Value', 'Rationale'],
        ['KDF', 'Argon2id', 'Memory-hard; resistant to GPU/ASIC attacks'],
        ['Memory', '64 MB', 'Balance security/performance for desktop'],
        ['Iterations', '3', 'Standard recommendation'],
        ['Parallelism', '1', 'Single-threaded derivation'],
        ['Salt Length', '16 bytes', 'Random per-wallet'],
        ['Cipher', 'AES-256-GCM', 'Authenticated encryption'],
        ['IV Length', '12 bytes', 'GCM standard'],
        ['Tag Length', '16 bytes', 'Full authentication tag'],
    ]
    story.append(create_table(vault_spec, [1.5*inch, 1.5*inch, 3*inch]))
    story.append(Paragraph("Table 5.1: Vault Encryption Parameters", styles['Caption']))
    
    story.append(Paragraph("""
    <b>Offline Attack Mitigation</b>: Rate limiting (exponential backoff) protects runtime but not 
    offline database theft. For enhanced protection, derive final key from PIN + device secret 
    stored in OS keychain (Keychain/DPAPI/SecretService). MVP uses PIN-only; device secret is v1.1.
    """, styles['Note']))
    
    story.append(Paragraph("5.3 Taproot + MuSig2", styles['H2']))
    
    story.append(Paragraph("""
    Swap HTLCs use P2TR (Pay-to-Taproot) with:
    """, styles['Normal']))
    
    musig_spec = [
        ['Component', 'Specification'],
        ['Key Aggregation Order', 'Provider pubkey first, then user pubkey (deterministic)'],
        ['Nonce Generation', 'Deterministic from session ID + private key'],
        ['Session Persistence', 'musig_* fields in swaps table; survives restart'],
        ['Key-Path Spend', 'MuSig2 aggregated signature (cooperative claim/refund)'],
        ['Script-Path Fallback', 'HTLC conditions if cooperation fails (timeout-based)'],
    ]
    story.append(create_table(musig_spec, [2*inch, 4*inch]))
    story.append(Paragraph("Table 5.2: MuSig2 Specification", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 6. NODE MANAGEMENT
    # =========================================================================
    story.append(Paragraph("6. Node Management", styles['H1']))
    
    story.append(Paragraph("6.1 Lifecycle Management", styles['H2']))
    
    story.append(Paragraph("""
    The Node Manager handles binary verification, spawning, health monitoring, and graceful 
    shutdown of embedded nodes. Each node has isolated data directories per network.
    """, styles['Normal']))
    
    node_spec = [
        ['Node', 'Version', 'Data Directory', 'Health Check'],
        ['bitcoind', '26.0', '%APPDATA%/xs-wallet/nodes/btc/{network}/', 'getblockchaininfo RPC'],
        ['elementsd', '23.2.1', '%APPDATA%/xs-wallet/nodes/liquid/{network}/', 'getblockchaininfo RPC'],
        ['LND', '0.17.3', '%APPDATA%/xs-wallet/nodes/lnd/{network}/', 'getinfo gRPC'],
    ]
    story.append(create_table(node_spec, [1.2*inch, 1*inch, 2.5*inch, 1.5*inch]))
    story.append(Paragraph("Table 6.1: Node Specifications", styles['Caption']))
    
    story.append(Paragraph("6.2 Binary Verification", styles['H2']))
    
    story.append(Paragraph("""
    Binaries are downloaded on first run from official sources (GitHub Releases, bitcoincore.org) 
    and verified using SHA256 checksums from a signed manifest.
    """, styles['Normal']))
    
    story.append(Paragraph("""
    <b>Trust Chain</b>: Application embeds a pinned public key. On startup, downloads 
    nodes-manifest.json from GitHub Releases, verifies signature against pinned key, then 
    verifies each binary checksum against manifest. No central custody server; only public 
    distribution endpoints.
    """, styles['Note']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 7. SECURITY MODEL
    # =========================================================================
    story.append(Paragraph("7. Security Model", styles['H1']))
    
    story.append(Paragraph("7.1 Threat Model", styles['H2']))
    
    threats = [
        ['Threat', 'In Scope', 'Mitigation'],
        ['Seed theft (offline)', 'Yes', 'Argon2id + device secret (v1.1)'],
        ['PIN brute force (runtime)', 'Yes', 'Exponential backoff; lockout after 10 attempts'],
        ['Swap manipulation', 'Yes', 'Local script verification; never trust provider'],
        ['Supply chain attack', 'Yes', 'Signed manifest; SHA256 verification'],
        ['Physical device access', 'No (future)', 'Consider hardware wallet support'],
        ['OS compromise (root)', 'No', 'Out of scope; assume trusted OS'],
    ]
    story.append(create_table(threats, [1.8*inch, 0.8*inch, 3.4*inch]))
    story.append(Paragraph("Table 7.1: Threat Model", styles['Caption']))
    
    story.append(Paragraph("7.2 Cryptographic Verification", styles['H2']))
    
    verify_spec = [
        ['Verification', 'When', 'Failure Action'],
        ['P2TR address rebuild', 'Before funding HTLC', 'Abort swap; log error'],
        ['Invoice hash == preimage hash', 'Before paying hold invoice', 'Abort swap; log error'],
        ['Timeout sanity (T_liquid < T_btc)', 'Before locking', 'Abort swap; log error'],
        ['MuSig2 partial signature', 'Before sending to provider', 'Abort; attempt script-path'],
        ['On-chain confirmation', 'After claim/refund broadcast', 'Retry with higher fee'],
    ]
    story.append(create_table(verify_spec, [2*inch, 1.8*inch, 2.2*inch]))
    story.append(Paragraph("Table 7.2: Verification Points", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 8. API SPECIFICATIONS
    # =========================================================================
    story.append(Paragraph("8. API Specifications", styles['H1']))
    
    story.append(Paragraph("8.1 Boltz API v2 Endpoints", styles['H2']))
    
    boltz_api = [
        ['Endpoint', 'Method', 'Purpose'],
        ['/v2/swap/submarine', 'POST', 'Create submarine swap (on-chain → LN)'],
        ['/v2/swap/reverse', 'POST', 'Create reverse swap (LN → on-chain)'],
        ['/v2/swap/chain', 'POST', 'Create chain swap (BTC ↔ Liquid)'],
        ['/v2/swap/{id}', 'GET', 'Get swap status'],
        ['/v2/swap/{id}/claim', 'POST', 'Submit claim transaction details'],
        ['/v2/swap/{id}/refund', 'POST', 'Submit refund transaction details'],
        ['/v2/ws', 'WebSocket', 'Real-time swap status updates'],
        ['/v2/nodes', 'GET', 'Get routing hints for LN payments'],
    ]
    story.append(create_table(boltz_api, [2*inch, 1*inch, 3*inch]))
    story.append(Paragraph("Table 8.1: Boltz API v2 Endpoints", styles['Caption']))
    
    story.append(Paragraph("8.2 Internal IPC API", styles['H2']))
    
    ipc_api = [
        ['Channel', 'Direction', 'Payload'],
        ['swap:create', 'Renderer → Backend', '{kind, fromAsset, toAsset, amount}'],
        ['swap:status', 'Backend → Renderer', '{swapId, state, details}'],
        ['wallet:balance', 'Renderer → Backend', '{chain: btc|liquid|ln}'],
        ['wallet:balance:response', 'Backend → Renderer', '{confirmed, unconfirmed, pending}'],
        ['node:status', 'Backend → Renderer', '{bitcoind, elementsd, lnd: running|stopped|syncing}'],
    ]
    story.append(create_table(ipc_api, [2*inch, 1.3*inch, 2.7*inch]))
    story.append(Paragraph("Table 8.2: IPC API Channels", styles['Caption']))
    
    story.append(PageBreak())
    
    # =========================================================================
    # 9. IMPLEMENTATION ROADMAP
    # =========================================================================
    story.append(Paragraph("9. Implementation Roadmap", styles['H1']))
    
    roadmap = [
        ['Phase', 'Duration', 'Deliverables', 'Status'],
        ['Foundation', 'Complete', 'Schema, DB Layer, Node Manager, Decisions', 'Done'],
        ['Backend Core', '2 weeks', 'Key Vault, Adapters, Boltz Client, Swap Engine', 'Next'],
        ['Frontend', '1.5 weeks', 'React UI, Onboarding, Dashboard, Swap Interface', 'Pending'],
        ['Electron', '1 week', 'Main Process, IPC, Auto-update, Deep Links', 'Pending'],
        ['Packaging', '0.5 weeks', 'Installers, Code Signing, E2E Tests', 'Pending'],
    ]
    story.append(create_table(roadmap, [1.3*inch, 1*inch, 2.7*inch, 1*inch]))
    story.append(Paragraph("Table 9.1: Implementation Roadmap", styles['Caption']))
    
    story.append(Paragraph("9.1 Post-MVP Roadmap", styles['H2']))
    
    post_mvp = [
        ['Version', 'Target', 'Features'],
        ['v1.1', 'Q2 2026', 'Hardware wallet support, SQLCipher, device secret'],
        ['v1.2', 'Q3 2026', 'Coinjoin integration, Payjoin, LNURL'],
        ['v2.0', 'Q4 2026', 'RGB assets, Taproot Assets, DLC support'],
    ]
    story.append(create_table(post_mvp, [1*inch, 1*inch, 4*inch]))
    story.append(Paragraph("Table 9.2: Post-MVP Roadmap", styles['Caption']))
    
    # =========================================================================
    # BUILD
    # =========================================================================
    doc.build(story, onFirstPage=add_header_footer, onLaterPages=add_header_footer)
    print(f"✅ Technical Specification generated: {OUTPUT_FILE}")

if __name__ == "__main__":
    create_pdf()
