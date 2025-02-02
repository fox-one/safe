package keeper

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/safe/apps/bitcoin"
	"github.com/MixinNetwork/safe/apps/ethereum"
	"github.com/MixinNetwork/safe/common"
	"github.com/MixinNetwork/safe/keeper/store"
	"github.com/MixinNetwork/trusted-group/mtg"
	gc "github.com/ethereum/go-ethereum/common"
	"github.com/gofrs/uuid/v5"
	"github.com/shopspring/decimal"
)

type Deposit struct {
	Chain        byte
	Asset        string
	AssetAddress string
	Hash         string
	Index        uint64
	Amount       *big.Int
}

func parseDepositExtra(req *common.Request) (*Deposit, error) {
	extra := req.ExtraBytes()
	if len(extra) < 1+16+32+8 {
		return nil, fmt.Errorf("invalid deposit extra %s", req.ExtraHEX)
	}
	deposit := &Deposit{
		Chain: extra[0],
		Asset: uuid.Must(uuid.FromBytes(extra[1:17])).String(),
	}
	if deposit.Chain != common.SafeCurveChain(req.Curve) {
		panic(req.Id)
	}
	extra = extra[17:]
	switch deposit.Chain {
	case common.SafeChainBitcoin, common.SafeChainLitecoin:
		deposit.Hash = hex.EncodeToString(extra[0:32])
		deposit.Index = binary.BigEndian.Uint64(extra[32:40])
		deposit.Amount = new(big.Int).SetBytes(extra[40:])
		if !deposit.Amount.IsInt64() {
			return nil, fmt.Errorf("invalid deposit amount %s", deposit.Amount.String())
		}
	case common.SafeChainEthereum, common.SafeChainPolygon:
		deposit.Hash = "0x" + hex.EncodeToString(extra[0:32])
		deposit.AssetAddress = gc.BytesToAddress(extra[32:52]).Hex()
		deposit.Index = binary.BigEndian.Uint64(extra[52:60])
		deposit.Amount = new(big.Int).SetBytes(extra[60:])
	default:
		return nil, fmt.Errorf("invalid deposit chain %d", deposit.Chain)
	}

	return deposit, nil
}

func (node *Node) CreateHolderDeposit(ctx context.Context, req *common.Request) ([]*mtg.Transaction, string) {
	if req.Role != common.RequestRoleObserver {
		panic(req.Role)
	}
	deposit, err := parseDepositExtra(req)
	logger.Printf("req.parseDepositExtra(%v) => %v %v", req, deposit, err)
	if err != nil {
		return node.failRequest(ctx, req, "")
	}

	safe, err := node.store.ReadSafe(ctx, req.Holder)
	if err != nil {
		panic(fmt.Errorf("store.ReadSafe(%s) => %v", req.Holder, err))
	}
	if safe == nil || safe.Chain != deposit.Chain {
		logger.Printf("Safe not exists or invalid chain %v", safe)
		return node.failRequest(ctx, req, "")
	}
	if safe.State != SafeStateApproved {
		logger.Printf("Invalid safe state %d", safe.State)
		return node.failRequest(ctx, req, "")
	}

	entry := node.fetchBondAssetReceiver(ctx, safe.Address, deposit.Asset)
	safeAssetId := node.getBondAssetId(ctx, entry, deposit.Asset, req.Holder)
	logger.Printf("node.getBondAssetId(%s %s) => %s", deposit.Asset, req.Holder, safeAssetId)
	bondId := crypto.Sha256Hash([]byte(safeAssetId))
	bond, err := node.fetchAssetMeta(ctx, bondId.String())
	logger.Printf("node.fetchAssetMeta(%v, %s) => %v %v", req, bondId.String(), bond, err)
	if err != nil {
		panic(fmt.Errorf("node.fetchAssetMeta(%s) => %v", bondId.String(), err))
	}
	if bond.Decimals != 18 || bond.AssetId != safeAssetId || bond.Chain != common.SafeChainPolygon {
		panic(bond.AssetId)
	}
	asset, err := node.fetchAssetMetaFromMessengerOrEthereum(ctx, deposit.Asset, deposit.AssetAddress, deposit.Chain)
	if err != nil {
		panic(fmt.Errorf("node.fetchAssetMeta(%s) => %v", deposit.Asset, err))
	}
	if asset.Chain != safe.Chain {
		panic(asset.AssetId)
	}

	plan, err := node.store.ReadLatestOperationParams(ctx, deposit.Chain, req.CreatedAt)
	logger.Printf("store.ReadLatestOperationParams(%d) => %v %v", deposit.Chain, plan, err)
	if err != nil {
		panic(fmt.Errorf("store.ReadLatestOperationParams(%d) => %v", deposit.Chain, err))
	} else if plan == nil || !plan.TransactionMinimum.IsPositive() {
		return node.failRequest(ctx, req, "")
	}

	switch deposit.Chain {
	case common.SafeChainBitcoin, common.SafeChainLitecoin:
		return node.doBitcoinHolderDeposit(ctx, req, deposit, safe, bond.AssetId, asset, plan.TransactionMinimum)
	case common.SafeChainEthereum, common.SafeChainPolygon:
		return node.doEthereumHolderDeposit(ctx, req, deposit, safe, bond.AssetId, asset)
	default:
		return node.failRequest(ctx, req, "")
	}
}

