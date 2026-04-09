// misc/gnodig/cmd/gnodig/main.go
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
	"github.com/gnolang/gno/misc/gnodig/internal/doctor"
	"github.com/gnolang/gno/misc/gnodig/internal/driver"
	"github.com/gnolang/gno/misc/gnodig/internal/logengine"
	mcpserver "github.com/gnolang/gno/misc/gnodig/internal/mcp"
	"github.com/gnolang/gno/misc/gnodig/internal/nodedata"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = cmdServe(os.Args[2:])
	case "node":
		err = cmdNode(os.Args[2:])
	case "block":
		err = cmdBlock(os.Args[2:])
	case "logs":
		err = cmdLogs(os.Args[2:])
	case "query":
		err = cmdQuery(os.Args[2:])
	case "compare":
		err = cmdCompare(os.Args[2:])
	case "data":
		err = cmdData(os.Args[2:])
	case "realm":
		err = cmdRealm(os.Args[2:])
	case "account":
		err = cmdAccount(os.Args[2:])
	case "genesis":
		err = cmdGenesis(os.Args[2:])
	case "doctor":
		err = cmdDoctor(os.Args[2:])
	case "probe":
		err = cmdProbe(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: gnodig <serve|node|block|logs|query|data|compare|realm|account|genesis|doctor|probe>")
}

// ---- serve

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	srv := mcpserver.NewServer(mcpserver.ServerConfig{})
	defer srv.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx, &sdkmcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

// ---- node

func cmdNode(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gnodig node <overview>")
	}

	switch args[0] {
	case "overview":
		return cmdNodeOverview(args[1:])
	default:
		return fmt.Errorf("unknown node subcommand: %s", args[0])
	}
}

func cmdNodeOverview(args []string) error {
	fs := flag.NewFlagSet("node overview", flag.ContinueOnError)
	target := fs.String("target", "", "RPC URL (required, e.g. http://localhost:26657)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	c := chainrpc.New(*target)
	ctx := context.Background()

	return writeJSON(chainrpc.FetchOverview(ctx, c))
}

// ---- block

func cmdBlock(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gnodig block <inspect>")
	}

	switch args[0] {
	case "inspect":
		return cmdBlockInspect(args[1:])
	default:
		return fmt.Errorf("unknown block subcommand: %s", args[0])
	}
}

func cmdBlockInspect(args []string) error {
	fs := flag.NewFlagSet("block inspect", flag.ContinueOnError)
	target := fs.String("target", "", "RPC URL (required, e.g. http://localhost:26657)")
	height := fs.Int64("height", 0, "block height (0 = latest)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	c := chainrpc.New(*target)
	ctx := context.Background()

	return writeJSON(chainrpc.FetchBlockInspect(ctx, c, *height))
}

// ---- logs

func cmdLogs(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gnodig logs <search|summary|navigate>")
	}

	switch args[0] {
	case "search":
		return cmdLogsSearch(args[1:])
	case "summary":
		return cmdLogsSummary(args[1:])
	case "navigate":
		return cmdLogsNavigate(args[1:])
	default:
		return fmt.Errorf("unknown logs subcommand: %s", args[0])
	}
}

func cmdLogsSearch(args []string) error {
	fs := flag.NewFlagSet("logs search", flag.ContinueOnError)
	target := fs.String("target", "", "log source URI (required)")
	text := fs.String("text", "", "substring to match")
	field := fs.String("field", "", "JSON field name to match")
	value := fs.String("value", "", "expected value for --field")
	level := fs.String("level", "", "minimum level (debug|info|warn|error)")
	module := fs.String("module", "", "include only this module")
	excludeModule := fs.String("exclude-module", "", "exclude this module")
	limit := fs.Int("limit", 100, "max results")
	dedup := fs.Bool("deduplicate", false, "group identical messages and return counts")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	src, err := openLogSource(*target)
	if err != nil {
		return err
	}
	defer src.Close()

	cache := logengine.NewCache()
	ctx := context.Background()

	idx, err := cache.GetOrBuild(ctx, src, logengine.ScanConfig{})
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}

	q := logengine.Query{
		Text:          *text,
		Field:         *field,
		Value:         *value,
		Module:        *module,
		ExcludeModule: *excludeModule,
		Limit:         *limit,
	}
	if *level != "" {
		q.Level = logengine.ParseLevelName(*level)
		if q.Level == 0 {
			return fmt.Errorf("unknown level %q: use debug, info, warn, or error", *level)
		}
	}

	entries, err := logengine.Search(ctx, src, idx, q)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if *dedup {
		return writeJSON(logengine.Deduplicate(entries))
	}
	return writeJSON(entries)
}

