"""
Gerador de PDF Moderno para XS Wallet - Web3 2026 Style
Design vibrante com gradientes, cores modernas e layout clean
"""

from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch, cm
from reportlab.lib.enums import TA_CENTER, TA_JUSTIFY, TA_LEFT, TA_RIGHT
from reportlab.platypus import (SimpleDocTemplate, Paragraph, Spacer, PageBreak, 
                                Table, TableStyle, Frame, PageTemplate, Image)
from reportlab.lib import colors
from reportlab.pdfgen import canvas
from reportlab.platypus.flowables import Flowable
from datetime import datetime

OUTPUT_FILE = r"c:\Users\windows10\Downloads\XS WALLET\docs\XS_Wallet_Documentacao_Tecnica.pdf"

# Cores Web3 modernas (gradientes Bitcoin/Lightning)
BITCOIN_ORANGE = colors.HexColor('#F7931A')
LIGHTNING_PURPLE = colors.HexColor('#7B3FF2')
LIQUID_TEAL = colors.HexColor('#00DACC')
DARK_BG = colors.HexColor('#0A0E27')
DARK_CARD = colors.HexColor('#1A1F3A')
TEXT_PRIMARY = colors.HexColor('#FFFFFF')
TEXT_SECONDARY = colors.HexColor('#A0AEC0')
ACCENT_GREEN = colors.HexColor('#00FF88')
ACCENT_BLUE = colors.HexColor('#00D4FF')

class GradientBox(Flowable):
    """Caixa com gradiente para destacar seções"""
    def __init__(self, width, height, color1, color2):
        Flowable.__init__(self)
        self.width = width
        self.height = height
        self.color1 = color1
        self.color2 = color2
    
    def draw(self):
        # Simula gradiente com múltiplas linhas
        steps = 50
        for i in range(steps):
            ratio = i / steps
            r = self.color1.red + (self.color2.red - self.color1.red) * ratio
            g = self.color1.green + (self.color2.green - self.color1.green) * ratio
            b = self.color1.blue + (self.color2.blue - self.color1.blue) * ratio
            self.canv.setFillColor(colors.Color(r, g, b))
            self.canv.rect(0, self.height - (i * self.height / steps), 
                          self.width, self.height / steps, fill=1, stroke=0)

def create_modern_cover(story, styles):
    """Capa moderna estilo Web3"""
    # Background gradient box
    story.append(GradientBox(6*inch, 10*inch, DARK_BG, DARK_CARD))
    story.append(Spacer(1, -9.5*inch))
    
    # Logo/Icon placeholder (você pode adicionar imagem real)
    icon_style = ParagraphStyle(
        'IconStyle',
        parent=styles['Normal'],
        fontSize=72,
        textColor=BITCOIN_ORANGE,
        alignment=TA_CENTER,
        fontName='Helvetica-Bold'
    )
    story.append(Spacer(1, 1.5*inch))
    story.append(Paragraph("⚡", icon_style))
    
    # Título principal
    title_style = ParagraphStyle(
        'ModernTitle',
        parent=styles['Heading1'],
        fontSize=42,
        textColor=TEXT_PRIMARY,
        spaceAfter=20,
        alignment=TA_CENTER,
        fontName='Helvetica-Bold',
        leading=50
    )
    
    subtitle_style = ParagraphStyle(
        'ModernSubtitle',
        parent=styles['Normal'],
        fontSize=18,
        textColor=ACCENT_BLUE,
        spaceAfter=60,
        alignment=TA_CENTER,
        fontName='Helvetica'
    )
    
    badge_style = ParagraphStyle(
        'BadgeStyle',
        parent=styles['Normal'],
        fontSize=11,
        textColor=ACCENT_GREEN,
        spaceAfter=8,
        alignment=TA_CENTER,
        fontName='Helvetica-Bold'
    )
    
    story.append(Paragraph("XS WALLET", title_style))
    story.append(Paragraph("HD Wallet + Atomic Swaps Desktop", subtitle_style))
    
    # Badges modernos
    story.append(Paragraph("🔐 SELF-CUSTODY  •  ⚡ LIGHTNING  •  🔗 TAPROOT", badge_style))
    story.append(Spacer(1, 1*inch))
    
    # Info box
    info_style = ParagraphStyle(
        'ModernInfo',
        parent=styles['Normal'],
        fontSize=13,
        textColor=TEXT_SECONDARY,
        spaceAfter=6,
        alignment=TA_CENTER,
        fontName='Helvetica'
    )
    
    story.append(Paragraph("Documentação Técnica v0.1.0", info_style))
    story.append(Paragraph("Janeiro 2026", info_style))
    story.append(Paragraph("Equipe XS Wallet", info_style))
    
    story.append(PageBreak())

