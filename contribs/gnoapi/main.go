package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gnolang/gno/contribs/gnoapi/pkg/proxyapi"
	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/integration"
	log "github.com/gnolang/gno/gno.land/pkg/log"
	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/commands"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/crypto/keys"
	"go.uber.org/zap/zapcore"
)

type apiCfg struct {
	remote   string
	address  string
	listener string
	realm    string
	chainID  string
	gnoHome  string
}

var defaultApiOptions = &apiCfg{
	listener: ":8282",
	remote:   "127.0.0.1:36657",
	realm:    "",
	chainID:  "tendermint_test",
	address:  "",
}

var (
	IntegrationAccountAddress = crypto.MustAddressFromString(integration.DefaultAccount_Address)
)

func main() {
	cfg := &apiCfg{}

	stdio := commands.NewDefaultIO()
	cmd := commands.NewCommand(
		commands.Metadata{
			Name:       "gnoapi",
			ShortUsage: "gnoapi [flags] [path ...]",
			ShortHelp:  "proxy node rpc api",
			LongHelp:   `gnoapi`,
		},
		cfg,
		func(_ context.Context, args []string) error {
			return execApi(cfg, args, stdio)
		})

	cmd.Execute(context.Background(), os.Args[1:])
}

func (c *apiCfg) RegisterFlags(fs *flag.FlagSet) {
	defaultGnoHome := gnoenv.HomeDir()

	fs.StringVar(
		&c.listener,
		"listen",
		defaultApiOptions.listener,
		"listener adress",
	)

	fs.StringVar(
		&c.remote,
		"remote",
		defaultApiOptions.remote,
		"remote node adress",
	)

	fs.StringVar(
		&c.realm,
		"realm",
		defaultApiOptions.realm,
		"target realm",
	)

	fs.StringVar(
		&c.chainID,
		"chain-id",
		defaultApiOptions.chainID,
		"chain id",
	)

	fs.StringVar(
		&c.address,
		"name",
		defaultApiOptions.address,
		"name or bech32 to load from the keybase",
	)

	fs.StringVar(
		&c.gnoHome,
		"home",
		defaultGnoHome,
		"gno home path",
	)
}

func execApi(cfg *apiCfg, args []string, io commands.IO) error {
	logger := log.ZapLoggerToSlog(log.NewZapConsoleLogger(io.Out(), zapcore.DebugLevel))

	home := gnoenv.HomeDir()

	var kb keys.Keybase
	if cfg.address != "" {
		var err error
		kb, err = keys.NewKeyBaseFromDir(home)
		if err != nil {
			return fmt.Errorf("unable to load keybase: %w", err)
		}
	} else {
		// create a inmemory keybase
		kb = keys.NewInMemory()
		kb.CreateAccount(integration.DefaultAccount_Name, integration.DefaultAccount_Seed, "", "", 0, 0)
		cfg.address = integration.DefaultAccount_Name
	}

	logger.Info("loading account", "name", cfg.address)
	signer, err := getSignerForAccount(io, kb, cfg)
	if err != nil {
		return fmt.Errorf("unable to get signer for account %q: %w", cfg.address, err)
	}

	client := &gnoclient.Client{
		Signer:    signer,
		RPCClient: client.NewHTTP(cfg.remote, "/websocket"),
	}
	// funcs, err := makeFuncs(logger, cfg.realm)

	proxycl := proxyapi.NewProxy(client, logger, true, true)

	var server http.Server
	server.ReadHeaderTimeout = 60 * time.Second
	server.Handler = proxycl

	// fd := &descriptorpb.FileDescriptorProto{
	// 	Name:    proto.String("dynamic.proto"),
	// 	Package: proto.String("dynamic"),
	// 	// Define syntax, options, etc., as needed.
	// }

	// service := descriptorpb.ServiceDescriptorProto{}
	// for _, fun := range funcs {

	// 	fd.Service = append(fd.Service)
	// 	msg := dynamicpb.NewMessage(desc)
	// 	fmt.Printf("func: %s\n", msg)
	// }

	// fmt.Printf("funcs list: %+v\n", funcs)

	l, err := net.Listen("tcp", cfg.listener)
	if err != nil {
		return fmt.Errorf("unable to listen on %q: %w", cfg.listener, err)
	}
	logger.Info("api listening", "addr", l.Addr())

	return server.Serve(l)
}

