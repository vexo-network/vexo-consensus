package app

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"math/big"
	"strings"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInsufficientFee        = errors.New("insufficient transaction fee")
	ErrInsufficientFeeBalance = errors.New("insufficient fee balance")
	ErrInvalidGas             = errors.New("invalid transaction gas")
	ErrMissingNonce           = errors.New("missing transaction nonce")
	ErrInvalidNonce           = errors.New("invalid transaction nonce")
)

type AnteConfig struct {
	MinFee        uint64
	BaseFee       uint64
	MinGas        uint64
	MaxGas        uint64
	RequireNonce  bool
	FeeCollector  string
	RequireSigned bool
}

type TxMeta struct {
	Signer   types.Address
	Nonce    uint64
	Fee      uint64
	FeeBig   *big.Int
	Gas      uint64
	HasNonce bool
}

type AnteHandler interface {
	CheckTx(ctx Context, tx types.Tx) error
	CheckBlock(ctx Context, txs []types.Tx) error
	BeforeTx(ctx Context, tx types.Tx) error
	AfterTx(ctx Context, tx types.Tx) error
	GasUsed(tx types.Tx) uint64
	GasLimit(tx types.Tx) uint64
	FeePaid(tx types.Tx) uint64
}

type AnteKeeper struct {
	config   AnteConfig
	accounts AccountKeeper
}

func NewAnteKeeper(config AnteConfig) *AnteKeeper {
	return &AnteKeeper{config: config, accounts: NewAccountKeeper()}
}

func ParseTxMeta(tx types.Tx) TxMeta {
	var meta TxMeta
	if signer, found := TxTag(tx, "signer"); found {
		meta.Signer = types.Address(signer)
	}
	if nonce, found := TxUintTag(tx, "nonce"); found {
		meta.Nonce = nonce
		meta.HasNonce = true
	}
	if fee, found := TxAmountBigTag(tx, "fee"); found {
		meta.FeeBig = fee
		if fee.IsUint64() {
			meta.Fee = fee.Uint64()
		} else {
			meta.Fee = math.MaxUint64
		}
	}
	if gas, found := TxUintTag(tx, "gas"); found {
		meta.Gas = gas
	} else if gasLimit, found := TxUintTag(tx, "gas_limit"); found {
		meta.Gas = gasLimit
	}
	return meta
}

func (keeper AnteKeeper) CheckTx(ctx Context, tx types.Tx) error {
	if err := keeper.verifySignature(ctx, tx); err != nil {
		return err
	}
	meta := ParseTxMeta(tx)
	if err := keeper.validateMetaForTx(tx, meta); err != nil {
		return err
	}
	if ctx.Store == nil || meta.Signer == "" || !meta.HasNonce {
		return nil
	}
	expected, err := keeper.expectedSequence(ctx.GoContext(), ctx.Store, meta.Signer, tx)
	if err != nil {
		return err
	}
	if meta.Nonce != expected {
		return ErrInvalidNonce
	}
	return nil
}

func (keeper AnteKeeper) CheckBlock(ctx Context, txs []types.Tx) error {
	nextBySigner := make(map[types.Address]uint64)
	for _, tx := range txs {
		if err := keeper.verifySignature(ctx, tx); err != nil {
			return err
		}
		meta := ParseTxMeta(tx)
		if err := keeper.validateMetaForTx(tx, meta); err != nil {
			return err
		}
		if ctx.Store == nil || meta.Signer == "" || !meta.HasNonce {
			continue
		}
		expected, found := nextBySigner[meta.Signer]
		if !found {
			var err error
			expected, err = keeper.expectedSequence(ctx.GoContext(), ctx.Store, meta.Signer, tx)
			if err != nil {
				return err
			}
		}
		if meta.Nonce != expected {
			return ErrInvalidNonce
		}
		nextBySigner[meta.Signer] = expected + 1
	}
	return nil
}

func (keeper AnteKeeper) BeforeTx(ctx Context, tx types.Tx) error {
	return keeper.CheckTx(ctx, tx)
}