def create_section_header(story, styles, title, subtitle="", icon=""):
    """Header de seção moderno com ícone"""
    # Gradient bar
    story.append(GradientBox(6*inch, 0.3*inch, BITCOIN_ORANGE, LIGHTNING_PURPLE))
    story.append(Spacer(1, 12))
    
    header_style = ParagraphStyle(
        'SectionHeader',
        parent=styles['Heading1'],
        fontSize=24,
        textColor=DARK_BG,
        spaceAfter=8,
        fontName='Helvetica-Bold'
    )
    
    subtitle_style = ParagraphStyle(
        'SectionSubtitle',
        parent=styles['Normal'],
        fontSize=13,
        textColor=TEXT_SECONDARY,
        spaceAfter=20,
        fontName='Helvetica'
    )
    
    story.append(Paragraph(f"{icon} {title}", header_style))
    if subtitle:
        story.append(Paragraph(subtitle, subtitle_style))

def create_feature_card(title, items, color):
    """Card moderno para features"""
    data = [[title]]
    for item in items:
        data.append([f"• {item}"])
    
    table = Table(data, colWidths=[5.5*inch])
    table.setStyle(TableStyle([
        ('BACKGROUND', (0, 0), (-1, 0), color),
        ('TEXTCOLOR', (0, 0), (-1, 0), colors.white),
        ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
        ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
        ('FONTSIZE', (0, 0), (-1, 0), 14),
        ('BOTTOMPADDING', (0, 0), (-1, 0), 12),
        ('TOPPADDING', (0, 0), (-1, 0), 12),
        ('BACKGROUND', (0, 1), (-1, -1), colors.HexColor('#F7FAFC')),
        ('FONTSIZE', (0, 1), (-1, -1), 11),
        ('LEFTPADDING', (0, 1), (-1, -1), 20),
        ('TOPPADDING', (0, 1), (-1, -1), 8),
        ('BOTTOMPADDING', (0, 1), (-1, -1), 8),
        ('BOX', (0, 0), (-1, -1), 2, color),
    ]))
    
    return table

def create_tech_table(data, header_color):
    """Tabela moderna para stack tecnológico"""
    table = Table(data, colWidths=[2*inch, 1.5*inch, 2.5*inch])
    table.setStyle(TableStyle([
        ('BACKGROUND', (0, 0), (-1, 0), header_color),
        ('TEXTCOLOR', (0, 0), (-1, 0), colors.white),
        ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
        ('FONTNAME', (0, 0), (-1, 0), 'Helvetica-Bold'),
        ('FONTSIZE', (0, 0), (-1, 0), 11),
        ('BOTTOMPADDING', (0, 0), (-1, 0), 10),
        ('TOPPADDING', (0, 0), (-1, 0), 10),
        ('BACKGROUND', (0, 1), (-1, -1), colors.white),
        ('ROWBACKGROUNDS', (0, 1), (-1, -1), [colors.HexColor('#F7FAFC'), colors.white]),
        ('FONTSIZE', (0, 1), (-1, -1), 10),
        ('TOPPADDING', (0, 1), (-1, -1), 8),
        ('BOTTOMPADDING', (0, 1), (-1, -1), 8),
        ('GRID', (0, 0), (-1, -1), 0.5, colors.HexColor('#E2E8F0')),
        ('BOX', (0, 0), (-1, -1), 2, header_color),
    ]))
    
    return table

