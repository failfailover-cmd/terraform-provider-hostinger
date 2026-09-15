package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

func writeWebsitePage(w http.ResponseWriter, page, size, total int, sites []Website) {
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"data": sites,
		"meta": map[string]int{"current_page": page, "per_page": size, "total": total},
	})
}

func inventoryTestClient(server *httptest.Server) *Client {
	c := NewClient("test-token")
	c.BaseURL = server.URL
	c.MinRequestInterval = 0
	c.MaxRetries = 0
	return c
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInventoryLoadPanicReleasesReadersAndAllowsRetry(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeWebsitePage(w, 1, 100, 1, []Website{{Domain: "site.example"}})
	}))
	defer server.Close()
	c := inventoryTestClient(server)
	var panicked atomic.Bool
	c.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !panicked.Swap(true) {
			panic("simulated transport bug")
		}
		return http.DefaultTransport.RoundTrip(r)
	})
	if _, err := c.GetWebsite("site.example"); err == nil || errors.Is(err, ErrWebsiteNotFound) {
		t.Fatalf("panic returned %v, want a read error", err)
	}
	if _, err := c.GetWebsite("site.example"); err != nil {
		t.Fatalf("retry after panic: %v", err)
	}
}

func TestConcurrentWebsiteReadsSharePaginatedInventory(t *testing.T) {
	t.Parallel()
	const count = 151
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 || page > 2 {
			t.Errorf("unexpected page %d", page)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sites := []Website{}
		for i := (page - 1) * 100; i < min(page*100, count); i++ {
			sites = append(sites, Website{Domain: fmt.Sprintf("site-%d.example", i), OrderID: i + 1})
		}
		writeWebsitePage(w, page, 100, count, sites)
	}))
	defer server.Close()
	c := inventoryTestClient(server)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			domain := fmt.Sprintf("site-%d.example", i)
			site, err := c.GetWebsite(domain)
			if err != nil || site.Domain != domain || site.OrderID != i+1 {
				t.Errorf("GetWebsite(%q) = %+v, %v", domain, site, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	for i := 0; i < 3; i++ {
		if _, err := c.GetWebsite("missing.example"); !errors.Is(err, ErrWebsiteNotFound) {
			t.Fatalf("missing domain: %v", err)
		}
	}
	site, _ := c.GetWebsite("site-0.example")
	site.OrderID = -1
	again, _ := c.GetWebsite("site-0.example")
	if again.OrderID != 1 {
		t.Fatal("caller modified the cached website")
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("151 parallel reads plus repeated misses made %d requests, want 2", got)
	}
}

func TestInventoryErrorsAreNotCachedOrReportedAsMissing(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"missing metadata": `{"data":[{"domain":"existing.example"}]}`,
		"missing data":     `{"meta":{"current_page":1,"per_page":100,"total":0}}`,
		"null data":        `{"data":null,"meta":{"current_page":1,"per_page":100,"total":0}}`,
		"zero page size":   `{"data":[],"meta":{"current_page":1,"per_page":0,"total":0}}`,
		"wrong page":       `{"data":[],"meta":{"current_page":2,"per_page":100,"total":0}}`,
		"missing total":    `{"data":[],"meta":{"current_page":1,"per_page":100}}`,
		"negative total":   `{"data":[],"meta":{"current_page":1,"per_page":100,"total":-1}}`,
		"truncated data":   `{"data":[],"meta":{"current_page":1,"per_page":100,"total":1}}`,
		"empty domain":     `{"data":[{}],"meta":{"current_page":1,"per_page":100,"total":1}}`,
		"duplicate domain": `{"data":[{"domain":"a"},{"domain":"a"}],"meta":{"current_page":1,"per_page":100,"total":2}}`,
		"invalid JSON":     `{`,
		"HTTP not found":   `{"message":"endpoint not found"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					if name == "HTTP not found" {
						w.WriteHeader(http.StatusNotFound)
					}
					_, _ = w.Write([]byte(body))
					return
				}
				writeWebsitePage(w, 1, 100, 1, []Website{{Domain: "existing.example"}})
			}))
			defer server.Close()
			c := inventoryTestClient(server)
			if _, err := c.GetWebsite("existing.example"); err == nil || errors.Is(err, ErrWebsiteNotFound) {
				t.Fatalf("incomplete inventory returned %v, want a read error", err)
			}
			if _, err := c.GetWebsite("existing.example"); err != nil {
				t.Fatalf("retry after invalid response: %v", err)
			}
			if requests.Load() != 2 {
				t.Fatalf("requests = %d, want 2", requests.Load())
			}
		})
	}
}

func TestMutationInvalidatesWebsiteInventory(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		for _, status := range []int{http.StatusNoContent, http.StatusConflict, http.StatusBadRequest} {
			t.Run(fmt.Sprintf("%s/%d", method, status), func(t *testing.T) {
				t.Parallel()
				var reads atomic.Int32
				var changed atomic.Bool
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodGet {
						changed.Store(true)
						w.WriteHeader(status)
						if status == http.StatusConflict {
							_, _ = w.Write([]byte(`{"message":"website already exists"}`))
						}
						return
					}
					reads.Add(1)
					sites := []Website{}
					if changed.Load() == (method == http.MethodPost) {
						sites = append(sites, Website{Domain: "site.example"})
					}
					writeWebsitePage(w, 1, 100, len(sites), sites)
				}))
				defer server.Close()
				c := inventoryTestClient(server)
				_, _ = c.GetWebsite("site.example")
				if method == http.MethodPost {
					_ = c.CreateWebsite("site.example", 1, "")
				} else {
					_ = c.DeleteWebsite("site.example")
				}
				_, err := c.GetWebsite("site.example")
				if method == http.MethodPost && err != nil || method == http.MethodDelete && !errors.Is(err, ErrWebsiteNotFound) {
					t.Fatalf("read after mutation: %v", err)
				}
				if reads.Load() != 2 {
					t.Fatalf("GET requests = %d, want 2", reads.Load())
				}
			})
		}
	}
}

func TestFailedLaterPageDoesNotPublishPartialInventory(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"HTTP error", "changed total", "repeated page", "empty page"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			var reads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := reads.Add(1)
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				if page == 1 {
					writeWebsitePage(w, 1, 1, 2, []Website{{Domain: "first.example"}})
					return
				}
				if n == 2 {
					switch failure {
					case "HTTP error":
						w.WriteHeader(http.StatusServiceUnavailable)
					case "changed total":
						writeWebsitePage(w, 2, 1, 3, []Website{{Domain: "second.example"}})
					case "repeated page":
						writeWebsitePage(w, 1, 1, 2, []Website{{Domain: "first.example"}})
					case "empty page":
						writeWebsitePage(w, 2, 1, 2, []Website{})
					}
					return
				}
				writeWebsitePage(w, 2, 1, 2, []Website{{Domain: "second.example"}})
			}))
			defer server.Close()
			c := inventoryTestClient(server)
			if _, err := c.GetWebsite("first.example"); err == nil || errors.Is(err, ErrWebsiteNotFound) {
				t.Fatalf("partial inventory accepted: %v", err)
			}
			if _, err := c.GetWebsite("second.example"); err != nil {
				t.Fatal(err)
			}
			if reads.Load() != 4 {
				t.Fatalf("requests = %d, want two full load attempts (4 pages)", reads.Load())
			}
		})
	}
}

func TestMutationDuringInventoryLoadDiscardsStaleResult(t *testing.T) {
	t.Parallel()
	var reads atomic.Int32
	loading, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if reads.Add(1) == 1 {
			close(loading)
			<-release
			writeWebsitePage(w, 1, 100, 0, []Website{})
			return
		}
		writeWebsitePage(w, 1, 100, 1, []Website{{Domain: "new.example"}})
	}))
	defer server.Close()
	c := inventoryTestClient(server)
	result := make(chan error, 1)
	go func() { _, err := c.GetWebsite("new.example"); result <- err }()
	<-loading
	err := c.CreateWebsite("new.example", 1, "")
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatalf("read used pre-mutation inventory: %v", err)
	}
	if reads.Load() != 2 {
		t.Fatalf("GET requests = %d, want 2", reads.Load())
	}
}

func TestInventoryIsClientScopedAndListRemainsFresh(t *testing.T) {
	t.Parallel()
	var reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order := int(reads.Add(1))
		writeWebsitePage(w, 1, 100, 1, []Website{{Domain: "site.example", OrderID: order}})
	}))
	defer server.Close()
	first, second := inventoryTestClient(server), inventoryTestClient(server)
	_, _ = first.GetWebsite("site.example")
	site, err := second.GetWebsite("site.example")
	if err != nil || site.OrderID != 2 {
		t.Fatalf("second client's inventory: %+v, %v", site, err)
	}
	sites, err := first.ListWebsites()
	if err != nil || len(sites) != 1 || sites[0].OrderID != 3 {
		t.Fatalf("fresh list: %+v, %v", sites, err)
	}
}
