package rpc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"golang.org/x/net/websocket"
)

const (
	web3SubscriptionPollInterval  = 100 * time.Millisecond
	web3SubscriptionWriteTimeout  = 5 * time.Second
	web3SubscriptionMaxCatchUp    = 1024
	web3SubscriptionMaxLogBatch   = 4096
	web3SubscriptionMaxPendingRun = 4096
	web3SubscriptionMaxPerConn    = 256
	web3SubscriptionIdleTimeout   = 2 * time.Minute
)

type web3Subscription struct {
	ID           string
	Type         string
	Address      string
	Filter       web3Filter
	LastHeight   uint64
	LastLogIndex int
	SeenPending  map[string]bool
	PendingFull  bool
}

type web3SubscriptionSession struct {
	conn     *websocket.Conn
	provider StatusProvider
	cfg      Config
	filters  *web3FilterStore
	ctx      context.Context
	cancel   context.CancelFunc
	send     func(any)
	sendMu   sync.Mutex
	mu       sync.Mutex
	subs     map[string]web3Subscription
}

func isWebSocketUpgrade(request *http.Request) bool {
	return strings.EqualFold(request.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(request.Header.Get("Connection")), "upgrade")
}

func handleWeb3WebSocket(writer http.ResponseWriter, request *http.Request, provider StatusProvider, cfg Config, filters *web3FilterStore) {
	server := websocket.Server{
		Handler: func(conn *websocket.Conn) {
			serveWeb3WebSocket(conn, request.Context(), provider, cfg, filters)
		},
	}
	server.ServeHTTP(writer, request)
}

func serveWeb3WebSocket(conn *websocket.Conn, parent context.Context, provider StatusProvider, cfg Config, filters *web3FilterStore) {
	ctx, cancel := context.WithCancel(parent)
	session := &web3SubscriptionSession{
		conn:     conn,
		provider: provider,
		cfg:      cfg,
		filters:  filters,
		ctx:      ctx,
		cancel:   cancel,
		subs:     make(map[string]web3Subscription),
	}
	defer cancel()
	go session.poll()
	for {
		if timeout := cfg.subscriptionIdleTimeout(); timeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(timeout))
		}
		var payload JSONRPCRequest
		if err := websocket.JSON.Receive(conn, &payload); err != nil {
			return
		}
		if cfg.subscriptionIdleTimeout() > 0 {
			_ = conn.SetReadDeadline(time.Time{})
		}
		session.handle(payload)
	}
}

func (session *web3SubscriptionSession) handle(payload JSONRPCRequest) {
	if payload.JSONRPC != "" && payload.JSONRPC != "2.0" {
		session.sendJSON(JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Error: &JSONRPCError{Code: -32600, Message: "invalid JSON-RPC version"}})
		return
	}
	switch payload.Method {
	case "eth_subscribe":
		id, rpcErr := session.subscribe(payload.Params)
		session.sendJSON(JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Result: id, Error: rpcErr})
	case "eth_unsubscribe":
		removed, rpcErr := session.unsubscribe(payload.Params)
		session.sendJSON(JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Result: removed, Error: rpcErr})
	default:
		result, rpcErr := executeWeb3Method(session.ctx, session.provider, session.cfg, session.filters, payload.Method, payload.Params)
		session.sendJSON(JSONRPCResponse{JSONRPC: "2.0", ID: payload.ID, Result: result, Error: rpcErr})
	}
}

