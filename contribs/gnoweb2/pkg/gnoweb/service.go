package gnoweb

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/tm2/pkg/log"
	"github.com/yuin/goldmark"
)

type ServiceRender interface {
	// Show the content of the give filename for the given realm path
	SourceFile(path, file string) ([]byte, error)
	// Return list of files of the given realm path
	Sources(path string) ([]string, error)
	// Render the realm
	Realm(path string, arg string) ([]byte, error)
}

type Config struct {
	// GoldMark renderer
	Markdown goldmark.Markdown
	Logger   *slog.Logger
}

func NewDefaultConfig() Config {
	return Config{
		Markdown: goldmark.New(),
		Logger:   log.NewNoopLogger(),
	}
}

type WebServiceRender struct {
	log    *slog.Logger
	client *gnoclient.Client
	md     goldmark.Markdown
}

func NewService(cfg Config) *WebServiceRender {
	return &WebServiceRender{
		log: cfg.Logger,
		md:  cfg.Markdown,
	}
}

func (s *WebServiceRender) File(path, filename string) ([]string, error) {
	const qpath = "vm/qfile"

	if filename == "" {
		return nil, errors.New("empty filename given") // XXX -> ErrXXX
	}

	// XXX: move this into gnoclient
	path = filepath.Join(path, filename)
	res, err := s.query(qpath, []byte(path))
	if err != nil {
		return nil, err
	}

	files := strings.Split(string(res), "\n")
	return files, nil
}

func (s *WebServiceRender) Sources(path string) ([]string, error) {
	const qpath = "vm/qfile"

	// XXX: move this into gnoclient
	res, err := s.query(qpath, []byte(path))
	if err != nil {
		return nil, err
	}

	files := strings.Split(string(res), "\n")
	return files, nil
}

func (s *WebServiceRender) RenderRealm(qpath string, args string) ([]byte, error) {
	const path = "vm/qrender"

	data := []byte(qpath + ":" + args)
	rawres, err := s.query(path, data)
	if err != nil {
		return nil, err
	}

	// XXX: use sync pool to save some allocs
	var buff bytes.Buffer
	if err := s.md.Convert(rawres, &buff); err != nil {
		return nil, fmt.Errorf("unable render realm: %q", err)
	}

	return buff.Bytes(), nil
}

func (s *WebServiceRender) query(qpath string, data []byte) ([]byte, error) {
	// XXX: move this into gnoclient
	qres, err := s.client.Query(gnoclient.QueryCfg{
		Path: qpath,
		Data: data,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to query")
	}

	if err != nil {
		s.log.Error("request error", "path", qpath, "error", err)
		return nil, fmt.Errorf("unable to query path %q: %w", qpath, err)
	}
	if qres.Response.Error != nil {
		s.log.Error("response error", "path", qpath, "log", qres.Response.Log)
		return nil, qres.Response.Error
	}

	return qres.Response.Data, nil
}
