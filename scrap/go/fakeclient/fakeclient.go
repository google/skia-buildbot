// Package fakeclient offers a fake implementation of scrap.ScrapExchange. The scraps
// can be loaded into memory, but are not persisted anywhere. Such an implementation
// does not require internet access or authentication.
// For simplicity, it ignores the scrap.Type, as it assumes a given client will only
// care about one type of scrap anyway.
//
// It is only meant to be used for local testing.
package fakeclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sort"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/scrap/go/scrap"
)

type FakeClient struct {
	scraps map[string]scrap.ScrapBody
}

// New returns a fake (in-memory) client with the map of scraps preloaded.
func New(scraps map[string]scrap.ScrapBody) *FakeClient {
	return &FakeClient{scraps: scraps}
}

func (f *FakeClient) LoadScrap(_ context.Context, _ scrap.Type, hashOrName string) (scrap.ScrapBody, error) {
	body, ok := f.scraps[hashOrName]
	if !ok {
		return scrap.ScrapBody{}, skerr.Fmt("no scrap in fake client with name %s", hashOrName)
	}
	return body, nil
}

func (f *FakeClient) CreateScrap(_ context.Context, sb scrap.ScrapBody) (scrap.ScrapID, error) {
	h := sha256.Sum256([]byte(string(sb.Type) + sb.Body))
	newID := hex.EncodeToString(h[:])
	f.scraps[newID] = sb
	return scrap.ScrapID{Hash: scrap.SHA256(newID)}, nil
}

func (f *FakeClient) DeleteScrap(_ context.Context, _ scrap.Type, hashOrName string) error {
	if _, ok := f.scraps[hashOrName]; !ok {
		return skerr.Fmt("no scrap in fake client with name %s", hashOrName)
	}
	delete(f.scraps, hashOrName)
	return nil
}

func (f *FakeClient) ListNames(_ context.Context, _ scrap.Type) ([]string, error) {
	var names []string
	for n := range f.scraps {
		names = append(names, n)
	}
	// sort for determinism
	sort.Strings(names)
	return names, nil
}

func (f *FakeClient) Expand(context.Context, scrap.Type, string, scrap.Lang, io.Writer) error {
	panic("Not implemented for FakeClient")
}

func (f *FakeClient) PutName(context.Context, scrap.Type, string, scrap.Name) error {
	panic("Not implemented for FakeClient")
}

func (f *FakeClient) GetName(context.Context, scrap.Type, string) (scrap.Name, error) {
	panic("Not implemented for FakeClient")
}

func (f *FakeClient) DeleteName(context.Context, scrap.Type, string) error {
	panic("Not implemented for FakeClient")
}

// Make sure our FakeClient implements all of scrap.ScrapExchange
var _ scrap.ScrapExchange = (*FakeClient)(nil)