func (keeper AnteKeeper) AfterTx(ctx Context, tx types.Tx) error {
	meta := ParseTxMeta(tx)
	if ctx.Store == nil {
		return nil
	}
	if !isEthereumRawTx(tx) {
		if err := keeper.collectFee(ctx.GoContext(), ctx.Store, meta); err != nil {
			return err
		}
	}
	if meta.Signer == "" || !meta.HasNonce {
		return nil
	}
	nextSequence := meta.Nonce
	if isEthereumRawTx(tx) {
		if meta.Nonce == math.MaxUint64 {
			return ErrInvalidNonce
		}
		nextSequence = meta.Nonce + 1
	}
	return keeper.accounts.SetSequence(ctx.GoContext(), ctx.Store, meta.Signer, nextSequence)
}

func (keeper AnteKeeper) GasUsed(tx types.Tx) uint64 {
	return keeper.GasLimit(tx)
}

func (keeper AnteKeeper) GasLimit(tx types.Tx) uint64 {
	meta := ParseTxMeta(tx)
	if meta.Gas > 0 {
		return meta.Gas
	}
	return uint64(len(tx))
}

func (keeper AnteKeeper) FeePaid(tx types.Tx) uint64 {
	fee := metaFeeBig(ParseTxMeta(tx))
	if !fee.IsUint64() {
		return math.MaxUint64
	}
	return fee.Uint64()
}

func (keeper *AnteKeeper) SetBaseFee(baseFee uint64) {
	if keeper == nil {
		return
	}
	keeper.config.BaseFee = baseFee
}

func (keeper AnteKeeper) validateMeta(meta TxMeta) error {
	fee := metaFeeBig(meta)
	if keeper.config.MinFee > 0 && fee.Cmp(new(big.Int).SetUint64(keeper.config.MinFee)) < 0 {
		return ErrInsufficientFee
	}
	if keeper.config.BaseFee > 0 {
		if meta.Gas == 0 {
			return ErrInvalidGas
		}
		requiredFee := multiplyGasPriceBig(meta.Gas, keeper.config.BaseFee)
		if fee.Cmp(requiredFee) < 0 {
			return ErrInsufficientFee
		}
	}
	if keeper.config.MinGas > 0 && meta.Gas < keeper.config.MinGas {
		return ErrInvalidGas
	}
	if keeper.config.MaxGas > 0 && meta.Gas > keeper.config.MaxGas {
		return ErrInvalidGas
	}
	if keeper.config.RequireNonce && (meta.Signer == "" || !meta.HasNonce) {
		return ErrMissingNonce
	}
	return nil
}

func (keeper AnteKeeper) validateMetaForTx(tx types.Tx, meta TxMeta) error {
	if !isEthereumRawTx(tx) {
		return keeper.validateMeta(meta)
	}
	if meta.Gas == 0 {
		return ErrInvalidGas
	}
	if keeper.config.MinGas > 0 && meta.Gas < keeper.config.MinGas {
		return ErrInvalidGas
	}
	if keeper.config.MaxGas > 0 && meta.Gas > keeper.config.MaxGas {
		return ErrInvalidGas
	}
	if keeper.config.RequireNonce && (meta.Signer == "" || !meta.HasNonce) {
		return ErrMissingNonce
	}
	return nil
}

func multiplyGasPrice(gas uint64, baseFee uint64) (uint64, bool) {
	if gas == 0 || baseFee == 0 {
		return 0, true
	}
	if gas > math.MaxUint64/baseFee {
		return 0, false
	}
	return gas * baseFee, true
}

func multiplyGasPriceBig(gas uint64, baseFee uint64) *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(gas), new(big.Int).SetUint64(baseFee))
}

func metaFeeBig(meta TxMeta) *big.Int {
	if meta.FeeBig == nil {
		return new(big.Int).SetUint64(meta.Fee)
	}
	return new(big.Int).Set(meta.FeeBig)
}

