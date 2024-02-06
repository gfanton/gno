package proxyapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/exp/slog"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/tm2/pkg/amino"
	abci "github.com/gnolang/gno/tm2/pkg/bft/abci/types"
	"github.com/gnolang/gno/tm2/pkg/errors"
	"github.com/gnolang/gno/tm2/pkg/std"
)

type Proxy struct {
	*gnoclient.Client

	account     std.BaseAccount
	accountOnce *sync.Once

	logger *slog.Logger
	debug  bool
	json   bool
}

func NewProxy(client *gnoclient.Client, logger *slog.Logger, debug, json bool) *Proxy {
	return &Proxy{
		accountOnce: &sync.Once{},
		Client:      client,
		logger:      logger,
		debug:       debug,
		json:        json,
	}
}

func (p *Proxy) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	p.logger.Info("receiving request", "url", req.URL.String())
	var err error
	switch req.Method {
	case "GET":
		err = p.handleGet(res, req)
	case "POST":
	}

	if err != nil {
		p.logger.Error("request", "url", req.URL.String(), "error", err)
		fmt.Fprintf(res, "error: %s", err)
	}
}

func (p *Proxy) handleGet(res http.ResponseWriter, req *http.Request) error {
	realm := strings.TrimLeft(req.URL.Path, "/")
	realm = filepath.Clean(realm)

	pkg := filepath.Base(realm)
	fmt.Println("pkg", pkg)
	spkg := strings.SplitN(pkg, ".", 2)
	if len(spkg) < 2 {
		return p.render(res, realm)
	}

	account, err := p.getSignerAccount()
	if err != nil {
		return fmt.Errorf("unable to get signer account: %w", err)
	}

	var cfg gnoclient.CallCfg
	cfg.Args = []string{}
	cfg.PkgPath = strings.TrimSuffix(realm, "."+spkg[1])
	cfg.FuncName = spkg[1]

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

	cfg.AccountNumber = account.AccountNumber
	cfg.SequenceNumber = account.Sequence
	cfg.GasFee = "1000000ugnot"
	cfg.GasWanted = 2000000

	fmt.Printf("args: %v", cfg.Args)
	return p.call(res, cfg)
}

func (p *Proxy) query(w io.Writer, req gnoclient.QueryCfg) error {
	res, err := p.Query(req)
	if err != nil {
		return fmt.Errorf("query error: %w", err)
	}

	if res.Response.IsErr() {
		return p.writeErrorJSON(w, res.Response.ResponseBase)
	}

	if _, err := w.Write(res.Response.Data); err != nil {
		return fmt.Errorf("unable to write data: %w", err)
	}

	return nil
}

func (p *Proxy) render(w io.Writer, realm string) error {
	var req gnoclient.QueryCfg

	req.Data = []byte(fmt.Sprintf("%s\n%s", realm, ""))
	req.Path = "vm/qrender"
	err := p.query(w, req)
	if err != nil {
		return fmt.Errorf("unable to render %q: %w", realm, err)
	}

	return nil
}

func (p *Proxy) call(w io.Writer, cfg gnoclient.CallCfg) error {
	p.logger.Info("call",
		"realm", cfg.PkgPath,
		"method", cfg.FuncName,
		"gazWanted", cfg.GasWanted,
		"gazFee", cfg.GasFee,
		"args", cfg.Args)

	res, err := p.Call(cfg)
	if err != nil {
		p.logTm2Error(err, "unable to make call")
		return fmt.Errorf("unable to make call on %q: %w", cfg.PkgPath, err)
	}

	if res.DeliverTx.IsErr() {
		return p.writeErrorJSON(w, res.DeliverTx.ResponseBase)
	}

	if res.CheckTx.IsErr() {
		return p.writeErrorJSON(w, res.CheckTx.ResponseBase)
	}

	fmt.Fprintf(w, "gaz used: %d", res.CheckTx.GasUsed)
	return nil
}

func (p *Proxy) getSignerAccount() (std.BaseAccount, error) {
	var err error
	p.accountOnce.Do(func() {
		info := p.Signer.Info()
		address := info.GetAddress()
		cfg := gnoclient.QueryCfg{
			Path: fmt.Sprintf("auth/accounts/%s", address),
		}

		qres, err := p.Query(cfg)
		if err != nil {
			p.accountOnce = &sync.Once{} // reset once
			err = fmt.Errorf("query account : %w", err)
			return
		}

		var qret struct{ BaseAccount std.BaseAccount }
		err = amino.UnmarshalJSON(qres.Response.Data, &qret)
		if err != nil {
			p.accountOnce = &sync.Once{} // reset once
			err = fmt.Errorf("unmarshall query response: %w", err)
			return
		}

		p.account = qret.BaseAccount
		p.logger.Info("retrieving signer account", "account", qret.BaseAccount)
	})

	return p.account, err
}

type ErrorResponse struct {
	Error string `json:"error"`
	Log   string `json:"log,omitempty"`
}

func (p *Proxy) writeErrorJSON(w io.Writer, res abci.ResponseBase) error {
	var errRes ErrorResponse
	p.logger.Error("response error", "error", res.Error, "log", res.Log)
	errRes.Error = res.Error.Error()
	if p.debug {
		errRes.Log = res.Log
	}

	return p.writeJSON(w, &errRes)
}

func (p *Proxy) writeJSON(w io.Writer, data any) error {
	var err error
	var raw []byte
	if p.debug {
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

func (p *Proxy) logTm2Error(err error, msg string, args ...any) {
	if werr, ok := err.(errors.Error); ok {
		p.logger.Error(msg, "error", err, "cause", errors.Cause(werr))
	} else {
		p.logger.Error(msg, "error", err)
	}

}