func (session *web3SubscriptionSession) subscribe(params []json.RawMessage) (string, *JSONRPCError) {
	if len(params) == 0 {
		return "", &JSONRPCError{Code: -32602, Message: "eth_subscribe requires subscription type"}
	}
	subscriptionType, err := jsonRPCStringParam(params[0])
	if err != nil {
		return "", &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	session.mu.Lock()
	if len(session.subs) >= session.cfg.subscriptionMaxPerConnection() {
		session.mu.Unlock()
		return "", &JSONRPCError{Code: -32005, Message: "web3 subscription limit exceeded"}
	}
	session.mu.Unlock()
	status := session.provider.Status(session.ctx)
	subscription := web3Subscription{
		ID:         randomSubscriptionID(),
		Type:       subscriptionType,
		LastHeight: uint64(status.LatestHeight),
	}
	switch subscriptionType {
	case "newHeads":
	case "logs":
		filter, rpcErr := web3LogFilterParam(session.ctx, session.provider, session.cfg, params[1:])
		if rpcErr != nil {
			return "", rpcErr
		}
		subscription.Filter = filter
		if logs, rpcErr := web3LogsForFilter(session.ctx, session.provider, session.cfg, filter); rpcErr == nil {
			subscription.LastLogIndex = len(logs)
		}
	case "newPendingTransactions":
		pendingProvider, ok := session.provider.(PendingTxProvider)
		if !ok {
			return "", &JSONRPCError{Code: -32000, Message: "pending transaction query is unavailable"}
		}
		if len(params) > 1 {
			var full bool
			if err := json.Unmarshal(params[1], &full); err != nil {
				return "", &JSONRPCError{Code: -32602, Message: "newPendingTransactions full transaction flag must be boolean"}
			}
			subscription.PendingFull = full
		}
		hashes, err := pendingProvider.PendingTxHashes(session.ctx)
		if err != nil {
			return "", &JSONRPCError{Code: -32000, Message: err.Error()}
		}
		subscription.SeenPending = make(map[string]bool, len(hashes))
		for _, hash := range hashes {
			subscription.SeenPending[web3HashString(hash)] = true
		}
	default:
		return "", &JSONRPCError{Code: -32602, Message: "unsupported subscription type"}
	}
	session.mu.Lock()
	session.subs[subscription.ID] = subscription
	session.mu.Unlock()
	return subscription.ID, nil
}

func (session *web3SubscriptionSession) unsubscribe(params []json.RawMessage) (bool, *JSONRPCError) {
	if len(params) != 1 {
		return false, &JSONRPCError{Code: -32602, Message: "eth_unsubscribe requires subscription id"}
	}
	id, err := jsonRPCStringParam(params[0])
	if err != nil {
		return false, &JSONRPCError{Code: -32602, Message: err.Error()}
	}
	session.mu.Lock()
	_, found := session.subs[id]
	delete(session.subs, id)
	session.mu.Unlock()
	return found, nil
}

func (session *web3SubscriptionSession) poll() {
	ticker := time.NewTicker(session.cfg.subscriptionPollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-session.ctx.Done():
			return
		case <-ticker.C:
			session.publish()
		}
	}
}

func (session *web3SubscriptionSession) publish() {
	session.mu.Lock()
	subscriptions := make([]web3Subscription, 0, len(session.subs))
	for _, subscription := range session.subs {
		subscriptions = append(subscriptions, subscription)
	}
	session.mu.Unlock()
	for _, subscription := range subscriptions {
		updated, keep := session.publishSubscription(subscription)
		if !keep {
			continue
		}
		session.mu.Lock()
		if _, found := session.subs[subscription.ID]; found {
			session.subs[subscription.ID] = updated
		}
		session.mu.Unlock()
	}
}

func (session *web3SubscriptionSession) publishSubscription(subscription web3Subscription) (web3Subscription, bool) {
	switch subscription.Type {
	case "newHeads":
		return session.publishHeads(subscription), true
	case "logs":
		return session.publishLogs(subscription), true
	case "newPendingTransactions":
		return session.publishPendingTransactions(subscription), true
	default:
		return subscription, false
	}
}

func (session *web3SubscriptionSession) publishHeads(subscription web3Subscription) web3Subscription {
	blockProvider, ok := session.provider.(BlockProvider)
	if !ok {
		return subscription
	}
	latest := uint64(session.provider.Status(session.ctx).LatestHeight)
	target := latest
	maxCatchUp := session.cfg.subscriptionMaxCatchUp()
	if target > subscription.LastHeight+maxCatchUp {
		target = subscription.LastHeight + maxCatchUp
	}
	for height := subscription.LastHeight + 1; height <= target; height++ {
		record, err := blockProvider.BlockByHeight(session.ctx, types.Height(height))
		if errors.Is(err, store.ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return subscription
		}
		header, ok := web3BlockHeader(session.ctx, session.provider, session.cfg, record)
		if !ok {
			return subscription
		}
		session.sendSubscription(subscription.ID, header)
		subscription.LastHeight = height
	}
	return subscription
}

func (session *web3SubscriptionSession) publishLogs(subscription web3Subscription) web3Subscription {
	logs, rpcErr := web3LogsForFilter(session.ctx, session.provider, session.cfg, subscription.Filter)
	if rpcErr != nil {
		return subscription
	}
	sent := 0
	for index := subscription.LastLogIndex; index < len(logs) && sent < session.cfg.subscriptionMaxLogBatch(); index++ {
		session.sendSubscription(subscription.ID, logs[index])
		subscription.LastLogIndex = index + 1
		sent++
	}
	subscription.LastHeight = uint64(session.provider.Status(session.ctx).LatestHeight)
	return subscription
}

