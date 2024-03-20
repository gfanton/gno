package proxyapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/amino"
	abci "github.com/gnolang/gno/tm2/pkg/bft/abci/types"
	"github.com/gnolang/gno/tm2/pkg/std"
)

type Proxy struct {
	*gnoclient.Client

	cfg gnoclient.BaseTxCfg

	account     std.BaseAccount
	accountOnce *sync.Once

	logger *slog.Logger
	debug  bool
}

func NewProxy(client *gnoclient.Client, logger *slog.Logger, debug bool) *Proxy {
	return &Proxy{
		accountOnce: &sync.Once{},
		Client:      client,
		logger:      logger,
		debug:       debug,
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	p.logger.Info("receiving request", "url", req.URL.String())
	var err error
	switch req.Method {
	case "GET":
		err = p.handleGet(w, req)
	case "POST":
		err = p.handlePost(w, req)
	}

	if err != nil {
		p.logger.Error("request", "url", req.URL.String(), "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (p *Proxy) handleGet(w http.ResponseWriter, req *http.Request) error {
	realm := strings.TrimLeft(req.URL.Path, "/")
	realm = filepath.Clean(realm)

	basename := filepath.Base(realm)

	splitMethod := strings.SplitN(basename, ".", 2)
	if len(splitMethod) < 2 {
		// Check for command
		splitCmd := strings.SplitN(basename, ":", 2)
		if len(splitCmd) < 2 {
			return p.render(w, realm)
		}

		command := splitCmd[1]
		realm = strings.TrimSuffix(realm, ":"+splitCmd[1])
		switch command {
		case "funcs":
			req := gnoclient.QueryCfg{}
			req.Path = "vm/qfuncs"
			req.Data = []byte(realm)
			return p.query(w, req)
		default:
			return fmt.Errorf("unknown command: %q", command)
		}
	}

	realm = strings.TrimSuffix(realm, "."+splitMethod[1])

	var cfg gnoclient.BaseTxCfg
	var call gnoclient.MsgCall
	call.PkgPath = realm
	call.FuncName = splitMethod[1]

	// we need to query funcs to correctly correlate arguments
	funcs, err := p.queryFuncs(realm)
	if err != nil {
		return fmt.Errorf("unable to query funcs on %q: %w", realm, err)
	}

	params, ok := funcs[call.FuncName]
	if !ok {
		return fmt.Errorf("unknown function call %q", call.FuncName)
	}

	values := make(map[string]string)
	query := req.URL.Query()
	for key, vals := range query {
		switch len(vals) {
		case 0:
			continue
		case 1:
			values[key] = vals[0]
		default:
			var s strings.Builder
			s.WriteRune('[')
			for i, val := range vals {
				if i > 0 {
					s.WriteRune(',')
				}

				s.WriteString(strconv.Quote(val))
			}
			s.WriteRune(']')
			values[key] = s.String()
		}
	}

	call.Args = make([]string, 0, len(params))
	for _, param := range params {
		if param.Name == "" || param.Name == "_" {
			continue
		}

		v, ok := values[param.Name]
		if !ok {
			return fmt.Errorf("missing field %q in query", param.Name)
		}

		call.Args = append(call.Args, string(v))
	}

	account, err := p.getSignerAccount()
	if err != nil {
		return fmt.Errorf("unable to get signer account: %w", err)
	}

	cfg.AccountNumber = account.AccountNumber
	cfg.SequenceNumber = account.Sequence
	cfg.GasFee = "1000000ugnot"
	cfg.GasWanted = 2000000

	return p.call(w, cfg, call)
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

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("error reading request body: %w", err)
	}

	body = bytes.TrimSpace(body)
	if len(body) < 2 {
		return fmt.Errorf("invalid request body, should be an array or an object")
	}

	// we need to query funcs to correctly correlate arguments
	funcs, err := p.queryFuncs(realm)
	if err != nil {
		return fmt.Errorf("unable to query funcs on %q: %w", realm, err)
	}

	params, ok := funcs[call.FuncName]
	if !ok {
		return fmt.Errorf("unknown function call %q", call.FuncName)
	}

	call.Args = make([]string, 0, len(params))
	switch {
	case body[0] == '{' && body[len(body)-1] == '}':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(body, &obj); err != nil {
			return fmt.Errorf("invalid request body object: %w", err)
		}

		for _, param := range params {
			if param.Name == "" || param.Name == "_" {
				continue
			}

			v, ok := obj[param.Name]
			if !ok {
				return fmt.Errorf("missing field %q in request", param.Name)
			}

			if unquoted, err := strconv.Unquote(string(v)); err == nil {
				call.Args = append(call.Args, unquoted)
				continue
			}

			call.Args = append(call.Args, string(v))
		}

	case body[0] == '[' && body[len(body)-1] == ']':
		var obj []json.RawMessage
		if err := json.Unmarshal(body, &obj); err != nil {
			return fmt.Errorf("invalid request body object: %w", err)
		}

		if len(obj) != len(params) {
			return fmt.Errorf("invalid number of arguments, have %d need %d ", len(obj), len(params))
		}

		for _, o := range obj {
			if unquoted, err := strconv.Unquote(string(o)); err == nil {
				call.Args = append(call.Args, unquoted)
				continue
			}

			call.Args = append(call.Args, string(o))
		}

	default:
		return fmt.Errorf("invalid request body, should be an array or an object")
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

func (p *Proxy) query(w http.ResponseWriter, req gnoclient.QueryCfg) error {
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

func (p *Proxy) render(w http.ResponseWriter, realm string) error {
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

func (p *Proxy) call(w http.ResponseWriter, cfg gnoclient.BaseTxCfg, call gnoclient.MsgCall) error {
	p.logger.Info("call",
		"realm", call.PkgPath,
		"method", call.FuncName,
		"gazWanted", cfg.GasWanted,
		"gazFee", cfg.GasFee,
		"args", call.Args)

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
		Data    json.RawMessage `json:"Data"`
		GasUsed int64           `json:"GasUsed"`
	}{
		Data:    json.RawMessage(res.DeliverTx.Data),
		GasUsed: res.CheckTx.GasUsed,
	}

	if err := p.writeJSON(w, res2); err != nil {
		panic(err)
	}

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

func (p *Proxy) writeErrorJSON(w http.ResponseWriter, res abci.ResponseBase) error {
	p.logger.Error(fmt.Sprintf("%s\n%s", res.Error, res.Log))
	mres, err := amino.MarshalJSONIndent(res, "", "\t")
	if err != nil {
		return res.Error
	}

	w.Header().Set("Content-Type", "application/json")
	http.Error(w, string(mres), http.StatusInternalServerError)
	return nil
}

func (p *Proxy) writeJSON(w http.ResponseWriter, data any) error {
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

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("write json error: %w", err)
	}

	return nil
}

func (p *Proxy) logTm2Error(err error, msg string) {
	p.logger.Error(fmt.Sprintf("%s: \n%+v", msg, err))
}
