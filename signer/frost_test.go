package signer

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/safe/common"
	"github.com/MixinNetwork/trusted-group/mtg"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestFROSTSigner(t *testing.T) {
	require := require.New(t)
	ctx, nodes := TestPrepare(require)

	public := testFROSTKeyGen(ctx, require, nodes, common.CurveEdwards25519Default)
	testFROSTSign(ctx, require, nodes, public, []byte("mixin"), common.CurveEdwards25519Default)

	public = testFROSTKeyGen(ctx, require, nodes, common.CurveSecp256k1SchnorrBitcoin)
	testFROSTSign(ctx, require, nodes, public, []byte("mixin"), common.CurveSecp256k1SchnorrBitcoin)
}

func testFROSTKeyGen(ctx context.Context, require *require.Assertions, nodes []*Node, curve uint8) string {
	sid := common.UniqueId("keygen", fmt.Sprint(curve))
	for i := 0; i < 4; i++ {
		node := nodes[i]
		op := &common.Operation{
			Type:  common.OperationTypeKeygenInput,
			Id:    sid,
			Curve: curve,
		}
		memo := mtg.EncodeMixinExtra("", sid, string(node.encryptOperation(op)))
		out := &mtg.Output{
			AssetID:         node.conf.KeeperAssetId,
			Memo:            memo,
			Amount:          decimal.NewFromInt(1),
			TransactionHash: crypto.NewHash([]byte(op.Id)),
			CreatedAt:       time.Now(),
		}

		msg := common.MarshalJSONOrPanic(out)
		network := node.network.(*testNetwork)
		network.mtgChannel(nodes[i].id) <- msg
	}

	var public string
	for _, node := range nodes {
		op := testWaitOperation(ctx, node, sid)
		logger.Verbosef("testWaitOperation(%s, %s) => %v\n", node.id, sid, op)
		require.Equal(common.OperationTypeKeygenOutput, int(op.Type))
		require.Equal(sid, op.Id)
		require.Equal(curve, op.Curve)
		require.Len(op.Public, 64)
		require.Len(op.Extra, 34)
		require.Equal(op.Extra[0], byte(common.RequestRoleSigner))
		require.Equal(op.Extra[33], byte(common.RequestFlagNone))
		public = op.Public
	}
	return public
}

func testFROSTSign(ctx context.Context, require *require.Assertions, nodes []*Node, public string, msg []byte, crv uint8) []byte {
	node := nodes[0]
	sid := common.UniqueId("sign", fmt.Sprintf("%d:%x", crv, msg))
	fingerPath := append(common.Fingerprint(public), []byte{0, 0, 0, 0}...)
	sop := &common.Operation{
		Type:   common.OperationTypeSignInput,
		Id:     sid,
		Curve:  crv,
		Public: hex.EncodeToString(fingerPath),
		Extra:  msg,
	}
	memo := mtg.EncodeMixinExtra("", sid, string(node.encryptOperation(sop)))
	out := &mtg.Output{
		AssetID:         node.conf.KeeperAssetId,
		Memo:            memo,
		Amount:          decimal.NewFromInt(1),
		TransactionHash: crypto.NewHash([]byte(sop.Id)),
		CreatedAt:       time.Now(),
	}
	op := TestProcessOutput(ctx, require, nodes, out, sid)

	require.Equal(common.OperationTypeSignOutput, int(op.Type))
	require.Equal(sid, op.Id)
	require.Equal(crv, op.Curve)
	require.Len(op.Public, 64)
	require.Len(op.Extra, 64)
	return op.Extra
}
