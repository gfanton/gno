package gnoweb

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	KindUser  PathKind = "u"
	KindRealm PathKind = "r"
	KindPure  PathKind = "p"
)

type GnoURL struct {
	Kind     PathKind
	Path     string
	PathArgs string
	WebQuery url.Values
	Query    url.Values
	FullPath string
}

func (url GnoURL) EncodeArgs() string {
	var urlstr strings.Builder
	if url.PathArgs != "" {
		urlstr.WriteString(url.PathArgs)
	}

	if len(url.Query) > 0 {
		urlstr.WriteString("?" + url.Query.Encode())
	}

	return urlstr.String()
}

func (url GnoURL) EncodePath() string {
	var urlstr strings.Builder
	urlstr.WriteString(url.Path)
	if url.PathArgs != "" {
		urlstr.WriteString(":" + url.PathArgs)
	}

	if len(url.Query) > 0 {
		urlstr.WriteString("?" + url.Query.Encode())
	}

	return urlstr.String()
}

func escapeDollarSign(s string) string {
	return strings.ReplaceAll(s, "$", "%24")
}

func (url GnoURL) EncodeWebPath() string {
	var urlstr strings.Builder
	urlstr.WriteString(url.Path)
	if url.PathArgs != "" {
		pathEscape := escapeDollarSign(url.PathArgs)
		urlstr.WriteString(":" + pathEscape)
	}

	if len(url.WebQuery) > 0 {
		urlstr.WriteString("$" + url.WebQuery.Encode())
	}

	if len(url.Query) > 0 {
		urlstr.WriteString("?" + url.Query.Encode())
	}

	return urlstr.String()
}

var (
	ErrURLMalformedPath   = errors.New("malformed URL path")
	ErrURLInvalidPathKind = errors.New("invalid path kind")
)

// reRealName match a realm path
// - matches[1]: path
// - matches[2]: path kind
// - matches[3]: path args
var reRealmPath = regexp.MustCompile(`(?m)^` +
	`(/([a-z]+)/` + // path kind
	`[a-z][a-z0-9_]*` + // First path segment
	`(?:/[a-z][a-z0-9_]*)*)` + // Additional path segments
	`([:$](?:.*)|$)`, // Remaining portions args, separate by `$` or `:`
)

func ParseGnoURL(u *url.URL) (*GnoURL, error) {
	matches := reRealmPath.FindStringSubmatch(u.EscapedPath())
	if len(matches) != 4 {
		return nil, fmt.Errorf("%w: %s", ErrURLMalformedPath, u.Path)
	}

	path, pathKind, args := matches[1], matches[2], matches[3]

	if len(args) > 0 {
		switch args[0] {
		case ':':
			args = args[1:]
		case '$':
		default:
			return nil, fmt.Errorf("%w: %s", ErrURLMalformedPath, u.Path)
		}
	}

	var err error
	webquery := url.Values{}
	args, webargs, found := strings.Cut(args, "$")
	if found {
		if webquery, err = url.ParseQuery(webargs); err != nil {
			return nil, fmt.Errorf("unable to parse webquery %q: %w ", webquery, err)
		}
	}

	uargs, err := url.PathUnescape(args)
	if err != nil {
		return nil, fmt.Errorf("unable to unescape path %q: %w", args, err)
	}

	host := u.Hostname()
	if host == "" {
		host = "gno.land"
	}

	fullPath := fmt.Sprintf("%s%s", host, path)

	return &GnoURL{
		Path:     path,
		Kind:     PathKind(pathKind),
		PathArgs: uargs,
		WebQuery: webquery,
		Query:    u.Query(),
		FullPath: fullPath,
	}, nil
}