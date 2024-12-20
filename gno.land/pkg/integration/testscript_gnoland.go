package integration

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/gno.land/pkg/keyscli"
	"github.com/gnolang/gno/gno.land/pkg/log"
	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	bft "github.com/gnolang/gno/tm2/pkg/bft/types"
	"github.com/gnolang/gno/tm2/pkg/commands"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/crypto/bip39"
	"github.com/gnolang/gno/tm2/pkg/crypto/keys"
	"github.com/gnolang/gno/tm2/pkg/crypto/keys/client"
	tm2Log "github.com/gnolang/gno/tm2/pkg/log"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/rogpeppe/go-internal/testscript"
	"go.uber.org/zap/zapcore"
)

const (
	envKeyGenesis int = iota
	envKeyLogger
	envKeyPkgsLoader
)

type testNode struct {
	*exec.Cmd
	remoteAddr  string
	cfg         *gnoland.InMemoryNodeConfig
	nGnoKeyExec uint // Counter for execution of gnokey.
}

// NodesManager manages access to the nodes map with synchronization.
type NodesManager struct {
	nodes map[string]*testNode
	mu    sync.RWMutex
}

// NewNodesManager creates a new instance of NodesManager.
func NewNodesManager() *NodesManager {
	return &NodesManager{
		nodes: make(map[string]*testNode),
	}
}

func (nm *NodesManager) IsNodeRunning(sid string) bool {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	_, ok := nm.nodes[sid]
	return ok
}

// Get retrieves a node by its SID.
func (nm *NodesManager) Get(sid string) (*testNode, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	node, exists := nm.nodes[sid]
	return node, exists
}

// Set adds or updates a node in the map.
func (nm *NodesManager) Set(sid string, node *testNode) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.nodes[sid] = node
}

// Delete removes a node from the map.
func (nm *NodesManager) Delete(sid string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	delete(nm.nodes, sid)
}

func SetupGnolandTestscript(t *testing.T, p *testscript.Params) error {
	t.Helper()

	tmpdir := t.TempDir()

	gnoRootDir := gnoenv.RootDir()

	gnolandBuildDir := filepath.Join(tmpdir, "build")
	gnolandBin := filepath.Join(gnolandBuildDir, "gnoland")
	if err := buildGnoland(t, gnoRootDir, gnolandBin); err != nil {
		return fmt.Errorf("unable to build gnoland: %w", err)
	}

	nodesManager := NewNodesManager()

	// Store the original setup scripts for potential wrapping
	origSetup := p.Setup
	p.Setup = func(env *testscript.Env) error {
		// If there's an original setup, execute it
		if origSetup != nil {
			if err := origSetup(env); err != nil {
				return err
			}
		}

		tmpdir := t.TempDir()
		gnoHomeDir := filepath.Join(tmpdir, "gno")

		// Get `TESTWORK` environement variable from setup
		persistWorkDir, _ := strconv.ParseBool(env.Getenv("TESTWORK"))

		kb, err := keys.NewKeyBaseFromDir(gnoHomeDir)
		if err != nil {
			return err
		}

		var sid string
		{
			works := env.Getenv("WORK")
			sum := crc32.ChecksumIEEE([]byte(works))
			sid = strconv.FormatUint(uint64(sum), 16)
			env.Setenv("SID", sid)
		}

		var logger *slog.Logger
		{
			logger = tm2Log.NewNoopLogger()
			if persistWorkDir || os.Getenv("LOG_PATH_DIR") != "" {
				logname := fmt.Sprintf("txtar-gnoland-%s.log", sid)
				logger, err = getTestingLogger(env, logname)
				if err != nil {
					return fmt.Errorf("unable to setup logger: %w", err)
				}
			}

			env.Values[envKeyLogger] = logger
		}

		// Track new user balances added via the `adduser`
		// command and packages added with the `loadpkg` command.
		// This genesis will be use when node is started.
		genesis := &gnoland.GnoGenesisState{
			Balances: LoadDefaultGenesisBalanceFile(t, gnoRootDir),
			Params:   LoadDefaultGenesisParamFile(t, gnoRootDir),
			Txs:      []gnoland.TxWithMetadata{},
		}

		kb.CreateAccount(DefaultAccount_Name, DefaultAccount_Seed, "", "", 0, 0)
		env.Setenv("USER_SEED_"+DefaultAccount_Name, DefaultAccount_Seed)
		env.Setenv("USER_ADDR_"+DefaultAccount_Name, DefaultAccount_Address)

		env.Values[envKeyGenesis] = genesis
		env.Values[envKeyPkgsLoader] = newPkgsLoader()

		env.Setenv("GNOROOT", gnoRootDir)
		env.Setenv("GNOHOME", gnoHomeDir)

		env.Defer(func() {
			n, exist := nodesManager.Get(sid)
			if !exist {
				return
			}

			if err := n.Cmd.Process.Kill(); err != nil {
				env.T().Fatal(err.Error())
			}

		})

		return nil
	}

	cmds := map[string]func(ts *testscript.TestScript, neg bool, args []string){
		"gnoland":     gnolandCmd(t, nodesManager, gnolandBin, gnoRootDir),
		"gnokey":      gnokeyCmd(nodesManager),
		"adduser":     adduserCmd(nodesManager),
		"adduserfrom": adduserfromCmd(nodesManager),
		"patchpkg":    patchpkgCmd(),
		"loadpkg":     loadpkgCmd(gnoRootDir),
	}

	// Initialize cmds map if needed
	if p.Cmds == nil {
		p.Cmds = make(map[string]func(ts *testscript.TestScript, neg bool, args []string))
	}

	// Register gnoland command
	for cmd, call := range cmds {
		if _, exist := p.Cmds[cmd]; exist {
			panic(fmt.Errorf("unable register %q: command already exist", cmd))
		}

		p.Cmds[cmd] = call
	}

	return nil
}

