// Package boltz - WebSocket client para Boltz API v2
// Single-writer pattern, gap-recovery, reconnection com backoff
package boltz

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsReconnectDelay    = 2 * time.Second
	wsMaxReconnectDelay = 60 * time.Second
	wsPingInterval      = 30 * time.Second
	wsReadTimeout       = 60 * time.Second
)

// WSHandler é chamada quando um update é recebido
type WSHandler func(swapID, status string, tx *TxInfo)

// WSClient é o cliente WebSocket para Boltz
type WSClient struct {
	url        string
	restClient *Client // Para gap-recovery

	mu         sync.RWMutex
	conn       *websocket.Conn
	connCtx    context.Context    // Contexto da conexão atual
	connCancel context.CancelFunc // Para cancelar loops da conexão atual
	subscribed map[string]bool
	handlers   map[string]WSHandler

	ctx    context.Context
	cancel context.CancelFunc

	// Flags de estado
	running      bool
	reconnecting bool

	// Single-writer (Correção #7)
	writeMu sync.Mutex
}

// NewWSClient cria um novo cliente WebSocket
func NewWSClient(wsURL string, restClient *Client) *WSClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &WSClient{
		url:        wsURL,
		restClient: restClient,
		subscribed: make(map[string]bool),
		handlers:   make(map[string]WSHandler),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Connect estabelece conexão WebSocket
func (ws *WSClient) Connect() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Cancelar loops da conexão anterior se existir
	if ws.connCancel != nil {
		ws.connCancel()
	}

	conn, _, err := websocket.DefaultDialer.Dial(ws.url, nil)
	if err != nil {
		return err
	}
	ws.conn = conn

	// Criar contexto para esta conexão específica (Correção Gap #2)
	ws.connCtx, ws.connCancel = context.WithCancel(ws.ctx)

	// Pong handler para estender deadline
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
		return nil
	})

	// Iniciar loops apenas se não estiverem rodando
	if !ws.running {
		ws.running = true
		go ws.readLoop()
		go ws.pingLoop()
	}

	// Resubscrever swaps ativos
	if len(ws.subscribed) > 0 {
		ids := make([]string, 0, len(ws.subscribed))
		for id := range ws.subscribed {
			ids = append(ids, id)
		}
		go ws.sendSubscribe(ids)
	}

	return nil
}

// Subscribe adiciona swap ID para monitorar
func (ws *WSClient) Subscribe(swapID string, handler WSHandler) error {
	ws.mu.Lock()
	ws.subscribed[swapID] = true
	ws.handlers[swapID] = handler
	ws.mu.Unlock()

	return ws.sendSubscribe([]string{swapID})
}

// Unsubscribe remove swap do monitoramento
func (ws *WSClient) Unsubscribe(swapID string) {
	ws.mu.Lock()
	delete(ws.subscribed, swapID)
	delete(ws.handlers, swapID)
	ws.mu.Unlock()
}

// sendSubscribe envia mensagem de subscribe com channel correto (Correção #2)
func (ws *WSClient) sendSubscribe(ids []string) error {
	ws.mu.RLock()
	conn := ws.conn
	ws.mu.RUnlock()

	if conn == nil {
		return ErrWebSocketClosed
	}

	msg := WSSubscribeRequest{
		Op:      "subscribe",
		Channel: "swap.update", // Correção #2: channel obrigatório
		Args:    ids,
	}

	// Single writer (Correção #7)
	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()
	return conn.WriteJSON(msg)
}

// readLoop lê mensagens do WebSocket
func (ws *WSClient) readLoop() {
	for {
		// Verificar se deve parar
		select {
		case <-ws.ctx.Done():
			return
		default:
		}

		ws.mu.RLock()
		conn := ws.conn
		connCtx := ws.connCtx
		ws.mu.RUnlock()

		if conn == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Verificar se conexão foi substituída
		select {
		case <-connCtx.Done():
			// Conexão foi substituída, continuar com nova
			continue
		default:
		}

		conn.SetReadDeadline(time.Now().Add(wsReadTimeout))

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("boltz ws read error: %v", err)
			ws.handleDisconnect()
			continue
		}

		// Parse message
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("boltz ws unmarshal error: %v", err)
			continue
		}

		// Processar updates
		if msg.Event == "update" && msg.Channel == "swap.update" {
			for _, arg := range msg.Args {
				ws.mu.RLock()
				handler := ws.handlers[arg.ID]
				ws.mu.RUnlock()

				if handler != nil {
					handler(arg.ID, arg.Status, arg.Transaction)
				}
			}
		}
	}
}

// pingLoop envia pings periódicos
func (ws *WSClient) pingLoop() {
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ws.ctx.Done():
			return
		case <-ticker.C:
			ws.mu.RLock()
			conn := ws.conn
			connCtx := ws.connCtx
			ws.mu.RUnlock()

			if conn == nil {
				continue
			}

			// Verificar se conexão é atual
			select {
			case <-connCtx.Done():
				continue
			default:
			}

			// Single writer (Correção #7)
			ws.writeMu.Lock()
			err := conn.WriteMessage(websocket.PingMessage, nil)
			ws.writeMu.Unlock()

			if err != nil {
				ws.handleDisconnect()
			}
		}
	}
}

// handleDisconnect gerencia desconexão e reconexão
func (ws *WSClient) handleDisconnect() {
	ws.mu.Lock()
	if ws.reconnecting {
		ws.mu.Unlock()
		return
	}
	ws.reconnecting = true

	// Cancelar loops da conexão atual
	if ws.connCancel != nil {
		ws.connCancel()
	}

	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}
	ws.mu.Unlock()

	// Gap recovery via REST
	ws.gapRecovery()

	// Reconectar com backoff
	delay := wsReconnectDelay
	for {
		select {
		case <-ws.ctx.Done():
			return
		case <-time.After(delay):
		}

		err := ws.Connect()
		if err != nil {
			log.Printf("boltz ws reconnect failed: %v", err)
			delay = delay * 2
			if delay > wsMaxReconnectDelay {
				delay = wsMaxReconnectDelay
			}
			continue
		}

		log.Println("boltz ws reconnected")
		ws.mu.Lock()
		ws.reconnecting = false
		ws.mu.Unlock()
		return
	}
}

// gapRecovery busca status via REST para todos os swaps subscritos
func (ws *WSClient) gapRecovery() {
	ws.mu.RLock()
	swaps := make([]string, 0, len(ws.subscribed))
	handlers := make(map[string]WSHandler)
	for id := range ws.subscribed {
		swaps = append(swaps, id)
		handlers[id] = ws.handlers[id]
	}
	ws.mu.RUnlock()

	for _, swapID := range swaps {
		status, err := ws.restClient.GetSwapStatus(context.Background(), swapID)
		if err != nil {
			log.Printf("boltz gap recovery error for %s: %v", swapID, err)
			continue
		}
		if handler := handlers[swapID]; handler != nil {
			handler(swapID, status.Status, status.Transaction)
		}
	}
}

// Close fecha o cliente WebSocket
func (ws *WSClient) Close() error {
	ws.cancel()

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.connCancel != nil {
		ws.connCancel()
	}

	if ws.conn != nil {
		return ws.conn.Close()
	}
	return nil
}

// IsConnected verifica se está conectado
func (ws *WSClient) IsConnected() bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.conn != nil && !ws.reconnecting
}
