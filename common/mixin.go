package common

import (
	"context"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/MixinNetwork/bot-api-go-client/v3"
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/trusted-group/mtg"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/shopspring/decimal"
)

type KernelTransactionReader interface {
	ReadKernelTransactionUntilSufficient(ctx context.Context, txHash string) (*common.VersionedTransaction, error)
}

// TODO the output should include the snapshot signature, then it can just be
// verified against the active kernel nodes public key
func VerifyKernelTransaction(ctx context.Context, reader KernelTransactionReader, out *mtg.Action, timeout time.Duration) (*common.VersionedTransaction, error) {
	signed, err := reader.ReadKernelTransactionUntilSufficient(ctx, out.TransactionHash)
	if err != nil {
		return nil, err
	}
	logger.Printf("common.readKernelTransaction(%s) => %v %v", out.TransactionHash, signed, err)

	if signed == nil {
		return nil, fmt.Errorf("common.VerifyKernelTransaction(%v) not found %v", out, err)
	}

	if !strings.Contains(string(signed.Extra), out.Extra) && !strings.Contains(hex.EncodeToString(signed.Extra), out.Extra) {
		return nil, fmt.Errorf("common.VerifyKernelTransaction(%v) memo mismatch %x", out, signed.Extra)
	}
	if signed.Asset != crypto.Sha256Hash([]byte(out.AssetId)) {
		return nil, fmt.Errorf("common.VerifyKernelTransaction(%v) asset mismatch %s", out, signed.Asset)
	}
	if len(signed.Outputs) < out.OutputIndex+1 {
		return nil, fmt.Errorf("common.VerifyKernelTransaction(%v) output mismatch %d", out, len(signed.Outputs))
	}
	if a, _ := decimal.NewFromString(signed.Outputs[out.OutputIndex].Amount.String()); !a.Equal(out.Amount) {
		return nil, fmt.Errorf("common.VerifyKernelTransaction(%v) amount mismatch %s", out, a)
	}

	return signed, nil
}

func getEnoughUtxosToSpend(utxos []*mixin.SafeUtxo, amount decimal.Decimal) ([]*mixin.SafeUtxo, bool) {
	total := decimal.NewFromInt(0)
	for i, o := range utxos {
		total = total.Add(o.Amount)
		if total.Cmp(amount) < 0 {
			continue
		}
		return utxos[:i+1], true
	}
	return nil, false
}

func ExtraLimit(tx mixinnet.Transaction) int {
	if tx.Asset != mixinnet.XINAssetId {
		return common.ExtraSizeGeneralLimit
	}
	if len(tx.Outputs) < 1 {
		return common.ExtraSizeGeneralLimit
	}
	out := tx.Outputs[0]
	if len(out.Keys) != 1 {
		return common.ExtraSizeGeneralLimit
	}
	if out.Type != common.OutputTypeScript {
		return common.ExtraSizeGeneralLimit
	}
	if out.Script.String() != "fffe40" {
		return common.ExtraSizeGeneralLimit
	}
	step := mixinnet.IntegerFromString(common.ExtraStoragePriceStep)
	if out.Amount.Cmp(step) < 0 {
		return common.ExtraSizeGeneralLimit
	}
	cells := out.Amount.Count(step)
	limit := cells * common.ExtraSizeStorageStep
	if limit > common.ExtraSizeStorageCapacity {
		return common.ExtraSizeStorageCapacity
	}
	return int(limit)
}

func WriteStorageUntilSufficient(ctx context.Context, client *mixin.Client, extra []byte, sTraceId string, su bot.SafeUser) (crypto.Hash, error) {
	for {
		old, err := SafeReadTransactionRequestUntilSufficient(ctx, client, sTraceId)
		if err != nil {
			return crypto.Hash{}, err
		}
		if old != nil && old.State == mixin.SafeUtxoStateSpent {
			return crypto.HashFromString(old.TransactionHash)
		}

		req, err := SafeReadMultisigRequestUntilSufficient(ctx, client, sTraceId)
		if err != nil {
			return crypto.Hash{}, err
		}
		if req != nil {
			if !slices.Contains(req.Signers, client.ClientID) {
				_, err = SignMultisigUntilSufficient(ctx, client, req.RequestID, req.RawTransaction, req.Views, []string{client.ClientID}, su.SpendPrivateKey)
				if err != nil {
					return crypto.Hash{}, err
				}
			}
			time.Sleep(3 * time.Second)
			continue
		}

		_, err = bot.CreateObjectStorageTransaction(ctx, nil, nil, extra, sTraceId, nil, "", &su)
		logger.Verbosef("common.mixin.CreateObjectStorageTransaction(%s) => %v", sTraceId, err)
		if err != nil {
			// FIXME the sdk error in signature
			if mtg.CheckRetryableError(err) || strings.Contains(err.Error(), "signature verification failed") {
				time.Sleep(3 * time.Second)
				continue
			}
			return crypto.Hash{}, err
		}
	}
}

