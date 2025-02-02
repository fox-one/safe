package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/MixinNetwork/safe/common"
	"github.com/MixinNetwork/safe/config"
	"github.com/MixinNetwork/safe/keeper"
	"github.com/MixinNetwork/trusted-group/mtg"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/gofrs/uuid/v5"
	"github.com/shopspring/decimal"
	"github.com/urfave/cli/v2"
)

// FIXME remove this
func mtgFixCache(ctx context.Context, path string) {
	_, memo := mtg.DecodeMixinExtraHEX("7665346b464152624d62653470336d59733239636b305037307948675144652d7073336e484975356866687376784c7042626b356f6844523049687853304e784c354d654451")
	db, err := common.OpenSQLite3Store(path, "")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	txn, err := db.BeginTx(ctx, nil)
	if err != nil {
		panic(err)
	}
	defer txn.Rollback()

	row := txn.QueryRowContext(ctx, "SELECT trace_id,app_id,opponent_app_id,state,asset_id,receivers,threshold,amount,refs,sequence,compaction,storage,storage_trace_id FROM transactions WHERE trace_id=?", "24a1cdf1-872d-3cc2-b826-e2a888b67303")
	var traceId, appId, opponentAppId, assetId, receivers, amount, refs, storageTraceId string
	var state, threshold, sequence int64
	var compaction, storage bool
	err = row.Scan(&traceId, &appId, &opponentAppId, &state, &assetId, &receivers, &threshold, &amount, &refs, &sequence, &compaction, &storage, &storageTraceId)
	if err == sql.ErrNoRows {
		return
	} else if err != nil {
		panic(err)
	}
	if state != 10 {
		panic(state)
	}

	r, err := txn.ExecContext(ctx, "DELETE FROM transactions WHERE trace_id=?", traceId)
	if err != nil {
		panic(err)
	}
	rac, err := r.RowsAffected()
	if err != nil || rac != 1 {
		panic(err)
	}

	r, err = txn.ExecContext(ctx, "INSERT INTO transactions (trace_id,app_id,opponent_app_id,state,asset_id,receivers,threshold,amount,refs,sequence,compaction,storage,memo,updated_at,storage_trace_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", "239f25cd-ee77-3534-b3dd-1c1815b581b6", appId, opponentAppId, state, assetId, receivers, threshold, amount, refs, sequence, compaction, storage, string(memo), time.Time{}, storageTraceId)
	if err != nil {
		panic(err)
	}
	rac, err = r.RowsAffected()
	if err != nil || rac != 1 {
		panic(err)
	}

	r, err = txn.ExecContext(ctx, "UPDATE outputs SET trace_id='239f25cd-ee77-3534-b3dd-1c1815b581b6' WHERE trace_id='24a1cdf1-872d-3cc2-b826-e2a888b67303'")
	if err != nil {
		panic(err)
	}
	rac, err = r.RowsAffected()
	if err != nil || rac != 1 {
		panic(err)
	}

	err = txn.Commit()
	if err != nil {
		panic(err)
	}
}

func KeeperBootCmd(c *cli.Context) error {
	ctx := context.Background()

	version := c.App.Metadata["VERSION"].(string)
	ua := fmt.Sprintf("Mixin Safe Keeper (%s)", version)
	resty := mixin.GetRestyClient()
	resty.SetTimeout(time.Second * 30)
	resty.SetHeader("User-Agent", ua)

	mc, err := config.ReadConfiguration(c.String("config"), "keeper")
	if err != nil {
		return err
	}
	mc.Keeper.MTG.GroupSize = 1
	mc.Signer.MTG.LoopWaitDuration = int64(time.Second)

	mtgFixCache(ctx, mc.Keeper.StoreDir+"/mtg.sqlite3")

	db, err := mtg.OpenSQLite3Store(mc.Keeper.StoreDir + "/mtg.sqlite3")
	if err != nil {
		return err
	}
	defer db.Close()

	group, err := mtg.BuildGroup(ctx, db, mc.Keeper.MTG)
	if err != nil {
		return err
	}
	group.EnableDebug()
	group.SetKernelRPC(mc.Keeper.MixinRPC)

	s := &mixin.Keystore{
		ClientID:          mc.Keeper.MTG.App.AppId,
		SessionID:         mc.Keeper.MTG.App.SessionId,
		SessionPrivateKey: mc.Keeper.MTG.App.SessionPrivateKey,
		ServerPublicKey:   mc.Keeper.MTG.App.ServerPublicKey,
	}
	client, err := mixin.NewFromKeystore(s)
	if err != nil {
		return err
	}
	me, err := client.UserMe(ctx)
	if err != nil {
		return err
	}
	key, err := mixinnet.ParseKeyWithPub(mc.Keeper.MTG.App.SpendPrivateKey, me.SpendPublicKey)
	if err != nil {
		return err
	}
	mc.Keeper.MTG.App.SpendPrivateKey = key.String()

	kd, err := keeper.OpenSQLite3Store(mc.Keeper.StoreDir + "/safe.sqlite3")
	if err != nil {
		return err
	}
	defer kd.Close()
	keeper := keeper.NewNode(kd, group, mc.Keeper, mc.Signer.MTG, client)
	keeper.Boot(ctx)

	if mmc := mc.Keeper.MonitorConversaionId; mmc != "" {
		go MonitorKeeper(ctx, db, kd, mc.Keeper, group, mmc, version)
	}

	group.AttachWorker(mc.Keeper.AppId, keeper)
	group.RegisterDepositEntry(mc.Keeper.AppId, mtg.DepositEntry{
		Destination: mc.Keeper.PolygonKeeperDepositEntry,
		Tag:         "",
	})
	group.Run(ctx)
	return nil
}

func KeeperFundRequest(c *cli.Context) error {
	mc, err := config.ReadConfiguration(c.String("config"), "keeper")
	if err != nil {
		return err
	}
	assetId := mc.Keeper.AssetId
	if c.String("asset") != "" {
		assetId = c.String("asset")
	}
	amount := decimal.RequireFromString(c.String("amount"))
	traceId := uuid.Must(uuid.NewV4()).String()
	return makeKeeperPaymentRequest(c.String("config"), assetId, amount, traceId, "")
}
