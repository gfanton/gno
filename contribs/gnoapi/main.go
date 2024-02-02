package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	log "github.com/gnolang/gno/gno.land/pkg/log"
	"github.com/gnolang/gno/tm2/pkg/commands"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type apiCfg struct {
	remote   string
	listener string
	realm    string
}

var defaultApiOptions = &apiCfg{
	listener: "127.0.0.1:8585",
	remote:   "127.0.0.1:36657",
	realm:    "",
}

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
	fs.StringVar(
		&c.listener,
		"listen",
		defaultApiOptions.listener,
		"listener adress",
	)

	fs.StringVar(
		&c.listener,
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

}

func execApi(cfg *apiCfg, args []string, io commands.IO) error {
	if cfg.realm == "" {
		panic("empty realm given")
	}

	logger := log.ZapLoggerToSlog(log.NewZapConsoleLogger(io.Out(), zapcore.DebugLevel))
	funcs, err := makeFuncs(logger, cfg.realm)
	if err != nil {
		panic(fmt.Errorf("unable to get funcs list: %w", err))
	}

	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dynamic.proto"),
		Package: proto.String("dynamic"),
		// Define syntax, options, etc., as needed.
	}

	service := descriptorpb.ServiceDescriptorProto{}
	for _, fun := range funcs {

		fd.Service = append(fd.Service)
		msg := dynamicpb.NewMessage(desc)
		fmt.Printf("func: %s\n", msg)
	}

	fmt.Printf("funcs list: %+v\n", funcs)

	return nil
}
