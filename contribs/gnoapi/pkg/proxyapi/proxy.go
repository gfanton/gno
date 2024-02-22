package proxyapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/amino"
	abci "github.com/gnolang/gno/tm2/pkg/bft/abci/types"
	"github.com/gnolang/gno/tm2/pkg/errors"
	"github.com/gnolang/gno/tm2/pkg/std"
)

type Proxy struct {
	*gnoclient.Client

	cfg gnoclient.BaseTxCfg

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
		err = p.handlePost(res, req)
	}

	if err != nil {
		p.logger.Error("request", "url", req.URL.String(), "error", err)
		fmt.Fprintf(res, "error: %s", err)
	}
}

func (p *Proxy) handleGet(res http.ResponseWriter, req *http.Request) error {
	realm := strings.TrimLeft(req.URL.Path, "/")
	realm = filepath.Clean(realm)

	basename := filepath.Base(realm)

	splitMethod := strings.SplitN(basename, ".", 2)
	if len(splitMethod) < 2 {
		// Check for command
		splitCmd := strings.SplitN(basename, ":", 2)
		if len(splitCmd) < 2 {
			return p.render(res, realm)
		}

		command := splitCmd[1]
		realm = strings.TrimSuffix(realm, ":"+splitCmd[1])
		switch command {
		case "funcs":
			req := gnoclient.QueryCfg{}
			req.Path = "vm/qfuncs"
			req.Data = []byte(realm)
			return p.query(res, req)
		default:
			return fmt.Errorf("unknown command: %q", command)
		}
	}

	realm = strings.TrimSuffix(realm, "."+splitMethod[1])

	var cfg gnoclient.BaseTxCfg
	var call gnoclient.MsgCall
	call.PkgPath = realm
	call.FuncName = splitMethod[1]

	values := map[string]any{}
	query := req.URL.Query()
	for key, val := range query {
		switch len(val) {
		case 0:
			continue
		case 1:
			values[key] = val[0]
		default:
			values[key] = val
		}
	}

	var err error
	call.JSONRequest, err = json.Marshal(values)
	if err != nil {
		return fmt.Errorf("marshal query to json error: %w", err)
	}

	account, err := p.getSignerAccount()
	if err != nil {
		return fmt.Errorf("unable to get signer account: %w", err)
	}

	cfg.AccountNumber = account.AccountNumber
	cfg.SequenceNumber = account.Sequence
	cfg.GasFee = "1000000ugnot"
	cfg.GasWanted = 2000000

	return p.call(res, cfg, call)
}

func (p *Proxy) handlePost(res http.ResponseWriter, req *http.Request) error {
	realm := strings.TrimLeft(req.URL.Path, "/")
	realm = filepath.Clean(realm)

	basename := filepath.Base(realm)

	splitMethod := strings.SplitN(basename, ".", 2)
	if len(splitMethod) < 2 {
		// Check for command
		splitCmd := strings.SplitN(basename, ":", 2)
		if len(splitCmd) < 2 {
			return p.render(res, realm)
		}

		return fmt.Errorf("invalid command")
	}

	realm = strings.TrimSuffix(realm, "."+splitMethod[1])

	var cfg gnoclient.BaseTxCfg
	var call gnoclient.MsgCall
	call.PkgPath = realm
	call.FuncName = splitMethod[1]

	// Read request body
	var err error
	call.JSONRequest, err = io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("Error reading request body: %w", err)
	}

	account, err := p.getSignerAccount()
	if err != nil {
		return fmt.Errorf("unable to get signer account: %w", err)
	}

	cfg.AccountNumber = account.AccountNumber
	cfg.SequenceNumber = account.Sequence
	cfg.GasFee = "1000000ugnot"
	cfg.GasWanted = 2000000

	return p.call(res, cfg, call)
}

func (p *Proxy) query(w io.Writer, req gnoclient.QueryCfg) error {
	res, err := p.Query(req)
	if err != nil {
		return fmt.Errorf("query error: %w", err)
	}

	if res.Response.IsErr() {
		return p.writeErrorJSON(w, res.Response.ResponseBase)
	}

	if _, err = fmt.Fprintln(w, string(res.Response.Data)); err != nil {
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

type funcsMap map[string] /* func name */ []vm.NamedType /* args name */

func (p *Proxy) queryFuncs(realm string) (funcsMap, error) {
	var req gnoclient.QueryCfg

	req.Path = "vm/qfuncs"
	req.Data = []byte(realm)
	res, err := p.Query(req)
	if err != nil {
		return nil, fmt.Errorf("unable to query funcs: %w", err)
	}

	var fsigs vm.FunctionSignatures
	if err := amino.UnmarshalJSON(res.Response.Data, &fsigs); err != nil {
		return nil, fmt.Errorf("unable to unmarshal msgs: %w", err)
	}

	mfuncs := make(funcsMap)
	for _, fn := range fsigs {
		mfuncs[fn.FuncName] = fn.Params
	}

	return mfuncs, nil
}

func (p *Proxy) call(w io.Writer, cfg gnoclient.BaseTxCfg, call gnoclient.MsgCall) error {
	p.logger.Info("call",
		"realm", call.PkgPath,
		"method", call.FuncName,
		"gazWanted", cfg.GasWanted,
		"gazFee", cfg.GasFee,
		"request", string(call.JSONRequest))

	res, err := p.Call(cfg, call)
	if err != nil {
		p.logTm2Error(err, "unable to make call")
		return fmt.Errorf("unable to make call on %q: %w", call.PkgPath, err)
	}

	if res.DeliverTx.IsErr() {
		return p.writeErrorJSON(w, res.DeliverTx.ResponseBase)
	}

	if res.CheckTx.IsErr() {
		return p.writeErrorJSON(w, res.CheckTx.ResponseBase)
	}

	res2 := struct {
		Data    json.RawMessage `json:"data"`
		GasUsed int64           `json:"gasUsed"`
	}{
		Data:    json.RawMessage(res.DeliverTx.Data),
		GasUsed: res.CheckTx.GasUsed,
	}

	ret, err := json.MarshalIndent(res2, "", "\t")
	if err != nil {
		panic(err)
	}

	fmt.Fprintln(w, string(ret))
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
