package app

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInsufficientFee        = errors.New("insufficient transaction fee")
	ErrInsufficientFeeBalance = errors.New("insufficient fee balance")
	ErrInvalidGas             = errors.New("invalid transaction gas")
	ErrMissingNonce           = errors.New("missing transaction nonce")
	ErrMissingSigner          = errors.New("missing transaction signer")
	ErrInvalidNonce           = errors.New("invalid transaction nonce")
)

// ethereumModuleName is the only module an Ethereum-authenticated transaction
// may be addressed to.
const ethereumModuleName = "evm"

// TxAuthenticator authenticates a transaction that carries no signed envelope.
type TxAuthenticator interface {
	AuthenticateTx(ctx Context, tx types.Tx) error
}

type AnteConfig struct {
	MinFee uint64
	// BaseFee is the initial base fee per gas of the fee market. After the first
	// block the base fee is the application state FinalizeBlock stores.
	BaseFee       uint64
	MinGas        uint64
	MaxGas        uint64
	RequireNonce  bool
	FeeCollector  string
	RequireSigned bool
	// SignatureVerifier verifies signed envelopes; nil selects Ed25519.
	SignatureVerifier TxSignatureVerifier
	// EthereumAuthenticator authenticates unsigned Ethereum transactions
	// (eth_raw/eth_hash tags) addressed to the evm module. Without one every
	// transaction carrying an Ethereum tag is rejected.
	EthereumAuthenticator TxAuthenticator
	// EvidenceAuthenticator admits unsigned, fee-less misbehaviour evidence
	// transactions addressed to the slashing module. The evidence carries the
	// offender's own signatures, so the transaction needs no signer: the
	// authenticator verifies those signatures against the validator set and
	// rejects evidence whose offence is already punished. Without one every
	// unsigned evidence transaction falls under RequireSigned.
	EvidenceAuthenticator EvidenceAuthenticator
	FeeMarket             FeeMarketConfig
}

// EvidenceAuthenticator authenticates an evidence transaction and reports the
// gas its execution consumes.
type EvidenceAuthenticator interface {
	TxAuthenticator
	EstimateGas(ctx Context, tx types.Tx) (uint64, error)
}

// evidenceModuleName and evidenceAction address the only transactions an
// EvidenceAuthenticator may admit.
const (
	evidenceModuleName = "slashing"
	evidenceAction     = "evidence"
)

// TxMeta is the execution metadata of a transaction. A transaction whose
// metadata does not parse yields the zero value; consensus paths use the
// strict parser instead and reject such a transaction.
type TxMeta struct {
	Signer   types.Address
	Nonce    uint64
	Fee      uint64
	FeeBig   *big.Int
	Gas      uint64
	HasNonce bool
}

type AnteKeeper struct {
	config   AnteConfig
	accounts AccountKeeper
}

func NewAnteKeeper(config AnteConfig) *AnteKeeper {
	return &AnteKeeper{config: config, accounts: NewAccountKeeper()}
}

func ParseTxMeta(tx types.Tx) TxMeta {
	parsed, err := txcodec.ParseMeta(tx)
	if err != nil {
		return TxMeta{}
	}
	meta := TxMeta{
		Signer:   types.Address(parsed.Signer),
		Nonce:    parsed.Nonce,
		HasNonce: parsed.HasNonce,
		Fee:      parsed.FeeSaturated(),
		Gas:      parsed.Gas,
	}
	if _, found := parsed.Tags[txcodec.TagFee]; found {
		meta.FeeBig = parsed.Fee
	}
	return meta
}

// admission is a transaction that passed every check that does not depend on
// account state: its envelope and signature, its tag grammar, the Ethereum
// authentication gate and the fee, gas and nonce rules.
type admission struct {
	tx       types.Tx
	payload  types.Tx
	meta     txcodec.Meta
	payer    types.Address
	ethereum bool
	gasLimit uint64
	blobGas  uint64
}

// parseAdmission unwraps and strictly parses tx. It is the first step of every
// path, including a runtime without an ante handler: a transaction whose
// envelope or tags are ambiguous is never executed.
func parseAdmission(tx types.Tx) (admission, error) {
	payload, err := txcodec.Unwrap(tx)
	if err != nil {
		return admission{}, err
	}
	meta, err := txcodec.ParseMeta(payload)
	if err != nil {
		return admission{}, err
	}
	return admission{tx: tx, payload: payload, meta: meta, payer: types.Address(meta.Signer)}, nil
}