func (keeper AnteKeeper) collectFee(ctx context.Context, store StateStore, meta TxMeta) error {
	fee := metaFeeBig(meta)
	if fee.Sign() == 0 {
		return nil
	}
	if meta.Signer == "" {
		return ErrMissingNonce
	}
	collector := types.Address(keeper.config.FeeCollector)
	if collector == "" {
		collector = "fee_collector"
	}
	balance, err := bankBalanceBig(ctx, store, meta.Signer)
	if err != nil {
		return err
	}
	if balance.Cmp(fee) < 0 {
		return ErrInsufficientFeeBalance
	}
	collectorBalance, err := bankBalanceBig(ctx, store, collector)
	if err != nil {
		return err
	}
	if new(big.Int).Add(collectorBalance, fee).BitLen() > 256 {
		return ErrInsufficientFeeBalance
	}
	if err := setBankBalanceBig(ctx, store, meta.Signer, new(big.Int).Sub(balance, fee)); err != nil {
		return err
	}
	return setBankBalanceBig(ctx, store, collector, new(big.Int).Add(collectorBalance, fee))
}

func (keeper AnteKeeper) verifySignature(ctx Context, tx types.Tx) error {
	if !IsSignedTx(tx) {
		if _, found := TxTag(tx, "eth_raw"); found {
			if _, hashFound := TxTag(tx, "eth_hash"); hashFound {
				return nil
			}
		}
		if keeper.config.RequireSigned {
			return ErrInvalidSignedTx
		}
		return nil
	}
	return VerifySignedTx(ctx.ChainID, tx, keeper.signatureVerifier())
}

func (keeper AnteKeeper) signatureVerifier() vexocrypto.Signer {
	return vexocrypto.Ed25519Signer{}
}

func (keeper AnteKeeper) expectedSequence(ctx context.Context, store StateStore, signer types.Address, tx types.Tx) (uint64, error) {
	if isEthereumRawTx(tx) {
		return keeper.accounts.Sequence(ctx, store, signer)
	}
	return keeper.accounts.NextSequence(ctx, store, signer)
}

func isEthereumRawTx(tx types.Tx) bool {
	_, found := TxTag(tx, "eth_raw")
	return found
}

func bankBalance(ctx context.Context, store StateStore, address types.Address) (uint64, error) {
	balance, err := bankBalanceBig(ctx, store, address)
	if err != nil {
		return 0, err
	}
	if !balance.IsUint64() {
		return 0, ErrInsufficientFeeBalance
	}
	return balance.Uint64(), nil
}

func bankBalanceBig(ctx context.Context, store StateStore, address types.Address) (*big.Int, error) {
	value, err := store.Get(ctx, "bank", bankBalanceKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		value, err = store.Get(ctx, "bank", []byte(address))
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return new(big.Int), nil
		}
	}
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return new(big.Int), nil
	}
	if len(value) > 32 {
		return nil, ErrInsufficientFeeBalance
	}
	return new(big.Int).SetBytes(value), nil
}

func setBankBalance(ctx context.Context, store StateStore, address types.Address, balance uint64) error {
	return setBankBalanceBig(ctx, store, address, new(big.Int).SetUint64(balance))
}

func setBankBalanceBig(ctx context.Context, store StateStore, address types.Address, balance *big.Int) error {
	if balance == nil || balance.Sign() < 0 || balance.BitLen() > 256 {
		return ErrInsufficientFeeBalance
	}
	return store.Set(ctx, "bank", bankBalanceKey(address), encodeBankBalanceBig(balance))
}

func encodeBankBalanceBig(balance *big.Int) []byte {
	if balance == nil || balance.Sign() == 0 {
		return make([]byte, 8)
	}
	if balance.IsUint64() {
		encoded := make([]byte, 8)
		binary.BigEndian.PutUint64(encoded, balance.Uint64())
		return encoded
	}
	return balance.Bytes()
}

func bankBalanceKey(address types.Address) []byte {
	raw := string(address)
	if !strings.HasPrefix(raw, "0x") {
		return []byte(address)
	}
	clean := strings.TrimPrefix(raw, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) > 20 {
		return []byte(address)
	}
	padded := make([]byte, 20)
	copy(padded[20-len(decoded):], decoded)
	return []byte("0x" + hex.EncodeToString(padded))
}