func gnolandCmd(t *testing.T, nodesManager *NodesManager, gnolandBin, gnoRootDir string) func(ts *testscript.TestScript, neg bool, args []string) {
	t.Helper()

	return func(ts *testscript.TestScript, neg bool, args []string) {
		// logger := ts.Value(envKeyLogger).(*slog.Logger)
		sid := getNodeSID(ts)

		cmd, cmdargs := "", []string{}
		if len(args) > 0 {
			cmd, cmdargs = args[0], args[1:]
		}

		var err error
		switch cmd {
		case "start":
			if nodesManager.IsNodeRunning(sid) {
				err = fmt.Errorf("node already started")
				break
			}

			// XXX: this is a bit hacky, we should consider moving
			// gnoland into his own package to be able to use it
			// directly or use the config command for this.
			fs := flag.NewFlagSet("start", flag.ContinueOnError)
			nonVal := fs.Bool("non-validator", false, "set up node as a non-validator")
			if err := fs.Parse(cmdargs); err != nil {
				ts.Fatalf("unable to parse `gnoland start` flags: %s", err)
			}

			pkgs := ts.Value(envKeyPkgsLoader).(*pkgsLoader)
			creator := crypto.MustAddressFromString(DefaultAccount_Address)
			defaultFee := std.NewFee(50000, std.MustParseCoin(ugnot.ValueString(1000000)))
			pkgsTxs, err := pkgs.LoadPackages(creator, defaultFee, nil)
			if err != nil {
				ts.Fatalf("unable to load packages txs: %s", err)
			}

			cfg := TestingMinimalNodeConfig(gnoRootDir)
			genesis := ts.Value(envKeyGenesis).(*gnoland.GnoGenesisState)
			genesis.Txs = append(pkgsTxs, genesis.Txs...)

			cfg.Genesis.AppState = *genesis
			if *nonVal {
				pv := gnoland.NewMockedPrivValidator()
				cfg.Genesis.Validators = []bft.GenesisValidator{
					{
						Address: pv.GetPubKey().Address(),
						PubKey:  pv.GetPubKey(),
						Power:   10,
						Name:    "none",
					},
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			remoteAddr, cmd, err := ExecuteForkBinary(ctx, gnolandBin, &ForkConfig{
				RootDir:  gnoRootDir,
				TMConfig: cfg.TMConfig,
				Genesis:  NewMarshalableGenesisDoc(cfg.Genesis),
			})
			if err != nil {
				ts.Fatalf("unable to start the node: %s", err)
			}

			cfg.TMConfig.RPC.ListenAddress = remoteAddr
			nodesManager.Set(sid, &testNode{Cmd: cmd, remoteAddr: remoteAddr, cfg: cfg})

			ts.Setenv("RPC_ADDR", remoteAddr)

			fmt.Fprintln(ts.Stdout(), "node started successfully")
		case "restart":
			node, exists := nodesManager.Get(sid)
			if !exists {
				err = fmt.Errorf("node must be started before being restarted")
				break
			}

			if stopErr := node.Cmd.Process.Kill(); stopErr != nil {
				err = fmt.Errorf("error stopping node: %w", stopErr)
				break
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			newRemoteAddr, cmd, err := ExecuteForkBinary(ctx, gnolandBin, &ForkConfig{
				RootDir:  gnoRootDir,
				TMConfig: node.cfg.TMConfig,
				Genesis:  NewMarshalableGenesisDoc(node.cfg.Genesis),
			})
			if err != nil {
				ts.Fatalf("unable to start the node: %s", err)
			}

			node.cfg.TMConfig.RPC.ListenAddress = newRemoteAddr
			nodesManager.Set(sid, &testNode{Cmd: cmd, remoteAddr: newRemoteAddr, cfg: node.cfg})

			fmt.Fprintln(ts.Stdout(), "node restarted successfully")
		case "stop":
			node, exists := nodesManager.Get(sid)
			if !exists {
				err = fmt.Errorf("node not started cannot be stopped")
				break
			}
			if stopErr := node.Cmd.Process.Kill(); stopErr != nil {
				err = fmt.Errorf("error stopping node: %w", stopErr)
				fmt.Fprintln(ts.Stdout(), "node stopped successfully")
			}

			nodesManager.Delete(sid)
		default:
			// Fallback on gnoland binary for other commands
			err = ts.Exec(gnolandBin, args...)
			if err != nil {
				ts.Logf("gno command error: %+v", err)
			}
		}

		tsValidateError(ts, strings.TrimSpace("gnoland "+cmd), neg, err)
	}
}

func gnokeyCmd(nodes *NodesManager) func(ts *testscript.TestScript, neg bool, args []string) {
	return func(ts *testscript.TestScript, neg bool, args []string) {
		gnoHomeDir := ts.Getenv("GNOHOME")

		logger := ts.Value(envKeyLogger).(*slog.Logger)
		sid := ts.Getenv("SID")

		args, err := unquote(args)
		if err != nil {
			tsValidateError(ts, "gnokey", neg, err)
		}

		io := commands.NewTestIO()
		io.SetOut(commands.WriteNopCloser(ts.Stdout()))
		io.SetErr(commands.WriteNopCloser(ts.Stderr()))
		cmd := keyscli.NewRootCmd(io, client.DefaultBaseOptions)

		io.SetIn(strings.NewReader("\n"))
		defaultArgs := []string{
			"-home", gnoHomeDir,
			"-insecure-password-stdin=true",
		}

		if n, ok := nodes.Get(sid); ok {
			if raddr := n.cfg.TMConfig.RPC.ListenAddress; raddr != "" {
				defaultArgs = append(defaultArgs, "-remote", raddr)
			}

			n.nGnoKeyExec++
			headerlog := fmt.Sprintf("%.02d!EXEC_GNOKEY", n.nGnoKeyExec)

			logger.Info(headerlog, "args", strings.Join(args, " "))
			defer logger.Info(headerlog, "delimiter", "END")
		}

		args = append(defaultArgs, args...)

		err = cmd.ParseAndRun(context.Background(), args)
		tsValidateError(ts, "gnokey", neg, err)
	}
}

func adduserCmd(nodesManager *NodesManager) func(ts *testscript.TestScript, neg bool, args []string) {
	return func(ts *testscript.TestScript, neg bool, args []string) {
		gnoHomeDir := ts.Getenv("GNOHOME")

		sid := getNodeSID(ts)
		if nodesManager.IsNodeRunning(sid) {
			tsValidateError(ts, "adduser", neg, errors.New("adduser must be used before starting node"))
			return
		}

		if len(args) == 0 {
			ts.Fatalf("new user name required")
		}

		kb, err := keys.NewKeyBaseFromDir(gnoHomeDir)
		if err != nil {
			ts.Fatalf("unable to get keybase")
		}

		balance, err := createAccount(ts, kb, args[0])
		if err != nil {
			ts.Fatalf("error creating account %s: %s", args[0], err)
		}

		genesis := ts.Value(envKeyGenesis).(*gnoland.GnoGenesisState)
		genesis.Balances = append(genesis.Balances, balance)
	}
}

func adduserfromCmd(nodesManager *NodesManager) func(ts *testscript.TestScript, neg bool, args []string) {
	return func(ts *testscript.TestScript, neg bool, args []string) {
		gnoHomeDir := ts.Getenv("GNOHOME")

		sid := getNodeSID(ts)
		if nodesManager.IsNodeRunning(sid) {
			tsValidateError(ts, "adduserfrom", neg, errors.New("adduserfrom must be used before starting node"))
			return
		}

		var account, index uint64
		var err error

		switch len(args) {
		case 2:
		case 4:
			index, err = strconv.ParseUint(args[3], 10, 32)
			if err != nil {
				ts.Fatalf("invalid index number %s", args[3])
			}
			fallthrough
		case 3:
			account, err = strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				ts.Fatalf("invalid account number %s", args[2])
			}
		default:
			ts.Fatalf("to create account from metadatas, user name and mnemonic are required ( account and index are optional )")
		}

		kb, err := keys.NewKeyBaseFromDir(gnoHomeDir)
		if err != nil {
			ts.Fatalf("unable to get keybase")
		}

		balance, err := createAccountFrom(ts, kb, args[0], args[1], uint32(account), uint32(index))
		if err != nil {
			ts.Fatalf("error creating wallet %s", err)
		}

		genesis := ts.Value(envKeyGenesis).(*gnoland.GnoGenesisState)
		genesis.Balances = append(genesis.Balances, balance)

		fmt.Fprintf(ts.Stdout(), "Added %s(%s) to genesis", args[0], balance.Address)
	}
}

func patchpkgCmd() func(ts *testscript.TestScript, neg bool, args []string) {
	return func(ts *testscript.TestScript, neg bool, args []string) {
		args, err := unquote(args)
		if err != nil {
			tsValidateError(ts, "patchpkg", neg, err)
		}

		if len(args) != 2 {
			ts.Fatalf("`patchpkg`: should have exactly 2 arguments")
		}

		pkgs := ts.Value(envKeyPkgsLoader).(*pkgsLoader)
		replace, with := args[0], args[1]
		pkgs.SetPatch(replace, with)
	}
}

func loadpkgCmd(gnoRootDir string) func(ts *testscript.TestScript, neg bool, args []string) {
	return func(ts *testscript.TestScript, neg bool, args []string) {
		workDir := ts.Getenv("WORK")
		examplesDir := filepath.Join(gnoRootDir, "examples")

		pkgs := ts.Value(envKeyPkgsLoader).(*pkgsLoader)

		var path, name string
		switch len(args) {
		case 2:
			name = args[0]
			path = filepath.Clean(args[1])
		case 1:
			path = filepath.Clean(args[0])
		case 0:
			ts.Fatalf("`loadpkg`: no arguments specified")
		default:
			ts.Fatalf("`loadpkg`: too many arguments specified")
		}

		if path == "all" {
			ts.Logf("warning: loading all packages")
			if err := pkgs.LoadAllPackagesFromDir(examplesDir); err != nil {
				ts.Fatalf("unable to load packages from %q: %s", examplesDir, err)
			}

			return
		}

		if !strings.HasPrefix(path, workDir) {
			path = filepath.Join(examplesDir, path)
		}

		if err := pkgs.LoadPackage(examplesDir, path, name); err != nil {
			ts.Fatalf("`loadpkg` unable to load package(s) from %q: %s", args[0], err)
		}

		ts.Logf("%q package was added to genesis", args[0])
	}
}

// `unquote` takes a slice of strings, resulting from splitting a string block by spaces, and
// processes them. The function handles quoted phrases and escape characters within these strings.
func unquote(args []string) ([]string, error) {
	const quote = '"'

	parts := []string{}
	var inQuote bool

	var part strings.Builder
	for _, arg := range args {
		var escaped bool
		for _, c := range arg {
			if escaped {
				// If the character is meant to be escaped, it is processed with Unquote.
				// We use `Unquote` here for two main reasons:
				// 1. It will validate that the escape sequence is correct
				// 2. It converts the escaped string to its corresponding raw character.
				//    For example, "\\t" becomes '\t'.
				uc, err := strconv.Unquote(`"\` + string(c) + `"`)
				if err != nil {
					return nil, fmt.Errorf("unhandled escape sequence `\\%c`: %w", c, err)
				}

				part.WriteString(uc)
				escaped = false
				continue
			}

			// If we are inside a quoted string and encounter an escape character,
			// flag the next character as `escaped`
			if inQuote && c == '\\' {
				escaped = true
				continue
			}

			// Detect quote and toggle inQuote state
			if c == quote {
				inQuote = !inQuote
				continue
			}

			// Handle regular character
			part.WriteRune(c)
		}

		// If we're inside a quote, add a single space.
		// It reflects one or multiple spaces between args in the original string.
		if inQuote {
			part.WriteRune(' ')
			continue
		}

		// Finalize part, add to parts, and reset for next part
		parts = append(parts, part.String())
		part.Reset()
	}

	// Check if a quote is left open
	if inQuote {
		return nil, errors.New("unfinished quote")
	}

	return parts, nil
}

func getNodeSID(ts *testscript.TestScript) string {
	return ts.Getenv("SID")
}

func getTestingLogger(env *testscript.Env, logname string) (*slog.Logger, error) {
	var path string

	if logdir := os.Getenv("LOG_PATH_DIR"); logdir != "" {
		if err := os.MkdirAll(logdir, 0o755); err != nil {
			return nil, fmt.Errorf("unable to make log directory %q", logdir)
		}

		var err error
		if path, err = filepath.Abs(filepath.Join(logdir, logname)); err != nil {
			return nil, fmt.Errorf("unable to get absolute path of logdir %q", logdir)
		}
	} else if workdir := env.Getenv("WORK"); workdir != "" {
		path = filepath.Join(workdir, logname)
	} else {
		return tm2Log.NewNoopLogger(), nil
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("unable to create log file %q: %w", path, err)
	}

	env.Defer(func() {
		if err := f.Close(); err != nil {
			panic(fmt.Errorf("unable to close log file %q: %w", path, err))
		}
	})

	// Initialize the logger
	logLevel, err := zapcore.ParseLevel(strings.ToLower(os.Getenv("LOG_LEVEL")))
	if err != nil {
		return nil, fmt.Errorf("unable to parse log level, %w", err)
	}

	// Build zap logger for testing
	zapLogger := log.NewZapTestingLogger(f, logLevel)
	env.Defer(func() { zapLogger.Sync() })

	env.T().Log("starting logger", path)
	return log.ZapLoggerToSlog(zapLogger), nil
}

func buildGnoland(t *testing.T, gnoroot, output string) error {
	t.Helper()

	t.Log("building gnoland fork binary...")
	if _, err := os.Stat(output); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			// Handle other potential errors from os.Stat
			return err
		}

		// Build a fresh gno binary in a temp directory
		gnoArgsBuilder := []string{"build", "-o", output}

		// Forward `-covermode` settings if set
		if coverMode := testing.CoverMode(); coverMode != "" {
			gnoArgsBuilder = append(gnoArgsBuilder, "-covermode", coverMode)
		}

		// Append the path to the gno command source
		gnoArgsBuilder = append(gnoArgsBuilder, filepath.Join(gnoroot,
			"gno.land", "pkg", "integration", "forknode"))

		if err = exec.Command("go", gnoArgsBuilder...).Run(); err != nil {
			return fmt.Errorf("unable to build gno binary: %w", err)
		}
	}

	return nil
}

func tsValidateError(ts *testscript.TestScript, cmd string, neg bool, err error) {
	if err != nil {
		fmt.Fprintf(ts.Stderr(), "%q error: %+v\n", cmd, err)
		if !neg {
			ts.Fatalf("unexpected %q command failure: %s", cmd, err)
		}
	} else {
		if neg {
			ts.Fatalf("unexpected %q command success", cmd)
		}
	}
}

type envSetter interface {
	Setenv(key, value string)
}

// createAccount creates a new account with the given name and adds it to the keybase.
func createAccount(env envSetter, kb keys.Keybase, accountName string) (gnoland.Balance, error) {
	var balance gnoland.Balance
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return balance, fmt.Errorf("error creating entropy: %w", err)
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return balance, fmt.Errorf("error generating mnemonic: %w", err)
	}

	var keyInfo keys.Info
	if keyInfo, err = kb.CreateAccount(accountName, mnemonic, "", "", 0, 0); err != nil {
		return balance, fmt.Errorf("unable to create account: %w", err)
	}

	address := keyInfo.GetAddress()
	env.Setenv("USER_SEED_"+accountName, mnemonic)
	env.Setenv("USER_ADDR_"+accountName, address.String())

	return gnoland.Balance{
		Address: address,
		Amount:  std.Coins{std.NewCoin(ugnot.Denom, 10e6)},
	}, nil
}

// createAccountFrom creates a new account with the given metadata and adds it to the keybase.
func createAccountFrom(env envSetter, kb keys.Keybase, accountName, mnemonic string, account, index uint32) (gnoland.Balance, error) {
	var balance gnoland.Balance

	// check if mnemonic is valid
	if !bip39.IsMnemonicValid(mnemonic) {
		return balance, fmt.Errorf("invalid mnemonic")
	}

	keyInfo, err := kb.CreateAccount(accountName, mnemonic, "", "", account, index)
	if err != nil {
		return balance, fmt.Errorf("unable to create account: %w", err)
	}

	address := keyInfo.GetAddress()
	env.Setenv("USER_SEED_"+accountName, mnemonic)
	env.Setenv("USER_ADDR_"+accountName, address.String())

	return gnoland.Balance{
		Address: address,
		Amount:  std.Coins{std.NewCoin(ugnot.Denom, 10e6)},
	}, nil
}