// admit runs the stateless admission checks of tx against the context's chain
// parameters and fee market state.
func (keeper *AnteKeeper) admit(ctx Context, tx types.Tx) (admission, error) {
	adm, err := parseAdmission(tx)
	if err != nil {
		return admission{}, err
	}
	signed := txcodec.IsSigned(tx)
	if !signed && isEvidenceTx(adm) {
		return keeper.admitEvidence(ctx, adm)
	}
	switch {
	case txcodec.HasEthereumTag(adm.meta.Tags):
		// Ethereum tags are never a way around the signature requirement: they
		// are honoured only for an unsigned transaction addressed to the evm
		// module whose raw signature the evm module verified.
		if err := keeper.authenticateEthereum(ctx, adm, signed); err != nil {
			return admission{}, err
		}
		adm.ethereum = true
	case signed:
		if err := VerifySignedTx(ctx.ChainID, tx, keeper.signatureVerifier()); err != nil {
			return admission{}, err
		}
	case keeper.config.RequireSigned:
		return admission{}, ErrInvalidSignedTx
	}
	adm.gasLimit = adm.meta.Gas
	if adm.gasLimit == 0 {
		adm.gasLimit = uint64(len(tx))
	}
	if adm.ethereum {
		if value, found := adm.meta.Tags[txcodec.TagEthereumBlobGas]; found {
			blobGas, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return admission{}, ErrInvalidGas
			}
			adm.blobGas = blobGas
		}
		if err := keeper.validateEthereumMeta(adm.meta); err != nil {
			return admission{}, err
		}
		return adm, nil
	}
	if err := keeper.validateMeta(ctx, adm.meta); err != nil {
		return admission{}, err
	}
	return adm, nil
}

// isEvidenceTx reports whether adm is an unsigned evidence transaction: it is
// addressed to slashing:evidence and carries no tags, so it names no signer,
// fee or nonce that admission would otherwise have to check.
func isEvidenceTx(adm admission) bool {
	canonical, err := txcodec.ParseCanonical(adm.payload)
	if err != nil {
		return false
	}
	return canonical.Module == evidenceModuleName && canonical.Action == evidenceAction && len(canonical.Tags) == 0
}

// admitEvidence admits an unsigned evidence transaction only when the slashing
// module authenticates its evidence. It pays no fee and bumps no nonce; its gas
// is what its execution consumes, so it still counts against the block limit.
func (keeper *AnteKeeper) admitEvidence(ctx Context, adm admission) (admission, error) {
	authenticator := keeper.config.EvidenceAuthenticator
	if authenticator == nil {
		if keeper.config.RequireSigned {
			return admission{}, ErrInvalidSignedTx
		}
		return admission{}, ErrInvalidTxSignature
	}
	if err := authenticator.AuthenticateTx(ctx, adm.payload); err != nil {
		return admission{}, fmt.Errorf("%w: %v", ErrInvalidTxSignature, err)
	}
	gas, err := authenticator.EstimateGas(ctx, adm.payload)
	if err != nil || gas == 0 {
		return admission{}, ErrInvalidGas
	}
	if keeper.config.MaxGas > 0 && gas > keeper.config.MaxGas {
		return admission{}, ErrInvalidGas
	}
	adm.payer = ""
	adm.gasLimit = gas
	return adm, nil
}

func (keeper *AnteKeeper) authenticateEthereum(ctx Context, adm admission, signed bool) error {
	_, hasRaw := adm.meta.Tags[txcodec.TagEthereumRaw]
	_, hasHash := adm.meta.Tags[txcodec.TagEthereumHash]
	authenticator := keeper.config.EthereumAuthenticator
	if signed || !hasRaw || !hasHash || authenticator == nil {
		return ErrInvalidTxSignature
	}
	canonical, err := txcodec.ParseCanonical(adm.payload)
	if err != nil || canonical.Module != ethereumModuleName {
		return ErrInvalidTxSignature
	}
	if err := authenticator.AuthenticateTx(ctx, adm.tx); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTxSignature, err)
	}
	return nil
}

