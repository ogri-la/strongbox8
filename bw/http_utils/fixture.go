package http_utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
)

// a canned response for a URL.
type Fixture struct {
	Status int // zero is 200
	Body   []byte
	Header http.Header
}

// a `http.RoundTripper` that answers requests from fixtures keyed by exact URL, for tests.
// it never touches the network: a request for a URL without a fixture fails and is
// recorded in `Unrouted`, so a test can fail on any request it did not expect.
// fixtures can be added or replaced while requests are being made, to simulate a host
// changing, such as a new release being published.
type FixtureTransport struct {
	mu        sync.Mutex
	routes    map[string]Fixture // URL => response
	requested []string
	unrouted  []string
}

var _ http.RoundTripper = (*FixtureTransport)(nil)

// returns a `FixtureTransport` answering from `routes`, URL => response.
func NewFixtureTransport(routes map[string]Fixture) *FixtureTransport {
	ft := &FixtureTransport{routes: map[string]Fixture{}}
	for url, fixture := range routes {
		ft.routes[url] = fixture
	}
	return ft
}

// adds or replaces the fixture for `url`.
func (ft *FixtureTransport) Set(url string, fixture Fixture) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.routes[url] = fixture
}

// returns every URL requested so far, in order.
func (ft *FixtureTransport) Requested() []string {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return slices.Clone(ft.requested)
}

// returns every URL requested so far that had no fixture, in order.
func (ft *FixtureTransport) Unrouted() []string {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return slices.Clone(ft.unrouted)
}

func (ft *FixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()

	ft.mu.Lock()
	ft.requested = append(ft.requested, url)
	fixture, present := ft.routes[url]
	if !present {
		ft.unrouted = append(ft.unrouted, url)
	}
	ft.mu.Unlock()

	if !present {
		return nil, fmt.Errorf("no fixture for url: %s", url)
	}

	status := fixture.Status
	if status == 0 {
		status = http.StatusOK
	}
	header := fixture.Header
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(fixture.Body)),
		ContentLength: int64(len(fixture.Body)),
		Request:       req,
	}, nil
}