func getSignerForAccount(io commands.IO, kb keys.Keybase, cfg *apiCfg) (gnoclient.Signer, error) {
	var signer gnoclient.SignerFromKeybase

	signer.Keybase = kb
	signer.Account = cfg.address
	signer.ChainID = cfg.chainID // XXX: override this
	// 	ChainID:  chainid, // Chain ID for transaction signing

	if ok, err := kb.HasByNameOrAddress(cfg.address); !ok || err != nil {
		if err != nil {
			return nil, fmt.Errorf("invalid name: %w", err)
		}

		return nil, fmt.Errorf("unknown name/address: %q", cfg.address)
	}

	// try empty password first
	if _, err := kb.ExportPrivKeyUnsafe(cfg.address, ""); err != nil {
		prompt := fmt.Sprintf("[%.10s] Enter password:", cfg.address)
		signer.Password, err = io.GetPassword(prompt, true)
		if err != nil {
			return nil, fmt.Errorf("error while reading password: %w", err)
		}

		if _, err := kb.ExportPrivKeyUnsafe(cfg.address, string(signer.Password)); err != nil {
			return nil, fmt.Errorf("invalid password: %w", err)
		}
	}
	return signer, nil
}

// Create a dynamic message based on field descriptions
// func createDynamicMessage(fs vm.FunctionSignature) *dynamicpb.Message {
// 	// Create a slice to hold field descriptors
// 	var fieldDescriptors []*descriptorpb.FieldDescriptorProto

// 	// Iterate over the fields to create field descriptors
// 	for i, field := range fs.Params {
// 		// Check if the field is an array
// 		label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
// 		if len(field.Type) > 2 && field.Type[:2] == "[]" {
// 			label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
// 			field.Type = field.Type[2:] // Remove the array prefix to handle the base type
// 		}

// 		var fieldType descriptorpb.FieldDescriptorProto_Type
// 		switch field.Type {
// 		case "bool":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_BOOL
// 		case "string":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_STRING
// 		case "int":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_INT32
// 		case "int8":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_BYTES
// 		case "int16":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_INT32
// 		case "int32":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_INT32
// 		case "int64":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_INT64
// 		case "uint":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_UINT32
// 		case "uint8":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_INT32
// 		case "uint16":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_UINT32
// 		case "uint32":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_UINT32
// 		case "uint64":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_UINT64
// 		case "float32":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_FLOAT
// 		case "float64":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE
// 		case "bigint":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_UINT64
// 		case "bigdec":
// 			fieldType = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE
// 			// case "<untyped> bigdec":
// 			// 	return typeid("<untyped> bigdec")
// 			// case "<untyped> bigint":
// 			// 	return typeid("<untyped> bigint")
// 			// case "<untyped> bool":
// 			// 	return typeid("<untyped> bool")
// 			// case UntypedRuneType:
// 			// 	return typeid("<untyped> rune")
// 			// case "<untyped> string":
// 			// 	return typeid("<untyped> string")
// 			// }
// 		}

// 		fieldDescriptor := &descriptorpb.FieldDescriptorProto{
// 			Name:   &field.Name,
// 			Number: proto.Int32(int32(i + 1)), // Field numbers should start at 1
// 			Type:   &fieldType,
// 			Label:  &label,
// 		}

// 		fieldDescriptors = append(fieldDescriptors, fieldDescriptor)
// 	}

// 	// // Create a dynamic message descriptor from the field descriptors
// 	// messageDescriptorProto := &descriptorpb.DescriptorProto{
// 	// 	Name:  proto.String("DynamicMessage"), // You might want to give each message a unique name
// 	// 	Field: fieldDescriptors,
// 	// }

// 	// messageDescriptor := dynamicpb.NewMessageType(messageDescriptorProto)

// 	// // Create a new dynamic message instance
// 	// message := dynamicpb.NewMessage(messageDescriptor)

// 	return nil
// }