func (keeper *AnteKeeper) validateMeta(ctx Context, meta txcodec.Meta) error {
	if meta.Fee.Cmp(new(big.Int).SetUint64(keeper.config.MinFee)) < 0 {
		return ErrInsufficientFee
	}
	if ctx.BaseFee > 0 {
		if meta.Gas == 0 {
			return ErrInvalidGas
		}
		required := new(big.Int).Mul(new(big.Int).SetUint64(meta.Gas), new(big.Int).SetUint64(ctx.BaseFee))
		if meta.Fee.Cmp(required) < 0 {
			return ErrInsufficientFee
		}
	}
	if err := keeper.validateGasAndNonce(meta); err != nil {
		return err
	}
	if meta.Fee.Sign() > 0 && meta.Signer == "" {
		return ErrMissingSigner
	}
	return nil
}

// validateEthereumMeta holds the rules for an authenticated Ethereum
// transaction. The fee is not checked here: the evm module bounds the fee tag
// by what the raw signature authorizes and executes the real fee market.
func (keeper *AnteKeeper) validateEthereumMeta(meta txcodec.Meta) error {
	if meta.Gas == 0 {
		return ErrInvalidGas
	}
	return keeper.validateGasAndNonce(meta)
}

func (keeper *AnteKeeper) validateGasAndNonce(meta txcodec.Meta) error {
	if keeper.config.MinGas > 0 && meta.Gas < keeper.config.MinGas {
		return ErrInvalidGas
	}
	if keeper.config.MaxGas > 0 && meta.Gas > keeper.config.MaxGas {
		return ErrInvalidGas
	}
	if keeper.config.RequireNonce && (meta.Signer == "" || !meta.HasNonce) {
		return ErrMissingNonce
	}
	if meta.HasNonce && meta.Nonce == math.MaxUint64 {
		return ErrInvalidNonce
	}
	return nil
}

// accountState is the account data the ante handler reads and writes: the
// per-account sequence and the bank balances fees move between.
type accountState interface {
	sequence(address types.Address) (uint64, error)
	setSequence(address types.Address, sequence uint64) error
	balance(address types.Address) (*big.Int, error)
	setBalance(address types.Address, balance *big.Int) error
}

// storeAccounts reads and writes the state store directly.
type storeAccounts struct {
	ctx      context.Context
	store    StateStore
	accounts AccountKeeper
}

func (accounts storeAccounts) sequence(address types.Address) (uint64, error) {
	return accounts.accounts.Sequence(accounts.ctx, accounts.store, address)
}

func (accounts storeAccounts) setSequence(address types.Address, sequence uint64) error {
	return accounts.accounts.SetSequence(accounts.ctx, accounts.store, address, sequence)
}

func (accounts storeAccounts) balance(address types.Address) (*big.Int, error) {
	return bankBalanceBig(accounts.ctx, accounts.store, address)
}

func (accounts storeAccounts) setBalance(address types.Address, balance *big.Int) error {
	return setBankBalanceBig(accounts.ctx, accounts.store, address, balance)
}

// cachedAccounts is a copy-on-write view over another accountState. Reads fall
// through once and are cached, writes stay in memory: a simulation of a list of
// transactions sees the fees and sequences of the transactions before it.
type cachedAccounts struct {
	base      accountState
	sequences map[string]uint64
	balances  map[string]*big.Int
}

func newCachedAccounts(base accountState) *cachedAccounts {
	return &cachedAccounts{
		base:      base,
		sequences: make(map[string]uint64),
		balances:  make(map[string]*big.Int),
	}
}

func (accounts *cachedAccounts) sequence(address types.Address) (uint64, error) {
	key := NormalizeActor(address)
	if sequence, found := accounts.sequences[key]; found {
		return sequence, nil
	}
	sequence, err := accounts.base.sequence(address)
	if err != nil {
		return 0, err
	}
	accounts.sequences[key] = sequence
	return sequence, nil
}

func (accounts *cachedAccounts) setSequence(address types.Address, sequence uint64) error {
	accounts.sequences[NormalizeActor(address)] = sequence
	return nil
}

