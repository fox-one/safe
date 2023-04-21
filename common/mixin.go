package common

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MixinNetwork/bot-api-go-client"
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/trusted-group/mtg"
	"github.com/fox-one/mixin-sdk-go"
	"github.com/shopspring/decimal"
)

// TODO the output should include the snapshot signature, then it can just be
// verified against the active kernel nodes public key
func VerifyKernelTransaction(rpc string, out *mtg.Output, timeout time.Duration) error {
	signed, err := ReadKernelTransaction(rpc, out.TransactionHash)
	logger.Printf("common.readKernelTransaction(%s) => %v %v", out.TransactionHash, signed, err)

	if (err != nil || signed == nil) && out.CreatedAt.Add(timeout).After(time.Now()) {
		time.Sleep(time.Second)
		return VerifyKernelTransaction(rpc, out, timeout)
	} else if err != nil || signed == nil {
		return fmt.Errorf("common.VerifyKernelTransaction(%v) not found %v", out, err)
	}

	if !strings.Contains(string(signed.Extra), out.Memo) && !strings.Contains(hex.EncodeToString(signed.Extra), out.Memo) {
		return fmt.Errorf("common.VerifyKernelTransaction(%v) memo mismatch %x", out, signed.Extra)
	}
	if signed.Asset != crypto.NewHash([]byte(out.AssetID)) {
		return fmt.Errorf("common.VerifyKernelTransaction(%v) asset mismatch %s", out, signed.Asset)
	}
	if len(signed.Outputs) < out.OutputIndex+1 {
		return fmt.Errorf("common.VerifyKernelTransaction(%v) output mismatch %d", out, len(signed.Outputs))
	}
	if a, _ := decimal.NewFromString(signed.Outputs[out.OutputIndex].Amount.String()); !a.Equal(out.Amount) {
		return fmt.Errorf("common.VerifyKernelTransaction(%v) amount mismatch %s", out, a)
	}

	return nil
}

func CheckMixinDomainAddress(rpc string, chainId, address string) (bool, error) {
	inAddress, err := bot.ExternalAdddressCheck(context.Background(), chainId, address, "")
	if err != nil && !strings.Contains(err.Error(), "30102") {
		return false, fmt.Errorf("bot.ExternalAdddressCheck(%s) => %v", address, err)
	}
	return inAddress != nil && inAddress.Fee == "0", nil
}

func SendTransactionUntilSufficient(ctx context.Context, client *mixin.Client, assetId string, receivers []string, threshold int, amount decimal.Decimal, memo, traceId string, pin string) error {
	for {
		err := SendTransaction(ctx, client, assetId, receivers, threshold, amount, memo, traceId, pin)
		if mixin.IsErrorCodes(err, 30103) {
			time.Sleep(7 * time.Second)
			continue
		}
		if err != nil && strings.Contains(err.Error(), "Client.Timeout exceeded") {
			time.Sleep(7 * time.Second)
			continue
		}
		return err
	}
}

func SendTransaction(ctx context.Context, client *mixin.Client, assetId string, receivers []string, threshold int, amount decimal.Decimal, memo, traceId string, pin string) error {
	logger.Printf("SendTransaction(%s, %v, %d, %s, %s, %s)", assetId, receivers, threshold, amount, memo, traceId)
	input := &mixin.TransferInput{
		AssetID: assetId,
		Amount:  amount,
		TraceID: traceId,
		Memo:    memo,
	}
	if len(receivers) == 1 {
		input.OpponentID = receivers[0]
		_, err := client.Transfer(ctx, input, pin)
		return err
	}
	input.OpponentMultisig.Receivers = receivers
	input.OpponentMultisig.Threshold = uint8(threshold)
	_, err := client.Transaction(ctx, input, pin)
	return err
}

func ReadKernelTransaction(rpc string, tx crypto.Hash) (*common.VersionedTransaction, error) {
	raw, err := callMixinRPC(rpc, "gettransaction", []any{tx.String()})
	if err != nil {
		return nil, err
	}
	var signed map[string]any
	err = json.Unmarshal(raw, &signed)
	if err != nil {
		return nil, err
	}
	if signed["hex"] == nil {
		return nil, fmt.Errorf("transaction %s not found in kernel", tx)
	}
	hex, err := hex.DecodeString(signed["hex"].(string))
	if err != nil {
		return nil, err
	}
	return common.UnmarshalVersionedTransaction(hex)
}

func callMixinRPC(node, method string, params []any) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}

	body, err := json.Marshal(map[string]any{
		"method": method,
		"params": params,
	})
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", node, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Close = true
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data  any `json:"data"`
		Error any `json:"error"`
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	err = dec.Decode(&result)
	if err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("ERROR %s", result.Error)
	}

	return json.Marshal(result.Data)
}
