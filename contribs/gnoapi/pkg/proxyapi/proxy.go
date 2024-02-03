package proxyapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slog"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	abci "github.com/gnolang/gno/tm2/pkg/bft/abci/types"
)

type Proxy struct {
	*gnoclient.Client
	logger *slog.Logger
	debug  bool
	json   bool
}

func NewProxy(client *gnoclient.Client, logger *slog.Logger, debug, json bool) *Proxy {
	return &Proxy{
		Client: client,
		logger: logger,
		debug:  debug,
		json:   json,
	}
}

func (c *Proxy) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	c.logger.Info("receiving request", "url", req.URL.String())
	var err error
	switch req.Method {
	case "GET":
		err = c.handleGet(res, req)
	case "POST":
	}

	if err != nil {
		fmt.Fprintf(res, "error: %s", err)
	}
}

func (c *Proxy) handleGet(res http.ResponseWriter, req *http.Request) error {
	realm := strings.TrimLeft(req.URL.Path, "/")

	pkg := filepath.Base(realm)
	spkg := strings.SplitN(pkg, ".", 2)
	if len(spkg) < 2 {
		return c.render(res, realm)
	}

	var cfg gnoclient.CallCfg
	cfg.Args = []string{}
	cfg.PkgPath, cfg.FuncName = spkg[0], spkg[1]

	for key, values := range req.URL.Query() {
		switch len(values) {
		case 1:
			cfg.Args = append(cfg.Args, fmt.Sprintf("%q=%q", key, values[0]))
		case 0:
			cfg.Args = append(cfg.Args, fmt.Sprintf("%q", key))
		default:
			value := fmt.Sprintf("[%s]", strings.Join(values, ","))
			cfg.Args = append(cfg.Args, fmt.Sprintf("%q=%q", key, value))
		}
	}

	// cfg.gasFee = "1000000ugnot"
	// cfg.gasWanted = 2000000
	// cfg.send = ""
	// cfg.broadcast = true
	// cfg.chainID = "tendermint_test"
	// cfg.remote = remoteAddr
	// cfg.pkgPath = makecfg.rlmpath
	// cfg.kb = makecfg.kb

	cfg.AccountNumber = 0
	cfg.GasFee = "1000000ugnot"
	cfg.GasWanted = 2000000

	fmt.Printf("args: %v", cfg.Args)
	return c.call(res, realm, cfg)
}

func (c *Proxy) query(w io.Writer, req gnoclient.QueryCfg) error {
	res, err := c.Query(req)
	if err != nil {
		return fmt.Errorf("query error: %w", err)
	}

	if res.Response.IsErr() {
		return c.writeErrorJSON(w, res.Response.ResponseBase)
	}

	if _, err := w.Write(res.Response.Data); err != nil {
		return fmt.Errorf("unable to write data: %w", err)
	}

	return nil
}

func (c *Proxy) render(w io.Writer, realm string) error {
	var req gnoclient.QueryCfg

	req.Data = []byte(fmt.Sprintf("%s\n%s", realm, ""))
	req.Path = "vm/qrender"
	err := c.query(w, req)
	if err != nil {
		return fmt.Errorf("unable to render %q: %w", realm, err)
	}

	return nil
}

func (c *Proxy) call(w io.Writer, realm string, cfg gnoclient.CallCfg) error {
	res, err := c.Call(cfg)
	if err != nil {
		return fmt.Errorf("uanble to query %q render", realm)
	}

	if res.DeliverTx.IsErr() {
		return c.writeErrorJSON(w, res.DeliverTx.ResponseBase)
	}

	if res.CheckTx.IsErr() {
		return c.writeErrorJSON(w, res.CheckTx.ResponseBase)
	}

	fmt.Fprintf(w, "gaz used: %d", res.CheckTx.GasUsed)
	return nil
}

type ErrorResponse struct {
	Error string `json:"error"`
	Log   string `json:"log,omitempty"`
}

func (c *Proxy) writeErrorJSON(w io.Writer, res abci.ResponseBase) error {
	var errRes ErrorResponse

	errRes.Error = res.Error.Error()
	if c.debug {
		errRes.Log = res.Log
	}

	return c.writeJSON(w, &errRes)
}

func (c *Proxy) writeJSON(w io.Writer, data any) error {
	var err error
	var raw []byte
	if c.debug {
		raw, err = json.MarshalIndent(data, "", "\t")
	} else {
		raw, err = json.Marshal(data)
	}

	if err != nil {
		return fmt.Errorf("unable to marshal error response: %w", err)
	}

	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}