// FIXME Keeper should deny new deposits when too many unspent outputs
func (node *Node) doBitcoinHolderDeposit(ctx context.Context, req *common.Request, deposit *Deposit, safe *store.Safe, safeAssetId string, asset *store.Asset, minimum decimal.Decimal) ([]*mtg.Transaction, string) {
	if asset.Decimals != bitcoin.ValuePrecision {
		panic(asset.Decimals)
	}
	switch asset.AssetId {
	case common.SafeBitcoinChainId, common.SafeLitecoinChainId:
	default:
		panic(asset.AssetId)
	}
	old, _, err := node.store.ReadBitcoinUTXO(ctx, deposit.Hash, int(deposit.Index))
	logger.Printf("store.ReadBitcoinUTXO(%s, %d) => %v %v", deposit.Hash, deposit.Index, old, err)
	if err != nil {
		panic(fmt.Errorf("store.ReadBitcoinUTXO(%s, %d) => %v", deposit.Hash, deposit.Index, err))
	} else if old != nil {
		return node.failRequest(ctx, req, "")
	}
	deposited, err := node.store.ReadDeposit(ctx, deposit.Hash, int64(deposit.Index))
	logger.Printf("store.ReadDeposit(%s, %d, %s, %s) => %v %v", deposit.Hash, int64(deposit.Index), asset.AssetId, safe.Address, deposited, err)
	if err != nil {
		panic(fmt.Errorf("store.ReadDeposit(%s, %d, %s, %s) => %v", deposit.Hash, int64(deposit.Index), asset.AssetId, safe.Address, err))
	} else if deposited != nil {
		return node.failRequest(ctx, req, "")
	}
	c, err := node.store.ReadUnspentUtxoCountForSafe(ctx, safe.Address)
	logger.Printf("store.ReadUnspentUtxoCountForSafe(%s) => %d %v", safe.Address, c, err)
	if err != nil {
		panic(fmt.Errorf("store.ReadUnspentUtxoCountForSafe(%s) => %d %v", safe.Address, c, err))
	}
	if c >= bitcoin.MaxUnspentUtxo {
		return node.failRequest(ctx, req, "")
	}

	rpc, _ := node.bitcoinParams(deposit.Chain)
	btx, err := bitcoin.RPCGetTransaction(deposit.Chain, rpc, deposit.Hash)
	if err != nil {
		panic(fmt.Errorf("bitcoin.RPCTransaction(%s) => %v", deposit.Hash, err))
	}

	amount := decimal.NewFromBigInt(deposit.Amount, -int32(asset.Decimals))
	change, err := node.checkBitcoinChange(ctx, deposit, btx)
	logger.Printf("node.checkBitcoinChange(%v, %v) => %t %v", deposit, btx, change, err)
	if err != nil {
		panic(fmt.Errorf("node.checkBitcoinChange(%v) => %v", deposit, err))
	}
	if amount.Cmp(minimum) < 0 && !change {
		return node.failRequest(ctx, req, "")
	}
	if amount.Cmp(decimal.New(bitcoin.ValueDust(safe.Chain), -bitcoin.ValuePrecision)) < 0 {
		panic(deposit.Hash)
	}

	output, err := node.verifyBitcoinTransaction(ctx, req, deposit, safe, bitcoin.InputTypeP2WSHMultisigHolderSigner)
	logger.Printf("node.verifyBitcoinTransaction(%v) => %v %v", req, output, err)
	if err != nil {
		panic(fmt.Errorf("node.verifyBitcoinTransaction(%s) => %v", deposit.Hash, err))
	}
	if output == nil {
		return node.failRequest(ctx, req, "")
	}

	var txs []*mtg.Transaction
	if !change {
		tx := node.buildTransaction(ctx, req.Output, safe.RequestId, safeAssetId, safe.Receivers, int(safe.Threshold), amount.String(), nil, req.Id)
		if tx == nil {
			// no compaction needed, just retry from observer
			return node.failRequest(ctx, req, "")
		}
		txs = append(txs, tx)
	}

	sender, err := bitcoin.RPCGetTransactionSender(safe.Chain, rpc, btx)
	if err != nil {
		panic(fmt.Errorf("bitcoin.RPCGetTransactionSender(%s) => %v", btx.TxId, err))
	}
	err = node.store.WriteBitcoinOutputFromRequest(ctx, safe, output, req, asset.AssetId, sender, txs)
	if err != nil {
		panic(err)
	}
	return txs, ""
}