func cmdLogsSummary(args []string) error {
	fs := flag.NewFlagSet("logs summary", flag.ContinueOnError)
	target := fs.String("target", "", "log source URI (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	src, err := openLogSource(*target)
	if err != nil {
		return err
	}
	defer src.Close()

	cache := logengine.NewCache()
	ctx := context.Background()

	idx, err := cache.GetOrBuild(ctx, src, logengine.ScanConfig{})
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}

	summary := logengine.Summarize(idx)
	return writeJSON(summary)
}

func cmdLogsNavigate(args []string) error {
	fs := flag.NewFlagSet("logs navigate", flag.ContinueOnError)
	target := fs.String("target", "", "log source URI (required)")
	timeStr := fs.String("time", "", "seek to time (RFC3339 or nanoseconds); mutually exclusive with --offset")
	offset := fs.Int64("offset", -1, "byte offset (use next_offset from a previous call); mutually exclusive with --time")
	count := fs.Int("count", 20, "lines to read (max 100)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	hasTime := *timeStr != ""
	hasOffset := *offset >= 0

	if hasTime == hasOffset {
		return fmt.Errorf("provide exactly one of --time or --offset")
	}

	src, err := openLogSource(*target)
	if err != nil {
		return err
	}
	defer src.Close()

	cache := logengine.NewCache()
	ctx := context.Background()

	idx, err := cache.GetOrBuild(ctx, src, logengine.ScanConfig{})
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}

	r, _, err := src.Reader(ctx)
	if err != nil {
		return fmt.Errorf("open reader: %w", err)
	}

	n := *count
	if n <= 0 {
		n = 20
	}
	if n > 100 {
		n = 100
	}

	var startOffset int64
	if hasOffset {
		startOffset = *offset
	}

	cursor := logengine.NewCursor(r, idx, startOffset)

	if hasTime {
		ts, err := logengine.ParseTimestamp(*timeStr)
		if err != nil {
			return err
		}
		cursor.SeekTime(ts)
	}

	entries, err := cursor.Read(n)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	return writeJSON(logengine.NavigateResult{
		Entries:    entries,
		NextOffset: cursor.Offset(),
	})
}

// ---- query

