package service

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/yuin/goldmark"
)

type Render interface {
	// Show the content of the give filename for the given realm path
	SourceFile(path, filename string) ([]byte, error)
	// Return list of files of the given realm path
	Sources(path string) ([]string, error)
	// Render the realm
	Render(w io.Writer, path string, arg string) error
}

var _ Render = (*WebRender)(nil)

type WebRender struct {
	log    *slog.Logger
	client *gnoclient.Client
	md     goldmark.Markdown
}

func NewWebRender(log *slog.Logger, cl *gnoclient.Client, md goldmark.Markdown) *WebRender {
	return &WebRender{
		log:    log,
		client: cl,
		md:     md,
	}
}

func (s *WebRender) SourceFile(path, filename string) ([]byte, error) {
	const qpath = "vm/qfile"

	filename = strings.TrimSpace(filename) // sanitize filename
	if filename == "" {
		return nil, errors.New("empty filename given") // XXX -> ErrXXX
	}

	// XXX: move this into gnoclient ?
	path = filepath.Join(path, filename)
	res, err := s.query(qpath, []byte(path))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *WebRender) Sources(path string) ([]string, error) {
	const qpath = "vm/qfile"

	// XXX: move this into gnoclient
	res, err := s.query(qpath, []byte(path))
	if err != nil {
		return nil, err
	}

	files := strings.Split(string(res), "\n")
	return files, nil
}

func (s *WebRender) Render(w io.Writer, pkgPath string, args string) error {
	const path = "vm/qrender"

	pkgPath = strings.Trim(pkgPath, "/")
	data := []byte(fmt.Sprintf("gno.land/%s:%s", pkgPath, args))
	rawres, err := s.query(path, data)
	if err != nil {
		return err
	}

	if err := s.md.Convert(rawres, w); err != nil {
		return fmt.Errorf("unable render realm: %q", err)
	}

	return nil
}

func (s *WebRender) query(qpath string, data []byte) ([]byte, error) {
	s.log.Info("query", "qpath", qpath, "data", string(data))
	// XXX: move this into gnoclient
	qres, err := s.client.Query(gnoclient.QueryCfg{
		Path: qpath,
		Data: data,
	})

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
