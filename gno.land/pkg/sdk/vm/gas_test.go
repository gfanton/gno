package vm

import (
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/gnovm"
	bft "github.com/gnolang/gno/tm2/pkg/bft/types"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/sdk"
	"github.com/gnolang/gno/tm2/pkg/sdk/auth"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/gnolang/gno/tm2/pkg/store"
	"github.com/stretchr/testify/assert"
)

// Gas for entire tx is consumed in both CheckTx and DeliverTx.
// Gas for executing VM tx (VM CPU and Store Access in bytes) is consumed in DeliverTx.
// Gas for balance checking, message size checking, and signature verification is consumed (deducted) in checkTx.

// Insufficient gas for a successful message.

func TestAddPkgDeliverTxInsuffGas(t *testing.T) {
	success := true
	ctx, tx, vmHandler := setupAddPkg(success)

	ctx = ctx.WithMode(sdk.RunTxModeDeliver)
	simulate := false
	tx.Fee.GasWanted = 3000
	gctx := auth.SetGasMeter(simulate, ctx, tx.Fee.GasWanted)
	// Has to be set up after gas meter in the context; so the stores are
	// correctly wrapped in gas stores.
	gctx = vmHandler.vm.MakeGnoTransactionStore(gctx)

	var res sdk.Result
	abort := false

	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case store.OutOfGasException:
				res.Error = sdk.ABCIError(std.ErrOutOfGas(""))
				abort = true
			default:
				t.Errorf("should panic on OutOfGasException only")
			}
			assert.True(t, abort)
			assert.False(t, res.IsOK())
			gasCheck := gctx.GasMeter().GasConsumed()
			assert.Equal(t, int64(3231), gasCheck)
		} else {
			t.Errorf("should panic")
		}
	}()
	msgs := tx.GetMsgs()
	res = vmHandler.Process(gctx, msgs[0])
}

// Enough gas for a successful message.
func TestAddPkgDeliverTx(t *testing.T) {
	success := true
	ctx, tx, vmHandler := setupAddPkg(success)

	var simulate bool

	ctx = ctx.WithMode(sdk.RunTxModeDeliver)
	simulate = false
	tx.Fee.GasWanted = 500000
	gctx := auth.SetGasMeter(simulate, ctx, tx.Fee.GasWanted)
	gctx = vmHandler.vm.MakeGnoTransactionStore(gctx)
	msgs := tx.GetMsgs()
	res := vmHandler.Process(gctx, msgs[0])
	gasDeliver := gctx.GasMeter().GasConsumed()

	assert.True(t, res.IsOK())

	// NOTE: let's try to keep this bellow 100_000 :)
	assert.Equal(t, int64(92825), gasDeliver)
}

// Enough gas for a failed transaction.
func TestAddPkgDeliverTxFailed(t *testing.T) {
	success := false
	ctx, tx, vmHandler := setupAddPkg(success)

	var simulate bool

	ctx = ctx.WithMode(sdk.RunTxModeDeliver)
	simulate = false
	tx.Fee.GasWanted = 500000
	gctx := auth.SetGasMeter(simulate, ctx, tx.Fee.GasWanted)
	gctx = vmHandler.vm.MakeGnoTransactionStore(gctx)
	msgs := tx.GetMsgs()
	res := vmHandler.Process(gctx, msgs[0])
	gasDeliver := gctx.GasMeter().GasConsumed()

	assert.False(t, res.IsOK())
	assert.Equal(t, int64(2231), gasDeliver)
}

// Not enough gas for a failed transaction.
func TestAddPkgDeliverTxFailedNoGas(t *testing.T) {
	success := false
	ctx, tx, vmHandler := setupAddPkg(success)

	var simulate bool

	ctx = ctx.WithMode(sdk.RunTxModeDeliver)
	simulate = false
	tx.Fee.GasWanted = 2230
	gctx := auth.SetGasMeter(simulate, ctx, tx.Fee.GasWanted)
	gctx = vmHandler.vm.MakeGnoTransactionStore(gctx)

	var res sdk.Result
	abort := false

	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case store.OutOfGasException:
				res.Error = sdk.ABCIError(std.ErrOutOfGas(""))
				abort = true
			default:
				t.Errorf("should panic on OutOfGasException only")
			}
			assert.True(t, abort)
			assert.False(t, res.IsOK())
			gasCheck := gctx.GasMeter().GasConsumed()
			assert.Equal(t, int64(2231), gasCheck)
		} else {
			t.Errorf("should panic")
		}
	}()

	msgs := tx.GetMsgs()
	res = vmHandler.Process(gctx, msgs[0])
}

// Set up a test env for both a successful and a failed tx.
func setupAddPkg(success bool) (sdk.Context, sdk.Tx, vmHandler) {
	// setup
	env := setupTestEnv()
	ctx := env.ctx
	// conduct base gas meter tests from a non-genesis block since genesis block use infinite gas meter instead.
	ctx = ctx.WithBlockHeader(&bft.Header{Height: int64(1)})
	vmHandler := NewHandler(env.vmk)
	// Create an account  with 10M ugnot (10gnot)
	addr := crypto.AddressFromPreimage([]byte("test1"))
	acc := env.acck.NewAccountWithAddress(ctx, addr)
	env.acck.SetAccount(ctx, acc)
	env.bank.SetCoins(ctx, addr, std.MustParseCoins(ugnot.ValueString(10000000)))
	// success message
	var files []*gnovm.MemFile
	if success {
		files = []*gnovm.MemFile{
			{
				Name: "hello.gno",
				Body: `package hello

func Echo() string {
  return "hello world"
}`,
			},
		}
	} else {
		// failed message
		files = []*gnovm.MemFile{
			{
				Name: "hello.gno",
				Body: `package hello

func Echo() UnknowType {
  return "hello world"
}`,
			},
		}
	}

	pkgPath := "gno.land/r/hello"
	// create messages and a transaction
	msg := NewMsgAddPackage(addr, pkgPath, files)
	msgs := []std.Msg{msg}
	fee := std.NewFee(500000, std.MustParseCoin(ugnot.ValueString(1)))
	tx := std.NewTx(msgs, fee, []std.Signature{}, "")

	return ctx, tx, vmHandler
}