func cmdQuery(args []string) error {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	target := fs.String("target", "", "RPC URL (required, e.g. http://localhost:26657)")
	method := fs.String("method", "", "RPC method: tx, genesis, abci_query, blockchain (required)")
	// tx
	hash := fs.String("hash", "", "tx hash (hex, for --method tx)")
	// abci_query
	path := fs.String("path", "", "ABCI query path (for --method abci_query)")
	data := fs.String("data", "", "ABCI query data (hex, for --method abci_query)")
	abciHeight := fs.Int64("height", 0, "block height for ABCI query or blockchain range")
	// blockchain
	minHeight := fs.Int64("min-height", 0, "min height (for --method blockchain)")
	maxHeight := fs.Int64("max-height", 0, "max height (for --method blockchain)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}
	if *method == "" {
		return fmt.Errorf("--method is required (tx, genesis, abci_query, blockchain)")
	}

	c := chainrpc.New(*target)
	ctx := context.Background()

	switch *method {
	case "tx":
		if *hash == "" {
			return fmt.Errorf("--hash is required for method tx")
		}
		hashStr := strings.TrimPrefix(*hash, "0x")
		hashBytes, err := hex.DecodeString(hashStr)
		if err != nil {
			return fmt.Errorf("invalid hash %q: %w", *hash, err)
		}
		result, err := c.Tx(ctx, hashBytes)
		if err != nil {
			return fmt.Errorf("query tx: %w", err)
		}
		fmt.Println(result.Raw)

	case "genesis":
		result, err := c.Genesis(ctx)
		if err != nil {
			return fmt.Errorf("query genesis: %w", err)
		}
		fmt.Println(result.Raw)

	case "abci_query":
		if *path == "" {
			return fmt.Errorf("--path is required for method abci_query")
		}
		var dataBytes []byte
		if *data != "" {
			var err error
			dataBytes, err = hex.DecodeString(strings.TrimPrefix(*data, "0x"))
			if err != nil {
				return fmt.Errorf("invalid data %q: %w", *data, err)
			}
		}
		result, err := c.ABCIQuery(ctx, *path, dataBytes, *abciHeight, false)
		if err != nil {
			return fmt.Errorf("query abci_query: %w", err)
		}
		fmt.Println(result.Raw)

	case "blockchain":
		result, err := c.BlockRange(ctx, *minHeight, *maxHeight)
		if err != nil {
			return fmt.Errorf("query blockchain: %w", err)
		}
		fmt.Println(result.Raw)

	default:
		return fmt.Errorf("unknown method %q: use tx, genesis, abci_query, or blockchain", *method)
	}

	return nil
}

// ---- data

func cmdData(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gnodig data <open|block|wal|state|tx>")
	}

	switch args[0] {
	case "open":
		return cmdDataOpen(args[1:])
	case "block":
		return cmdDataBlock(args[1:])
	case "wal":
		return cmdDataWAL(args[1:])
	case "state":
		return cmdDataState(args[1:])
	case "tx":
		return cmdDataTx(args[1:])
	default:
		return fmt.Errorf("unknown data subcommand: %s", args[0])
	}
}

