package got

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// Got holds got download config.
type Got struct {
	ProgressFunc

	Client *http.Client

	ctx context.Context
}

// UserAgent is the default Got user agent to send http requests.
var UserAgent = "CloudApp/8.9.1 (com.bitqiu.pan; build:99; iOS 14.7.0) Alamofire/4.7.0"
var ContentType = "application/x-www-form-urlencoded"

// ErrDownloadAborted - When download is aborted by the OS before it is completed, ErrDownloadAborted will be triggered
var ErrDownloadAborted = errors.New("Operation aborted")

// DefaultClient is the default http client for got requests.
var DefaultClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		Proxy:               http.ProxyFromEnvironment,
	},
}
var downloadWg *sync.WaitGroup

// Download creates *Download item and runs it.
func (g Got) Download(URL, dest string, wg *sync.WaitGroup) error {
	downloadWg = wg
	defer downloadWg.Done()
	return g.Do(&Download{
		ctx:    g.ctx,
		URL:    URL,
		Dest:   dest,
		Client: g.Client,
	})
}

// Do inits and runs ProgressFunc if set and starts the Download.
func (g Got) Do(dl *Download) error {

	if err := dl.Init(); err != nil {
		return err
	}

	if g.ProgressFunc != nil {

		defer func() {
			dl.StopProgress = true
		}()

		go dl.RunProgress(g.ProgressFunc)
	}

	return dl.Start()
}

// New returns new *Got with default context and client.
func New() *Got {
	return NewWithContext(context.Background())
}

// NewWithContext wants Context and returns *Got with default http client.
func NewWithContext(ctx context.Context) *Got {
	return &Got{
		ctx:    ctx,
		Client: DefaultClient,
	}
}

// NewRequest returns a new http.Request and error if any.
func NewRequest(ctx context.Context, method, URL string, header []GotHeader) (req *http.Request, err error) {
	if req, err = http.NewRequestWithContext(ctx, method, URL, nil); err != nil {
		return
	}
	req.Header.Add("User-Agent", UserAgent)
	req.Header.Add("Content-Type", ContentType)
	for _, h := range header {
		req.Header.Set(h.Key, h.Value)
	}
	return
}