func SendTransactionUntilSufficient(ctx context.Context, client *mixin.Client, members []string, threshold int, receivers []string, receiversThreshold int, amount decimal.Decimal, traceId, assetId, memo, spendPrivateKey string) (*mixin.SafeTransactionRequest, error) {
	for {
		req, err := SafeReadTransactionRequestUntilSufficient(ctx, client, traceId)
		if err != nil {
			return nil, err
		}
		if req != nil {
			if req.State == mixin.SafeUtxoStateSpent {
				return req, nil
			}
			time.Sleep(3 * time.Second)
			continue
		}

		utxos, err := listSafeUtxosUntilSufficient(ctx, client, members, threshold, assetId)
		if err != nil {
			return nil, err
		}
		utxos, sufficient := getEnoughUtxosToSpend(utxos, amount)
		if !sufficient {
			time.Sleep(10 * time.Second)
			continue
		}
		b := mixin.NewSafeTransactionBuilder(utxos)
		b.Memo = memo
		b.Hint = traceId

		tx, err := client.MakeTransaction(ctx, b, []*mixin.TransactionOutput{
			{
				Address: mixin.RequireNewMixAddress(receivers, byte(receiversThreshold)),
				Amount:  amount,
			},
		})
		if err != nil {
			return nil, err
		}
		raw, err := tx.Dump()
		if err != nil {
			return nil, err
		}
		req, err = CreateTransactionRequestUntilSufficient(ctx, client, traceId, raw)
		if err != nil {
			if CheckTransactionRetryError(err.Error()) {
				time.Sleep(3 * time.Second)
				continue
			}
			return nil, err
		}
		_, err = SignTransactionUntilSufficient(ctx, client, req.RequestID, req.RawTransaction, req.Views, spendPrivateKey)
		if err != nil {
			if CheckTransactionRetryError(err.Error()) {
				time.Sleep(3 * time.Second)
				continue
			}
			return nil, err
		}
		time.Sleep(3 * time.Second)
	}
}

func listSafeUtxosUntilSufficient(ctx context.Context, client *mixin.Client, members []string, threshold int, assetId string) ([]*mixin.SafeUtxo, error) {
	for {
		utxos, err := client.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
			Members:   members,
			Threshold: uint8(threshold),
			State:     mixin.SafeUtxoStateUnspent,
			Asset:     assetId,
		})
		logger.Verbosef("common.mixin.SafeListUtxos(%v %d %s) => %v %v\n", members, threshold, assetId, utxos, err)
		if err != nil && mtg.CheckRetryableError(err) {
			time.Sleep(3 * time.Second)
			continue
		}
		return utxos, err
	}
}

func CreateTransactionRequestUntilSufficient(ctx context.Context, client *mixin.Client, id, raw string) (*mixin.SafeTransactionRequest, error) {
	for {
		req, err := client.SafeCreateTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
			RequestID:      id,
			RawTransaction: raw,
		})
		logger.Verbosef("common.mixin.SafeCreateTransactionRequest(%s, %s) => %v %v\n", id, raw, req, err)
		if err != nil && mtg.CheckRetryableError(err) {
			time.Sleep(3 * time.Second)
			continue
		}
		return req, err
	}
}