func (accounts *cachedAccounts) balance(address types.Address) (*big.Int, error) {
	key := NormalizeActor(address)
	if balance, found := accounts.balances[key]; found {
		return new(big.Int).Set(balance), nil
	}
	balance, err := accounts.base.balance(address)
	if err != nil {
		return nil, err
	}
	accounts.balances[key] = new(big.Int).Set(balance)
	return balance, nil
}

func (accounts *cachedAccounts) setBalance(address types.Address, balance *big.Int) error {
	accounts.balances[NormalizeActor(address)] = new(big.Int).Set(balance)
	return nil
}

// simulation applies the account-level admission of a list of transactions to a
// private copy of the state, in order. It is how CheckTx, PrepareProposal and
// ProcessProposal decide that a sender's nonces are consecutive and that its
// fees are affordable together, not merely one at a time.
type simulation struct {
	keeper   *AnteKeeper
	accounts *cachedAccounts
}

// newSimulation returns nil when there is nothing to simulate against: no ante
// handler or no state store. A nil simulation accepts every transaction.
func (keeper *AnteKeeper) newSimulation(ctx Context) *simulation {
	if keeper == nil || ctx.Store == nil {
		return nil
	}
	return &simulation{
		keeper:   keeper,
		accounts: newCachedAccounts(storeAccounts{ctx: ctx.GoContext(), store: ctx.Store, accounts: keeper.accounts}),
	}
}

// apply admits adm after the transactions applied before it, or fails without
// changing the simulated state.
func (sim *simulation) apply(adm admission) error {
	if sim == nil {
		return nil
	}
	return sim.keeper.charge(sim.accounts, adm, true)
}

// charge validates the account-level admission of adm (the nonce is the
// account's next one, the payer can afford the fee) and then applies it: the
// fee moves to the collector and the sequence advances. Every validation
// happens before the first write, so a rejected transaction changes nothing.
//
// The sequence of an Ethereum transaction belongs to the EVM, which advances it
// when the transaction executes. Only a simulation advances it here, so the
// next transaction of the same sender is checked against the right nonce.
func (keeper *AnteKeeper) charge(accounts accountState, adm admission, simulate bool) error {
	meta := adm.meta
	if adm.payer == "" {
		return nil
	}
	if meta.HasNonce {
		current, err := accounts.sequence(adm.payer)
		if err != nil {
			return err
		}
		expected := current
		if !adm.ethereum {
			if current == math.MaxUint64 {
				return invalidNonceError(meta, current)
			}
			expected = current + 1
		}
		if meta.Nonce != expected {
			return invalidNonceError(meta, expected)
		}
	}
	if !adm.ethereum && meta.Fee.Sign() > 0 {
		if err := keeper.transferFee(accounts, adm.payer, meta.Fee); err != nil {
			return err
		}
	}
	if !meta.HasNonce {
		return nil
	}
	switch {
	case !adm.ethereum:
		return accounts.setSequence(adm.payer, meta.Nonce)
	case simulate:
		return accounts.setSequence(adm.payer, meta.Nonce+1)
	}
	return nil
}

// transferFee moves fee from payer to the fee collector. A payer that is the
// collector keeps its balance: crediting the collector from the same record
// would otherwise write the balance twice and mint the fee.
func (keeper *AnteKeeper) transferFee(accounts accountState, payer types.Address, fee *big.Int) error {
	collector := keeper.feeCollector()
	payerBalance, err := accounts.balance(payer)
	if err != nil {
		return err
	}
	if payerBalance.Cmp(fee) < 0 {
		return ErrInsufficientFeeBalance
	}
	if NormalizeActor(payer) == NormalizeActor(collector) {
		return nil
	}
	collectorBalance, err := accounts.balance(collector)
	if err != nil {
		return err
	}
	credited := new(big.Int).Add(collectorBalance, fee)
	if credited.BitLen() > 256 {
		return ErrInsufficientFeeBalance
	}
	if err := accounts.setBalance(payer, new(big.Int).Sub(payerBalance, fee)); err != nil {
		return err
	}
	return accounts.setBalance(collector, credited)
}

// begin charges adm against the state in ctx.Store before the transaction
// executes: the fee is paid and the sequence advanced whatever the module
// does afterwards.
func (keeper *AnteKeeper) begin(ctx Context, adm admission) error {
	if ctx.Store == nil {
		return nil
	}
	return keeper.charge(keeper.storeAccounts(ctx), adm, false)
}

