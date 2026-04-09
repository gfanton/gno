package chainrpc

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// ---- ABCI Response Types

// RealmEvalResult holds the result of a vm/qeval query.
type RealmEvalResult struct {
	Result  string `json:"result"`
	PkgPath string `json:"pkg_path"`
	Height  int64  `json:"height"`
}

// RealmInspectResult holds package overview from vm/qfuncs + vm/qdoc + vm/qfile.
type RealmInspectResult struct {
	PkgPath      string       `json:"pkg_path"`
	Functions    string       `json:"functions,omitempty"`
	FunctionsErr string       `json:"functions_error,omitempty"`
	Doc          string       `json:"doc,omitempty"`
	DocErr       string       `json:"doc_error,omitempty"`
	Files        []SourceFile `json:"files,omitempty"`
	FilesErr     string       `json:"files_error,omitempty"`
	Height       int64        `json:"height"`
}

// SourceFile holds a file name in a package listing.
type SourceFile struct {
	Name string `json:"name"`
}

// RealmSourceResult holds the content of a specific source file.
type RealmSourceResult struct {
	PkgPath string `json:"pkg_path"`
	File    string `json:"file"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	Height  int64  `json:"height"`
}

// AccountInfoResult holds account details from auth/accounts query.
type AccountInfoResult struct {
	Address       string `json:"address"`
	Coins         string `json:"coins,omitempty"`
	Sequence      int64  `json:"sequence,omitempty"`
	AccountNumber int64  `json:"account_number,omitempty"`
	PubKeyType    string `json:"pub_key_type,omitempty"`
	Height        int64  `json:"height"`
	Exists        bool   `json:"exists"`
}

// ---- Data Format Helpers

func formatEvalData(pkgPath, expression string) []byte {
	return []byte(pkgPath + "." + expression)
}

func formatFileData(pkgPath, fileName string) []byte {
	if fileName == "" {
		return []byte(pkgPath)
	}
	return []byte(pkgPath + "/" + fileName)
}

func formatAccountPath(address string) string {
	return "auth/accounts/" + address
}

// ---- ABCI Response Parsing

// abciQueryData makes an ABCI query and returns the decoded response data.
// It handles error checking and base64 decoding of the response.
func (c *Client) abciQueryData(ctx context.Context, path string, data []byte) ([]byte, int64, error) {
	result, err := c.ABCIQuery(ctx, path, data, 0, false)
	if err != nil {
		return nil, 0, err
	}

	// Check for ABCI error in response
	abciErr := result.Get("response.ResponseBase.Error")
	if abciErr.Exists() && abciErr.String() != "" {
		return nil, 0, fmt.Errorf("abci: %s", abciErr.String())
	}

	height := result.Get("response.Height").Int()

	// Decode base64 response data
	b64 := result.Get("response.ResponseBase.Data").String()
	if b64 == "" {
		return nil, height, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, height, fmt.Errorf("decode response data: %w", err)
	}

	return decoded, height, nil
}

// ---- Public Query Methods

// EvalExpression evaluates a Gno expression on a realm via vm/qeval.
func (c *Client) EvalExpression(ctx context.Context, pkgPath, expression string) (*RealmEvalResult, error) {
	data := formatEvalData(pkgPath, expression)
	decoded, height, err := c.abciQueryData(ctx, "vm/qeval", data)
	if err != nil {
		return nil, fmt.Errorf("eval %s.%s: %w", pkgPath, expression, err)
	}

	return &RealmEvalResult{
		Result:  string(decoded),
		PkgPath: pkgPath,
		Height:  height,
	}, nil
}

// InspectPackage returns package overview: functions, doc, and file list.
// Partial failures are captured in error fields -- the call succeeds if at
// least one sub-query returns data.
func (c *Client) InspectPackage(ctx context.Context, pkgPath string) (*RealmInspectResult, error) {
	result := &RealmInspectResult{PkgPath: pkgPath}
	pkgData := []byte(pkgPath)

	// Functions (vm/qfuncs)
	funcs, height, err := c.abciQueryData(ctx, "vm/qfuncs", pkgData)
	if err != nil {
		result.FunctionsErr = err.Error()
	} else {
		result.Functions = string(funcs)
		result.Height = height
	}

	// Documentation (vm/qdoc)
	doc, height, err := c.abciQueryData(ctx, "vm/qdoc", pkgData)
	if err != nil {
		result.DocErr = err.Error()
	} else {
		result.Doc = string(doc)
		if result.Height == 0 {
			result.Height = height
		}
	}

	// File listing (vm/qfile with no filename)
	files, height, err := c.abciQueryData(ctx, "vm/qfile", pkgData)
	if err != nil {
		result.FilesErr = err.Error()
	} else {
		if result.Height == 0 {
			result.Height = height
		}
		for _, name := range strings.Split(string(files), "\n") {
			name = strings.TrimSpace(name)
			if name != "" {
				result.Files = append(result.Files, SourceFile{Name: name})
			}
		}
	}

	// If all three failed, return an error
	if result.FunctionsErr != "" && result.DocErr != "" && result.FilesErr != "" {
		return nil, fmt.Errorf("inspect %s: all queries failed: funcs=%s, doc=%s, files=%s",
			pkgPath, result.FunctionsErr, result.DocErr, result.FilesErr)
	}

	return result, nil
}

// FetchSource fetches a specific source file from a package via vm/qfile.
func (c *Client) FetchSource(ctx context.Context, pkgPath, fileName string) (*RealmSourceResult, error) {
	data := formatFileData(pkgPath, fileName)
	decoded, height, err := c.abciQueryData(ctx, "vm/qfile", data)
	if err != nil {
		return nil, fmt.Errorf("source %s/%s: %w", pkgPath, fileName, err)
	}

	return &RealmSourceResult{
		PkgPath: pkgPath,
		File:    fileName,
		Content: string(decoded),
		Size:    len(decoded),
		Height:  height,
	}, nil
}

// QueryAccount fetches account info via auth/accounts/<address>.
// The response is amino JSON parsed with gjson.
func (c *Client) QueryAccount(ctx context.Context, address string) (*AccountInfoResult, error) {
	path := formatAccountPath(address)
	decoded, height, err := c.abciQueryData(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("account %s: %w", address, err)
	}

	result := &AccountInfoResult{
		Address: address,
		Height:  height,
	}

	if len(decoded) == 0 {
		return result, nil
	}

	result.Exists = true

	// Parse amino JSON response
	parsed := gjson.ParseBytes(decoded)
	result.Coins = parsed.Get("BaseAccount.coins").String()
	result.Sequence = parsed.Get("BaseAccount.sequence").Int()
	result.AccountNumber = parsed.Get("BaseAccount.account_number").Int()
	result.PubKeyType = parsed.Get("BaseAccount.public_key.@type").String()

	return result, nil
}
