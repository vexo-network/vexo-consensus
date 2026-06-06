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

const web3SubscriptionPollInterval = 100 * time.Millisecond

type web3Subscription struct {
	ID           string
	Type         string
	Address      string
	Filter       web3Filter
	LastHeight   uint64
	LastLogIndex int
	SeenPending  map[string]bool
}

type web3SubscriptionSession struct {
	conn     *websocket.Conn
	provider StatusProvider
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

func handleWeb3WebSocket(writer http.ResponseWriter, request *http.Request, provider StatusProvider, filters *web3FilterStore) {
	server := websocket.Server{
		Handler: func(conn *websocket.Conn) {
			serveWeb3WebSocket(conn, request.Context(), provider, filters)
		},
	}
	server.ServeHTTP(writer, request)
}

func serveWeb3WebSocket(conn *websocket.Conn, parent context.Context, provider StatusProvider, filters *web3FilterStore) {
	ctx, cancel := context.WithCancel(parent)
	session := &web3SubscriptionSession{
		conn:     conn,
		provider: provider,
		filters:  filters,
		ctx:      ctx,
		cancel:   cancel,
		subs:     make(map[string]web3Subscription),
	}
	defer cancel()
	go session.poll()
	for {
		var payload JSONRPCRequest
		if err := websocket.JSON.Receive(conn, &payload); err != nil {
			return
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
		result, rpcErr := executeWeb3Method(session.ctx, session.provider, session.filters, payload.Method, payload.Params)
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
	status := session.provider.Status(session.ctx)
	subscription := web3Subscription{
		ID:         randomSubscriptionID(),
		Type:       subscriptionType,
		LastHeight: uint64(status.LatestHeight),
	}
	switch subscriptionType {
	case "newHeads":
	case "logs":
		filter, rpcErr := web3LogFilterParam(session.ctx, session.provider, params[1:])
		if rpcErr != nil {
			return "", rpcErr
		}
		subscription.Filter = filter
		if logs, rpcErr := web3LogsForFilter(session.ctx, session.provider, filter); rpcErr == nil {
			subscription.LastLogIndex = len(logs)
		}
	case "newPendingTransactions":
		pendingProvider, ok := session.provider.(PendingTxProvider)
		if !ok {
			return "", &JSONRPCError{Code: -32000, Message: "pending transaction query is unavailable"}
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
	ticker := time.NewTicker(web3SubscriptionPollInterval)
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
	for height := subscription.LastHeight + 1; height <= latest; height++ {
		record, err := blockProvider.BlockByHeight(session.ctx, types.Height(height))
		if errors.Is(err, store.ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return subscription
		}
		session.sendSubscription(subscription.ID, web3BlockHeader(record))
		subscription.LastHeight = height
	}
	return subscription
}

func (session *web3SubscriptionSession) publishLogs(subscription web3Subscription) web3Subscription {
	logs, rpcErr := web3LogsForFilter(session.ctx, session.provider, subscription.Filter)
	if rpcErr != nil {
		return subscription
	}
	for index := subscription.LastLogIndex; index < len(logs); index++ {
		session.sendSubscription(subscription.ID, logs[index])
	}
	subscription.LastLogIndex = len(logs)
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
	for _, hash := range hashes {
		encoded := web3HashString(hash)
		live[encoded] = true
		if subscription.SeenPending[encoded] {
			continue
		}
		session.sendSubscription(subscription.ID, encoded)
	}
	subscription.SeenPending = live
	return subscription
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
	_ = websocket.JSON.Send(session.conn, value)
}

func web3HashString(hash types.Hash) string {
	return "0x" + hex.EncodeToString(hash[:])
}

func web3BlockHeader(record store.BlockRecord) map[string]any {
	return map[string]any{
		"number":           hexQuantity(uint64(record.Block.Header.Height)),
		"hash":             web3HashString(record.Hash),
		"parentHash":       web3HashString(record.Block.Header.PreviousBlockHash),
		"nonce":            "0x0000000000000000",
		"sha3Uncles":       "0x0000000000000000000000000000000000000000000000000000000000000000",
		"logsBloom":        "0x" + strings.Repeat("00", 256),
		"transactionsRoot": web3TransactionsRoot(record.Block.Txs),
		"stateRoot":        "0x" + hex.EncodeToString(record.AppHash[:]),
		"receiptsRoot":     web3ReceiptsRoot(record.Block.Txs, record.TxResults),
		"miner":            "0x0000000000000000000000000000000000000000",
		"difficulty":       "0x0",
		"totalDifficulty":  "0x0",
		"extraData":        "0x",
		"gasLimit":         "0x0",
		"gasUsed":          hexQuantity(web3BlockGasUsed(record.TxResults)),
		"timestamp":        hexQuantity(uint64(record.Block.Header.TimeUnixNano / int64(time.Second))),
	}
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
