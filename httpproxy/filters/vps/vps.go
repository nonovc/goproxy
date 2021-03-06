package vps

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/golang/glog"
	"github.com/phuslu/goproxy/httpproxy"
	"github.com/phuslu/goproxy/httpproxy/filters"
	"github.com/phuslu/http2"
)

const (
	filterName string = "vps"
)

type Filter struct {
	FetchServers []*FetchServer
	Transport    *http2.Transport
	Sites        *httpproxy.HostMatcher
}

func init() {
	filename := filterName + ".json"
	config, err := NewConfig(filters.LookupConfigStoreURI(filterName), filename)
	if err != nil {
		glog.Fatalf("NewConfig(%#v) failed: %s", filename, err)
	}

	err = filters.Register(filterName, &filters.RegisteredFilter{
		New: func() (filters.Filter, error) {
			return NewFilter(config)
		},
	})

	if err != nil {
		glog.Fatalf("Register(%#v) error: %s", filterName, err)
	}
}

func NewFilter(config *Config) (filters.Filter, error) {
	fetchServers := make([]*FetchServer, 0)
	for _, fs := range config.FetchServers {
		u, err := url.Parse(fs.URL)
		if err != nil {
			return nil, err
		}

		fs := &FetchServer{
			URL:       u,
			Password:  fs.Password,
			SSLVerify: fs.SSLVerify,
		}

		fetchServers = append(fetchServers, fs)
	}

	transport := &http2.Transport{
		InsecureTLSDial: true,
	}

	return &Filter{
		FetchServers: fetchServers,
		Transport:    transport,
		Sites:        httpproxy.NewHostMatcher(config.Sites),
	}, nil
}

func (p *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Response, error) {
	if !f.Sites.Match(req.Host) {
		return ctx, nil, nil
	}

	i := 0
	switch path.Ext(req.URL.Path) {
	case ".jpg", ".png", ".webp", ".bmp", ".gif", ".flv", ".mp4":
		i = rand.Intn(len(f.FetchServers))
	case "":
		name := path.Base(req.URL.Path)
		if strings.Contains(name, "play") ||
			strings.Contains(name, "video") {
			i = rand.Intn(len(f.FetchServers))
		}
	}

	fetchServer := f.FetchServers[i]

	req1, err := fetchServer.encodeRequest(req)
	if err != nil {
		return ctx, nil, fmt.Errorf("VPS encodeRequest: %s", err.Error())
	}
	res, err := f.Transport.RoundTrip(req1)
	if err != nil {
		return ctx, nil, err
	} else {
		glog.Infof("%s \"VPS %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, res.StatusCode, res.Header.Get("Content-Length"))
	}
	resp, err := fetchServer.decodeResponse(res)
	return ctx, resp, err
}