func (node *Node) doEthereumHolderDeposit(ctx context.Context, req *common.Request, deposit *Deposit, safe *store.Safe, safeAssetId string, asset *store.Asset) ([]*mtg.Transaction, string) {
	_, chainId := node.ethereumParams(deposit.Chain)
	if asset.AssetId == chainId && asset.Decimals != ethereum.ValuePrecision {
		panic(asset.Decimals)
	}
	deposited, err := node.store.ReadDeposit(ctx, deposit.Hash, int64(deposit.Index))
	logger.Printf("store.ReadDeposit(%s, %d, %s, %s) => %v %v", deposit.Hash, int64(deposit.Index), asset.AssetId, safe.Address, deposited, err)
	if err != nil {
		panic(fmt.Errorf("store.ReadDeposit(%s, %d, %s, %s) => %v", deposit.Hash, int64(deposit.Index), asset.AssetId, safe.Address, err))
	} else if deposited != nil {
		return node.failRequest(ctx, req, "")
	}

	safeBalance, err := node.store.ReadEthereumBalance(ctx, safe.Address, asset.AssetId, safeAssetId)
	logger.Printf("store.ReadEthereumBalance(%s, %s) => %v %v", safe.Address, asset.AssetId, safeBalance, err)
	if err != nil {
		panic(err)
	}
	safeBalance.UpdateBalance(deposit.Amount)
	if safeBalance.AssetAddress == "" {
		safeBalance.AssetAddress = deposit.AssetAddress
	}

	output, err := node.verifyEthereumTransaction(ctx, req, deposit, safe)
	logger.Printf("node.verifyEthereumTransaction(%v) => %v %v", req, output, err)
	if err != nil {
		panic(fmt.Errorf("node.verifyEthereumTransaction(%s) => %v", deposit.Hash, err))
	}
	if output == nil {
		return node.failRequest(ctx, req, "")
	}

	t := node.buildTransaction(ctx, req.Output, safe.RequestId, safeAssetId, safe.Receivers, int(safe.Threshold), decimal.NewFromBigInt(deposit.Amount, -int32(asset.Decimals)).String(), nil, req.Id)
	if t == nil {
		// no compaction needed, just retry from observer
		return node.failRequest(ctx, req, "")
	}
	err = node.store.CreateEthereumBalanceDepositFromRequest(ctx, safe, safeBalance, deposit.Hash, int64(deposit.Index), deposit.Amount, output.Sender, req, []*mtg.Transaction{t})
	logger.Printf("store.UpdateEthereumBalanceFromRequest(%v) => %v", req, err)
	if err != nil {
		panic(err)
	}
	return []*mtg.Transaction{t}, ""
}

func (node *Node) checkBitcoinChange(ctx context.Context, deposit *Deposit, btx *bitcoin.RPCTransaction) (bool, error) {
	vin, spentBy, err := node.store.ReadBitcoinUTXO(ctx, btx.Vin[0].TxId, int(btx.Vin[0].VOUT))
	if err != nil || vin == nil {
		return false, err
	}
	tx, err := node.store.ReadTransaction(ctx, spentBy)
	if err != nil {
		return false, err
	}
	var recipients []map[string]string
	err = json.Unmarshal([]byte(tx.Data), &recipients)
	if err != nil || len(recipients) == 0 {
		return false, fmt.Errorf("store.ReadTransaction(%s) => %s", spentBy, tx.Data)
	}
	return deposit.Index >= uint64(len(recipients)), nil
}