def create_pdf():
    """Gera PDF moderno"""
    doc = SimpleDocTemplate(
        OUTPUT_FILE,
        pagesize=A4,
        rightMargin=1*inch,
        leftMargin=1*inch,
        topMargin=0.75*inch,
        bottomMargin=0.75*inch
    )
    
    story = []
    styles = getSampleStyleSheet()
    
    # Customizar estilos base
    styles['Normal'].fontSize = 11
    styles['Normal'].leading = 16
    styles['Normal'].textColor = colors.HexColor('#2D3748')
    
    # Capa moderna
    create_modern_cover(story, styles)
    
    # Sumário Executivo
    create_section_header(story, styles, "Sumário Executivo", 
                         "Visão geral do projeto XS Wallet", "📋")
    
    exec_text = """<b>XS Wallet</b> é uma aplicação desktop self-custody que permite atomic swaps 
    entre Bitcoin, Liquid e Lightning Network usando Taproot e a API Boltz v2 (REST + WebSocket) 
    com reconstrução local de scripts/taptree e enforcement de claim/refund via script-path."""
    
    story.append(Paragraph(exec_text, styles['Normal']))
    story.append(Spacer(1, 20))
    
    # Features em cards
    features_card = create_feature_card("Características Principais", [
        "Carteira HD compatível com BIP39/32/84/85",
        "Suporte Multi-Chain: Bitcoin, Liquid, Lightning",
        "Atomic Swaps via Boltz API v2 com verificação local",
        "Integração Taproot com MuSig2 + fallback script-path",
        "Auto-custódia total com criptografia Argon2id + AES-256-GCM",
        "Aplicação Electron com nodes embarcados"
    ], BITCOIN_ORANGE)
    
    story.append(features_card)
    story.append(Spacer(1, 20))
    
    # Status
    status_card = create_feature_card("Status do Projeto", [
        "Fundação do storage/infra completa (15%)",
        "Cronograma: 5 semanas até MVP",
        "Database layer production-ready (SQLite + WAL)",
        "Node manager com downloads verificáveis",
        "8 decisões técnicas críticas documentadas"
    ], LIGHTNING_PURPLE)
    
    story.append(status_card)
    story.append(PageBreak())
    
    # Arquitetura
    create_section_header(story, styles, "Arquitetura do Sistema",
                         "Componentes e fluxo de dados", "🏗️")
    
    arch_text = """O XS Wallet é estruturado em múltiplas camadas para garantir separação de 
    responsabilidades, isolamento de falhas e manutenibilidade. O backend roda em processo Node.js 
    separado para crash isolation."""
    
    story.append(Paragraph(arch_text, styles['Normal']))
    story.append(Spacer(1, 15))
    
    components_card = create_feature_card("Componentes Principais", [
        "Processo Main (Electron): Janelas, Auto-update, Node Manager, IPC",
        "Backend Process: Swap Engine, Database, Adapters, Boltz Client",
        "Frontend React: Onboarding, Dashboard, Swap UI, Settings",
        "Nodes Embarcados: bitcoind, elementsd, LND (download verificável)"
    ], LIQUID_TEAL)
    
    story.append(components_card)
    story.append(Spacer(1, 15))
    
    # Node Lifecycle
    lifecycle_text = """<b>Node Lifecycle</b>: O Node Manager cria datadirs isolados por rede 
    (regtest/testnet/mainnet), executa healthchecks RPC/gRPC, e implementa restart com backoff 
    exponencial em caso de falha. Detecção de versão incompatível de chainstate/wallets ao 
    atualizar nodes, com migração automática quando possível."""
    
    story.append(Paragraph(lifecycle_text, styles['Normal']))
    story.append(PageBreak())
    
    # Banco de Dados
    create_section_header(story, styles, "Banco de Dados",
                         "SQLite com WAL mode para concorrência", "💾")
    
    db_tables = [
        ['Tabela', 'Propósito', 'Chave Primária'],
        ['swaps', 'Estado autoritativo (CAS)', 'id (TEXT)'],
        ['swap_events', 'Trilha de auditoria', 'seq (AUTOINCREMENT)'],
        ['swap_ops', 'Ledger idempotência', '(swap_id, op_key)'],
        ['utxo_reservations', 'Anti double-spend', '(chain, txid, vout)'],
        ['ln_reservations', 'Anti duplicação LN', 'payment_hash_hex'],
        ['app_config', 'Configurações', 'key']
    ]
    
    story.append(create_tech_table(db_tables, DARK_CARD))
    story.append(Spacer(1, 15))
    
    pragmas_card = create_feature_card("Pragmas Críticos SQLite", [
        "journal_mode = WAL (concorrência)",
        "synchronous = NORMAL (balanço segurança/performance)",
        "busy_timeout = 5000 (espera 5s em lock)",
        "foreign_keys = ON (integridade referencial)",
        "temp_store = MEMORY (tabelas temp em RAM)",
        "cache_size = -20000 (~20MB cache)"
    ], ACCENT_BLUE)
    
    story.append(pragmas_card)
    story.append(PageBreak())
    
    # Decisões Técnicas
    create_section_header(story, styles, "Decisões Técnicas Críticas",
                         "8 decisões arquiteturais fundamentais", "⚙️")
    
    decisions = [
        ("1. SQLite com WAL", "Banco embarcado com excelente concorrência, sem servidor externo"),
        ("2. Backend Separado", "Processo Node.js isolado para crash isolation e recursos"),
        ("3. Downloads Verificáveis", "Instalador ~50MB com nodes baixados no primeiro uso (SHA256 + manifest assinado)"),
        ("4. API Boltz", "Provider externo com verificação local de scripts HTLC"),
        ("5. Taproot + MuSig2", "Key-path eficiente com fallback script-path + persistência de sessão"),
        ("6. PIN + Argon2id", "Criptografia user-friendly com device secret opcional (v1.1)"),
        ("7. BIP85 Swap Keys", "Child seed separado para recovery de swaps pendentes"),
        ("8. Config Management", "Tabela app_config com snapshot em cada swap")
    ]
    
    for title, desc in decisions:
        decision_text = f"<b>{title}</b>: {desc}"
        story.append(Paragraph(decision_text, styles['Normal']))
        story.append(Spacer(1, 10))
    
    story.append(Spacer(1, 10))
    
    # Security note
    security_note = """<b>Raiz de Confiança</b>: Os checksums são obtidos de um manifest assinado; 
    a aplicação valida a assinatura com chave pública fixa (pinned) embutida no código. Sem servidor 
    central de custódia/execução de swaps; apenas endpoints públicos de distribuição (GitHub Releases)."""
    
    story.append(Paragraph(security_note, styles['Normal']))
    story.append(PageBreak())
    
    # Stack Tecnológico
    create_section_header(story, styles, "Stack Tecnológico",
                         "Tecnologias modernas para 2026", "🚀")
    
    tech_data = [
        ['Componente', 'Tecnologia', 'Versão'],
        ['Desktop', 'Electron', '28+'],
        ['Frontend', 'React + Vite', '18 + 5'],
        ['Linguagem', 'TypeScript', '5.3'],
        ['Database', 'SQLite + better-sqlite3', '3.31+ + 9.2'],
        ['Bitcoin', 'bitcoinjs-lib + Taproot', '6.1'],
        ['HD Wallet', 'bip32 + bip39', '4.0 + 3.1'],
        ['Crypto', 'argon2 + AES-GCM', '0.31'],
        ['LN Client', '@grpc/grpc-js', '1.9']
    ]
    
    story.append(create_tech_table(tech_data, LIGHTNING_PURPLE))
    story.append(Spacer(1, 15))
    
    # LN↔BTC note
    ln_note = """<b>Nota sobre Swaps LN↔BTC</b>: O MVP suporta Submarine (Liquid→LN) e Reverse 
    (LN→Liquid). Swaps diretos LN↔BTC (sem Liquid) são suportados via Submarine/Reverse na chain 
    BTC se o par estiver disponível na API Boltz v2. Chain Swap (BTC↔Liquid) é implementado como 
    swap atômico cross-chain."""
    
    story.append(Paragraph(ln_note, styles['Normal']))
    story.append(PageBreak())
    
    # Roadmap
    create_section_header(story, styles, "Roadmap de Implementação",
                         "5 semanas até MVP completo", "📅")
    
    roadmap_data = [
        ['Fase', 'Duração', 'Entregas'],
        ['Backend Core', '2 semanas', 'Vault, Adapters, Boltz Client, Engine'],
        ['Frontend', '1.5 semanas', 'React UI, Onboarding, Dashboard, Swaps'],
        ['Electron', '1 semana', 'Main Process, IPC, Auto-update'],
        ['Packaging', '0.5 semana', 'Instaladores, Signing, Testes E2E']
    ]
    
    story.append(create_tech_table(roadmap_data, BITCOIN_ORANGE))
    story.append(Spacer(1, 20))
    
    # Status atual
    status_final = create_feature_card("Status Atual (15% Completo)", [
        "✅ Schema SQLite production-ready",
        "✅ DB Layer com CAS/transações",
        "✅ Node Manager com downloads verificáveis",
        "✅ 8 decisões técnicas documentadas",
        "🚧 Key Vault (próximo)",
        "🚧 Chain Adapters (próximo)",
        "🚧 Boltz Client + MuSig2 (próximo)"
    ], ACCENT_GREEN)
    
    story.append(status_final)
    story.append(Spacer(1, 30))
    
    # Footer
    footer_style = ParagraphStyle(
        'Footer',
        parent=styles['Normal'],
        fontSize=10,
        textColor=TEXT_SECONDARY,
        alignment=TA_CENTER
    )
    
    story.append(Paragraph("XS Wallet v0.1.0 • Janeiro 2026 • Fundação Completa", footer_style))
    
    # Gerar PDF
    doc.build(story)
    print(f"✅ PDF moderno gerado: {OUTPUT_FILE}")

if __name__ == "__main__":
    create_pdf()