func SignTransactionUntilSufficient(ctx context.Context, client *mixin.Client, requestId, raw string, views []mixinnet.Key, spendPrivateKey string) (*mixin.SafeTransactionRequest, error) {
	key, err := mixinnet.KeyFromString(spendPrivateKey)
	if err != nil {
		return nil, err
	}
	ver, err := mixinnet.TransactionFromRaw(raw)
	if err != nil {
		return nil, err
	}
	err = mixin.SafeSignTransaction(ver, key, views, uint16(0))
	if err != nil {
		return nil, err
	}
	signedRaw, err := ver.Dump()
	if err != nil {
		return nil, err
	}
	for {
		req, err := client.SafeSubmitTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
			RequestID:      requestId,
			RawTransaction: signedRaw,
		})
		logger.Verbosef("common.mixin.SafeSubmitTransactionRequest(%s, %s) => %v %v\n", requestId, signedRaw, req, err)
		if err != nil && mtg.CheckRetryableError(err) {
			time.Sleep(3 * time.Second)
			continue
		}
		return req, err
	}
}

func SignMultisigUntilSufficient(ctx context.Context, client *mixin.Client, requestId, raw string, views []mixinnet.Key, members []string, spendPrivateKey string) (*mixin.SafeMultisigRequest, error) {
	key, err := mixinnet.KeyFromString(spendPrivateKey)
	if err != nil {
		return nil, err
	}
	index := slices.Index(members, client.ClientID)
	if index == -1 {
		return nil, fmt.Errorf("invalid signer index: %d", index)
	}

	ver, err := mixinnet.TransactionFromRaw(raw)
	if err != nil {
		return nil, err
	}
	err = mixin.SafeSignTransaction(ver, key, views, uint16(index))
	if err != nil {
		return nil, err
	}
	signedRaw, err := ver.Dump()
	if err != nil {
		return nil, err
	}
	for {
		req, err := client.SafeSignMultisigRequest(ctx, &mixin.SafeTransactionRequestInput{
			RequestID:      requestId,
			RawTransaction: signedRaw,
		})
		logger.Verbosef("common.mixin.SafeSignMultisigRequest(%s, %s) => %v %v\n", requestId, signedRaw, req, err)
		if err != nil && mtg.CheckRetryableError(err) {
			time.Sleep(3 * time.Second)
			continue
		}
		return req, err
	}
}

func SafeReadTransactionRequestUntilSufficient(ctx context.Context, client *mixin.Client, id string) (*mixin.SafeTransactionRequest, error) {
	for {
		req, err := client.SafeReadTransactionRequest(ctx, id)
		logger.Verbosef("common.mixin.SafeReadTransactionRequest(%s) => %v %v\n", id, req, err)
		if err == nil {
			return req, nil
		}
		if mtg.CheckRetryableError(err) {
			time.Sleep(3 * time.Second)
			continue
		}
		if mixin.IsErrorCodes(err, 404) {
			return nil, nil
		}
		return nil, err
	}
}

func SafeReadMultisigRequestUntilSufficient(ctx context.Context, client *mixin.Client, id string) (*mixin.SafeMultisigRequest, error) {
	for {
		req, err := client.SafeReadMultisigRequests(ctx, id)
		logger.Verbosef("common.mixin.SafeReadMultisigRequests(%s) => %v %v", id, req, err)
		if err == nil {
			return req, nil
		}
		if mtg.CheckRetryableError(err) {
			time.Sleep(3 * time.Second)
			continue
		}
		if mixin.IsErrorCodes(err, 404) {
			return nil, nil
		}
		return nil, err
	}
}

func SafeAssetBalance(ctx context.Context, client *mixin.Client, members []string, threshold int, assetId string) (*common.Integer, error) {
	utxos, err := listSafeUtxosUntilSufficient(ctx, client, members, threshold, assetId)
	if err != nil {
		return nil, err
	}
	var total common.Integer
	for _, o := range utxos {
		amt := common.NewIntegerFromString(o.Amount.String())
		total = total.Add(amt)
	}
	return &total, nil
}

func ReadUsers(ctx context.Context, client *mixin.Client, ids []string) ([]*mixin.User, error) {
	if CheckTestEnvironment(ctx) {
		var us []*mixin.User
		for _, u := range ids {
			us = append(us, &mixin.User{
				UserID:  u,
				HasSafe: true,
			})
		}
		return us, nil
	}
	for {
		us, err := client.ReadUsers(ctx, ids...)
		logger.Verbosef("common.mixin.ReadUsers(%v) => %v %v\n", ids, us, err)
		if err == nil {
			return us, nil
		}
		if mtg.CheckRetryableError(err) {
			time.Sleep(3 * time.Second)
			continue
		}
		return nil, err
	}
}