func (node *Node) verifyBitcoinTransaction(ctx context.Context, req *common.Request, deposit *Deposit, safe *store.Safe, typ int) (*bitcoin.Input, error) {
	rpc, asset := node.bitcoinParams(safe.Chain)
	if deposit.Asset != asset {
		return nil, nil
	}

	input := &bitcoin.Input{
		TransactionHash: deposit.Hash,
		Index:           uint32(deposit.Index),
		Satoshi:         deposit.Amount.Int64(),
	}

	var receiver string
	switch typ {
	case bitcoin.InputTypeP2WSHMultisigHolderSigner:
		path := common.DecodeHexOrPanic(safe.Path)
		wsa, err := node.buildBitcoinWitnessAccountWithDerivation(ctx, safe.Holder, safe.Signer, safe.Observer, path, safe.Timelock, safe.Chain)
		if err != nil {
			panic(err)
		}
		if wsa.Address != safe.Address {
			panic(safe.Address)
		}
		receiver = wsa.Address
		input.Script = wsa.Script
		input.Sequence = wsa.Sequence
	default:
		panic(typ)
	}

	info, err := node.store.ReadLatestNetworkInfo(ctx, safe.Chain, req.CreatedAt)
	logger.Printf("store.ReadLatestNetworkInfo(%d) => %v %v", safe.Chain, info, err)
	if err != nil || info == nil {
		return nil, err
	}
	if info.CreatedAt.After(req.CreatedAt) {
		return nil, fmt.Errorf("malicious bitcoin network info %v", info)
	}

	tx, output, err := bitcoin.RPCGetTransactionOutput(deposit.Chain, rpc, deposit.Hash, int64(deposit.Index))
	logger.Printf("bitcoin.RPCGetTransactionOutput(%s, %d) => %v %v", deposit.Hash, deposit.Index, output, err)
	if err != nil || output == nil {
		return nil, fmt.Errorf("malicious bitcoin deposit or node not in sync? %s %v", deposit.Hash, err)
	}
	if output.Address != receiver || output.Satoshi != input.Satoshi {
		return nil, fmt.Errorf("malicious bitcoin deposit %s", deposit.Hash)
	}

	confirmations := info.Height - output.Height + 1
	if info.Height < output.Height {
		confirmations = 0
	}
	sender, err := bitcoin.RPCGetTransactionSender(safe.Chain, rpc, tx)
	if err != nil {
		return nil, fmt.Errorf("bitcoin.RPCGetTransactionSender(%s) => %v", tx.TxId, err)
	}
	isSafe, err := node.checkTrustedSender(ctx, sender)
	if err != nil {
		return nil, fmt.Errorf("node.checkTrustedSender(%s) => %v", sender, err)
	}
	if isSafe && confirmations > 0 {
		confirmations = 1000000
	}
	if !bitcoin.CheckFinalization(confirmations, output.Coinbase) {
		return nil, fmt.Errorf("bitcoin.CheckFinalization(%s)", tx.TxId)
	}

	return input, nil
}

func (node *Node) verifyEthereumTransaction(ctx context.Context, req *common.Request, deposit *Deposit, safe *store.Safe) (*ethereum.Transfer, error) {
	info, err := node.store.ReadLatestNetworkInfo(ctx, safe.Chain, req.CreatedAt)
	logger.Printf("store.ReadLatestNetworkInfo(%d) => %v %v", safe.Chain, info, err)
	if err != nil || info == nil {
		return nil, err
	}
	if info.CreatedAt.After(req.CreatedAt) {
		return nil, fmt.Errorf("malicious ethereum network info %v", info)
	}

	rpc, chainId := node.ethereumParams(safe.Chain)
	t, etx, err := ethereum.VerifyDeposit(ctx, deposit.Chain, rpc, deposit.Hash, chainId, deposit.AssetAddress, safe.Address, int64(deposit.Index), deposit.Amount)
	if err != nil || t == nil {
		return nil, fmt.Errorf("malicious ethereum deposit or node not in sync? %s %v", deposit.Hash, err)
	}
	if t.Receiver != safe.Address {
		return nil, fmt.Errorf("malicious ethereum deposit %s", deposit.Hash)
	}

	confirmations := info.Height - etx.BlockHeight + 1
	if info.Height < etx.BlockHeight {
		confirmations = 0
	}
	isSafe, err := node.checkTrustedSender(ctx, t.Sender)
	if err != nil {
		return nil, fmt.Errorf("node.checkTrustedSender(%s) => %v", t.Sender, err)
	}
	if isSafe && confirmations > 0 {
		confirmations = 1000000
	}
	if !ethereum.CheckFinalization(confirmations, safe.Chain) {
		return nil, fmt.Errorf("ethereum.CheckFinalization(%s)", etx.Hash)
	}

	return t, nil
}

func (node *Node) checkTrustedSender(ctx context.Context, address string) (bool, error) {
	if slices.Contains([]string{
		"bc1ql24x05zhqrpejar0p3kevhu48yhnnr3r95sv4y",
		"ltc1qs46hqx885kpz83vfg6evm9dsuapznfaw997qwl",
		"0x1616b057F8a89955d4A4f9fd9Eb10289ac0e44A1",
	}, address) {
		return true, nil
	}
	safe, err := node.store.ReadSafeByAddress(ctx, address)
	if err != nil {
		return false, fmt.Errorf("store.ReadSafeByAddress(%s) => %v", address, err)
	}
	return safe != nil, nil
}