// finish re-establishes the sequence after a native transaction executed: the
// ante handler owns it, whatever a module wrote under the same key.
func (keeper *AnteKeeper) finish(ctx Context, adm admission) error {
	if ctx.Store == nil || adm.ethereum || adm.payer == "" || !adm.meta.HasNonce {
		return nil
	}
	return keeper.accounts.SetSequence(ctx.GoContext(), ctx.Store, adm.payer, adm.meta.Nonce)
}

// fail charges an Ethereum transaction whose execution failed and was
// discarded. The EVM wrote neither the fee nor the nonce, so the ante handler
// consumes the nonce and collects the failure fee when the payer can afford
// it; a payer that cannot is neither charged nor sequenced. It returns the fee
// charged, nil when the transaction was not charged.
func (keeper *AnteKeeper) fail(ctx Context, adm admission) (*big.Int, error) {
	if ctx.Store == nil || !adm.ethereum || adm.payer == "" || !adm.meta.HasNonce {
		return nil, nil
	}
	accounts := keeper.storeAccounts(ctx)
	fee := ethereumFailureFee(ctx, adm.meta)
	if fee.Sign() > 0 {
		if err := keeper.transferFee(accounts, adm.payer, fee); err != nil {
			if errors.Is(err, ErrInsufficientFeeBalance) {
				return nil, nil
			}
			return nil, err
		}
	}
	current, err := accounts.sequence(adm.payer)
	if err != nil {
		return nil, err
	}
	if current <= adm.meta.Nonce {
		if err := accounts.setSequence(adm.payer, adm.meta.Nonce+1); err != nil {
			return nil, err
		}
	}
	return fee, nil
}

// ethereumFailureFee is what a failed Ethereum transaction pays: the fee tag,
// but never less than its gas at the block's base fee. The fee tag is not
// signed; the evm module bounds it from above by what the signature
// authorizes, and that bound is at least the gas at the base fee, because a
// transaction whose fee cap is below the base fee is not admitted. Without the
// floor a relayer that leaves the tag out or sets it low would have block
// space consumed for free.
func ethereumFailureFee(ctx Context, meta txcodec.Meta) *big.Int {
	fee := new(big.Int)
	if meta.Fee != nil {
		fee.Set(meta.Fee)
	}
	if ctx.BaseFee > 0 {
		floor := new(big.Int).Mul(new(big.Int).SetUint64(meta.Gas), new(big.Int).SetUint64(ctx.BaseFee))
		if fee.Cmp(floor) < 0 {
			fee = floor
		}
	}
	return fee
}

func (keeper *AnteKeeper) storeAccounts(ctx Context) storeAccounts {
	return storeAccounts{ctx: ctx.GoContext(), store: ctx.Store, accounts: keeper.accounts}
}

func (keeper *AnteKeeper) feeCollector() types.Address {
	if keeper.config.FeeCollector == "" {
		return "fee_collector"
	}
	return types.Address(keeper.config.FeeCollector)
}

func (keeper *AnteKeeper) signatureVerifier() TxSignatureVerifier {
	if keeper.config.SignatureVerifier != nil {
		return keeper.config.SignatureVerifier
	}
	return vexocrypto.Ed25519Signer{}
}

func invalidNonceError(meta txcodec.Meta, expected uint64) error {
	return fmt.Errorf("%w: signer=%s nonce=%d expected=%d", ErrInvalidNonce, meta.Signer, meta.Nonce, expected)
}

func ParseInvalidNonceLog(log string) (uint64, uint64, bool) {
	if !strings.Contains(log, ErrInvalidNonce.Error()) {
		return 0, 0, false
	}
	var nonce uint64
	var expected uint64
	nonceFound := false
	expectedFound := false
	for _, field := range strings.Fields(log) {
		if value, found := strings.CutPrefix(field, "nonce="); found {
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return 0, 0, false
			}
			nonce = parsed
			nonceFound = true
		}
		if value, found := strings.CutPrefix(field, "expected="); found {
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return 0, 0, false
			}
			expected = parsed
			expectedFound = true
		}
	}
	return nonce, expected, nonceFound && expectedFound
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
		return new(big.Int), nil
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
	return []byte(txcodec.NormalizeAddress(string(address)))
}