func cmdDataOpen(args []string) error {
	fs := flag.NewFlagSet("data open", flag.ContinueOnError)
	target := fs.String("target", "", "path to gnoland data directory (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	dd, err := nodedata.Open(*target)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer dd.Close()

	ov, err := dd.Overview()
	if err != nil {
		return fmt.Errorf("overview: %w", err)
	}

	return writeJSON(ov)
}

func cmdDataBlock(args []string) error {
	fs := flag.NewFlagSet("data block", flag.ContinueOnError)
	target := fs.String("target", "", "path to gnoland data directory (required)")
	height := fs.Int64("height", 0, "block height (0 = latest)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	dd, err := nodedata.Open(*target)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer dd.Close()

	h := *height
	if h == 0 {
		h = dd.BlockStore().Height()
	}

	detail, err := dd.Block(h)
	if err != nil {
		return fmt.Errorf("block: %w", err)
	}

	return writeJSON(detail)
}

func cmdDataWAL(args []string) error {
	fs := flag.NewFlagSet("data wal", flag.ContinueOnError)
	target := fs.String("target", "", "path to gnoland data directory (required)")
	height := fs.Int64("height", 0, "block height (required)")
	mode := fs.String("mode", "summary", "output mode: summary or raw")
	round := fs.Int("round", -1, "filter by consensus round (-1 = all)")
	msgType := fs.String("type", "", "filter by message type: proposal, prevote, precommit, timeout")
	limit := fs.Int("limit", 50, "max messages in raw mode")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}
	if *height == 0 {
		return fmt.Errorf("--height is required")
	}

	dd, err := nodedata.Open(*target)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer dd.Close()

	switch *mode {
	case "raw":
		var rp *int
		if *round >= 0 {
			rp = round
		}
		if rp == nil && *msgType == "" {
			return fmt.Errorf("raw mode requires at least one of --round or --type")
		}
		detail, err := dd.WALFiltered(*height, rp, *msgType, *limit)
		if err != nil {
			return fmt.Errorf("wal: %w", err)
		}
		return writeJSON(detail)

	case "summary":
		summary, err := dd.WALSummary(*height)
		if err != nil {
			return fmt.Errorf("wal: %w", err)
		}
		return writeJSON(summary)

	default:
		return fmt.Errorf("unknown mode %q: use summary or raw", *mode)
	}
}

func cmdDataState(args []string) error {
	fs := flag.NewFlagSet("data state", flag.ContinueOnError)
	target := fs.String("target", "", "path to gnoland data directory (required)")
	height := fs.Int64("height", 0, "state version / block height (0 = latest)")
	path := fs.String("path", "", "package/realm path (e.g. gno.land/r/demo/boards)")
	account := fs.String("account", "", "account address (e.g. g1jg8...)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	hasPath := *path != ""
	hasAccount := *account != ""

	if hasPath == hasAccount {
		return fmt.Errorf("provide exactly one of --path or --account")
	}

	dd, err := nodedata.Open(*target)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer dd.Close()

	if hasAccount {
		result, err := dd.AccountQuery(*height, *account)
		if err != nil {
			return fmt.Errorf("account query: %w", err)
		}
		return writeJSON(result)
	}

	result, err := dd.PackageQuery(*height, *path)
	if err != nil {
		return fmt.Errorf("package query: %w", err)
	}
	return writeJSON(result)
}

func cmdDataTx(args []string) error {
	fs := flag.NewFlagSet("data tx", flag.ContinueOnError)
	target := fs.String("target", "", "path to gnoland data directory (required)")
	hash := fs.String("hash", "", "transaction hash (hex)")
	height := fs.Int64("height", 0, "block height (use with --index)")
	index := fs.Int("index", 0, "transaction index within block (use with --height)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	hasHash := *hash != ""
	hasHeight := *height > 0

	if hasHash == hasHeight {
		return fmt.Errorf("provide either --hash or --height+--index, not both")
	}

	dd, err := nodedata.Open(*target)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer dd.Close()

	if hasHash {
		hashStr := strings.TrimPrefix(*hash, "0x")
		detail, err := dd.TxByHash(hashStr, nodedata.TxHashScanWindow)
		if err != nil {
			return fmt.Errorf("tx by hash: %w", err)
		}
		return writeJSON(detail)
	}

	detail, err := dd.TxByIndex(*height, *index)
	if err != nil {
		return fmt.Errorf("tx by index: %w", err)
	}
	return writeJSON(detail)
}

// ---- compare

func cmdCompare(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	height := fs.Int64("height", 0, "block height to compare (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *height == 0 {
		return fmt.Errorf("--height is required")
	}

	paths := fs.Args()
	if len(paths) < 2 || len(paths) > 5 {
		return fmt.Errorf("provide 2-5 data directory paths as arguments")
	}

	dirs := make([]*nodedata.DataDir, len(paths))
	for i, p := range paths {
		dd, err := nodedata.Open(p)
		if err != nil {
			// Close already-opened dirs on failure.
			for j := range i {
				dirs[j].Close()
			}
			return fmt.Errorf("open %q: %w", p, err)
		}
		dirs[i] = dd
	}
	defer func() {
		for _, dd := range dirs {
			dd.Close()
		}
	}()

	result, err := nodedata.CompareBlocks(dirs, *height)
	if err != nil {
		return fmt.Errorf("compare: %w", err)
	}

	return writeJSON(result)
}

// ---- realm

func cmdRealm(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gnodig realm <eval|inspect|source>")
	}

	switch args[0] {
	case "eval":
		return cmdRealmEval(args[1:])
	case "inspect":
		return cmdRealmInspect(args[1:])
	case "source":
		return cmdRealmSource(args[1:])
	default:
		return fmt.Errorf("unknown realm subcommand: %s", args[0])
	}
}

func cmdRealmEval(args []string) error {
	fs := flag.NewFlagSet("realm eval", flag.ContinueOnError)
	target := fs.String("target", "", "RPC URL (required)")
	pkg := fs.String("pkg", "", "package path (required)")
	expr := fs.String("expr", "", "expression to evaluate (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" || *pkg == "" || *expr == "" {
		return fmt.Errorf("--target, --pkg, and --expr are required")
	}

	c := chainrpc.New(*target)
	ctx := context.Background()

	result, err := c.EvalExpression(ctx, *pkg, *expr)
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	return writeJSON(result)
}

func cmdRealmInspect(args []string) error {
	fs := flag.NewFlagSet("realm inspect", flag.ContinueOnError)
	target := fs.String("target", "", "RPC URL (required)")
	pkg := fs.String("pkg", "", "package path (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" || *pkg == "" {
		return fmt.Errorf("--target and --pkg are required")
	}

	c := chainrpc.New(*target)
	ctx := context.Background()

	result, err := c.InspectPackage(ctx, *pkg)
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}
	return writeJSON(result)
}

func cmdRealmSource(args []string) error {
	fs := flag.NewFlagSet("realm source", flag.ContinueOnError)
	target := fs.String("target", "", "RPC URL (required)")
	pkg := fs.String("pkg", "", "package path (required)")
	file := fs.String("file", "", "file name (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" || *pkg == "" || *file == "" {
		return fmt.Errorf("--target, --pkg, and --file are required")
	}

	c := chainrpc.New(*target)
	ctx := context.Background()

	result, err := c.FetchSource(ctx, *pkg, *file)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	return writeJSON(result)
}

// ---- account

func cmdAccount(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gnodig account <info>")
	}

	switch args[0] {
	case "info":
		return cmdAccountInfo(args[1:])
	default:
		return fmt.Errorf("unknown account subcommand: %s", args[0])
	}
}

func cmdAccountInfo(args []string) error {
	fs := flag.NewFlagSet("account info", flag.ContinueOnError)
	target := fs.String("target", "", "RPC URL (required)")
	address := fs.String("address", "", "account address (required)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" || *address == "" {
		return fmt.Errorf("--target and --address are required")
	}

	c := chainrpc.New(*target)
	ctx := context.Background()

	result, err := c.QueryAccount(ctx, *address)
	if err != nil {
		return fmt.Errorf("account info: %w", err)
	}
	return writeJSON(result)
}

// ---- genesis

func cmdGenesis(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gnodig genesis <info>")
	}

	switch args[0] {
	case "info":
		return cmdGenesisInfo(args[1:])
	default:
		return fmt.Errorf("unknown genesis subcommand: %s", args[0])
	}
}

func cmdGenesisInfo(args []string) error {
	fs := flag.NewFlagSet("genesis info", flag.ContinueOnError)
	target := fs.String("target", "", "RPC URL (required)")
	mode := fs.String("mode", "summary", "mode: summary or balance")
	address := fs.String("address", "", "account address (for balance mode)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("--target is required")
	}

	c := chainrpc.New(*target)
	ctx := context.Background()
	cacheDir := ".debug"

	switch *mode {
	case "summary":
		result, err := c.FetchGenesisSummary(ctx, cacheDir)
		if err != nil {
			return fmt.Errorf("genesis info: %w", err)
		}
		return writeJSON(result)

	case "balance":
		if *address == "" {
			return fmt.Errorf("--address is required for balance mode")
		}
		result, err := c.LookupGenesisBalance(ctx, cacheDir, *address)
		if err != nil {
			return fmt.Errorf("genesis balance: %w", err)
		}
		return writeJSON(result)

	default:
		return fmt.Errorf("unknown mode %q: use summary or balance", *mode)
	}
}

// ---- doctor

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	target := fs.String("target", "", "RPC URL or data directory path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *target == "" {
		if fs.NArg() > 0 {
			*target = fs.Arg(0)
		} else {
			return fmt.Errorf("usage: gnodig doctor <target>")
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var rpcClient *chainrpc.Client
	var dd *nodedata.DataDir

	switch doctor.DetectTargetType(*target) {
	case doctor.TargetRPC:
		rpcClient = chainrpc.New(*target)
	case doctor.TargetDataDir:
		var err error
		dd, err = nodedata.Open(*target)
		if err != nil {
			return fmt.Errorf("open data dir: %w", err)
		}
		defer dd.Close()
	}

	report := doctor.RunDoctor(ctx, *target, rpcClient, dd)
	return writeJSON(report)
}

// ---- helpers

// openLogSource resolves a target URI using the standard log resolvers.
func openLogSource(target string) (driver.LogSource, error) {
	src, err := driver.ResolveURI(target, mcpserver.LogResolvers())
	if err != nil {
		return nil, fmt.Errorf("resolve target %q: %w", target, err)
	}
	return src, nil
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