func (session *web3SubscriptionSession) publishPendingTransactions(subscription web3Subscription) web3Subscription {
	pendingProvider, ok := session.provider.(PendingTxProvider)
	if !ok {
		return subscription
	}
	hashes, err := pendingProvider.PendingTxHashes(session.ctx)
	if err != nil {
		return subscription
	}
	if subscription.SeenPending == nil {
		subscription.SeenPending = make(map[string]bool, len(hashes))
	}
	live := make(map[string]bool, len(hashes))
	sent := 0
	for _, hash := range hashes {
		encoded := web3HashString(hash)
		if subscription.SeenPending[encoded] {
			live[encoded] = true
			continue
		}
		if sent >= session.cfg.subscriptionMaxPendingRun() {
			continue
		}
		live[encoded] = true
		result := any(encoded)
		if subscription.PendingFull {
			if tx, found, rpcErr := web3PendingTxByHash(session.ctx, session.provider, encoded); rpcErr == nil && found {
				result = web3PendingTransaction(tx)
			}
		}
		session.sendSubscription(subscription.ID, result)
		sent++
	}
	subscription.SeenPending = live
	return subscription
}

func (cfg Config) subscriptionPollInterval() time.Duration {
	if cfg.Web3SubscriptionInterval > 0 {
		return cfg.Web3SubscriptionInterval
	}
	return web3SubscriptionPollInterval
}

func (cfg Config) subscriptionMaxCatchUp() uint64 {
	if cfg.Web3SubscriptionMaxCatchUp > 0 {
		return cfg.Web3SubscriptionMaxCatchUp
	}
	return web3SubscriptionMaxCatchUp
}

func (cfg Config) subscriptionMaxLogBatch() int {
	if cfg.Web3SubscriptionMaxLogBatch > 0 {
		return cfg.Web3SubscriptionMaxLogBatch
	}
	return web3SubscriptionMaxLogBatch
}

func (cfg Config) subscriptionMaxPendingRun() int {
	if cfg.Web3SubscriptionMaxPendingRun > 0 {
		return cfg.Web3SubscriptionMaxPendingRun
	}
	return web3SubscriptionMaxPendingRun
}

func (cfg Config) subscriptionMaxPerConnection() int {
	if cfg.Web3SubscriptionMaxPerConn > 0 {
		return cfg.Web3SubscriptionMaxPerConn
	}
	return web3SubscriptionMaxPerConn
}

func (cfg Config) subscriptionIdleTimeout() time.Duration {
	if cfg.Web3SubscriptionIdleTimeout > 0 {
		return cfg.Web3SubscriptionIdleTimeout
	}
	return web3SubscriptionIdleTimeout
}

func (session *web3SubscriptionSession) sendSubscription(id string, result any) {
	session.sendJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  "eth_subscription",
		"params": map[string]any{
			"subscription": id,
			"result":       result,
		},
	})
}

func (session *web3SubscriptionSession) sendJSON(value any) {
	if session.send != nil {
		session.send(value)
		return
	}
	session.sendMu.Lock()
	defer session.sendMu.Unlock()
	if web3SubscriptionWriteTimeout > 0 {
		_ = session.conn.SetWriteDeadline(time.Now().Add(web3SubscriptionWriteTimeout))
		defer session.conn.SetWriteDeadline(time.Time{})
	}
	if err := websocket.JSON.Send(session.conn, value); err != nil {
		session.cancel()
	}
}

func web3HashString(hash types.Hash) string {
	return "0x" + hex.EncodeToString(hash[:])
}

func web3BlockHeader(ctx context.Context, provider StatusProvider, cfg Config, record store.BlockRecord) (map[string]any, bool) {
	stateRoot, ok := web3StateRoot(ctx, provider, cfg, record)
	if !ok {
		return nil, false
	}
	return map[string]any{
		"number":           hexQuantity(uint64(record.Block.Header.Height)),
		"hash":             web3HashString(record.Hash),
		"parentHash":       web3HashString(record.Block.Header.PreviousBlockHash),
		"nonce":            "0x0000000000000000",
		"sha3Uncles":       "0x0000000000000000000000000000000000000000000000000000000000000000",
		"logsBloom":        web3LogsBloom(record.Block.Txs, record.TxResults),
		"transactionsRoot": web3TransactionsRoot(record.Block.Txs),
		"stateRoot":        stateRoot,
		"receiptsRoot":     web3ReceiptsRoot(record.Block.Txs, record.TxResults),
		"miner":            "0x0000000000000000000000000000000000000000",
		"difficulty":       "0x0",
		"totalDifficulty":  "0x0",
		"extraData":        "0x",
		"gasLimit":         web3BlockGasLimit(record.TxResults),
		"gasUsed":          hexQuantity(web3BlockGasUsed(record.TxResults)),
		"timestamp":        hexQuantity(uint64(record.Block.Header.TimeUnixNano / int64(time.Second))),
	}, true
}

func web3LogArray(value any) []any {
	switch logs := value.(type) {
	case json.RawMessage:
		var items []any
		if err := json.Unmarshal(logs, &items); err != nil {
			return nil
		}
		return items
	case []any:
		return logs
	default:
		return nil
	}
}

func randomSubscriptionID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		sum := sha256.Sum256([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
		copy(bytes[:], sum[:len(bytes)])
	}
	return "0x" + hex.EncodeToString(bytes[:])
}
