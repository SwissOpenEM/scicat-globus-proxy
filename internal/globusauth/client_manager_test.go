package globusauth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// fakeFetcher is a test double for TokenFetcher. It fails a request if any
// of its requested scopes is in a mutable bad set, returning an error
// shaped exactly like a real Globus Auth "Unknown scope(s)" response so
// the real parseUnknownScopes code path gets exercised.
type fakeFetcher struct {
	mu     sync.Mutex
	bad    map[string]bool
	calls  [][]string
	nextID int
}

func newFakeFetcher(bad ...string) *fakeFetcher {
	badSet := make(map[string]bool, len(bad))
	for _, b := range bad {
		badSet[b] = true
	}
	return &fakeFetcher{bad: badSet}
}

func (f *fakeFetcher) fetch(_ context.Context, _ string, _ string, scopes []string) (*oauth2.Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, append([]string(nil), scopes...))

	var badHere []string
	for _, s := range scopes {
		if f.bad[s] {
			badHere = append(badHere, s)
		}
	}
	if len(badHere) > 0 {
		return nil, globusUnknownScopeError(badHere...)
	}

	f.nextID++
	return &oauth2.Token{AccessToken: fmt.Sprintf("token-%d", f.nextID), Expiry: time.Now().Add(time.Hour)}, nil
}

func (f *fakeFetcher) setBad(bad ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	badSet := make(map[string]bool, len(bad))
	for _, b := range bad {
		badSet[b] = true
	}
	f.bad = badSet
}

func (f *fakeFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// globusUnknownScopeError builds an *oauth2.RetrieveError with a body
// matching Globus Auth's real "Unknown scope(s)" response shape.
func globusUnknownScopeError(scopes ...string) error {
	quoted := make([]string, len(scopes))
	for i, s := range scopes {
		quoted[i] = "'" + s + "'"
	}
	detail := fmt.Sprintf("client_id=test requested unknown scopes: [%s]", strings.Join(quoted, " "))
	body, _ := json.Marshal(map[string]any{
		"errors": []map[string]string{
			{"title": "Unknown scope(s)", "detail": detail, "code": "UNKNOWN_SCOPE_ERROR"},
		},
		"error":             "unknown_scope_error",
		"error_description": "Unknown scope(s)",
	})
	return &oauth2.RetrieveError{Body: body}
}

func twoFacilityScopes() map[string][]string {
	return map[string][]string{
		"A": {"scopeA"},
		"B": {"scopeB"},
	}
}

func TestParseUnknownScopes(t *testing.T) {
	err := globusUnknownScopeError("scopeA", "scopeB")
	scopes, ok := parseUnknownScopes(err)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"scopeA", "scopeB"}, scopes)
}

func TestParseUnknownScopes_UnrelatedError(t *testing.T) {
	_, ok := parseUnknownScopes(fmt.Errorf("network error"))
	assert.False(t, ok)
}

func TestParseUnknownScopes_UnparseableRetrieveError(t *testing.T) {
	_, ok := parseUnknownScopes(&oauth2.RetrieveError{Body: []byte("not json")})
	assert.False(t, ok)
}

func TestNewClientManager_AllScopesValid(t *testing.T) {
	fetcher := newFakeFetcher()
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), fetcher.fetch)

	require.Equal(t, 1, fetcher.callCount())
	assert.ElementsMatch(t, []string{"scopeA", "scopeB"}, fetcher.calls[0])

	client, err := manager.Client(context.Background(), "A", "B")
	require.NoError(t, err)
	assert.True(t, client.IsClientSet())
	// Both facilities were already covered by the startup token - no extra fetch.
	assert.Equal(t, 1, fetcher.callCount())
}

func TestNewClientManager_OneFacilityInvalidAtStartup(t *testing.T) {
	fetcher := newFakeFetcher("scopeB")
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), fetcher.fetch)

	require.Equal(t, 2, fetcher.callCount()) // combined fails, retry with just scopeA succeeds
	assert.ElementsMatch(t, []string{"scopeA"}, fetcher.calls[1])

	client, err := manager.Client(context.Background(), "A")
	require.NoError(t, err)
	assert.True(t, client.IsClientSet())

	_, err = manager.Client(context.Background(), "B")
	assert.Error(t, err)
}

func TestNewClientManager_AllFacilitiesInvalidAtStartup(t *testing.T) {
	fetcher := newFakeFetcher("scopeA", "scopeB")
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), fetcher.fetch)

	client, err := manager.Client(context.Background(), "A")
	assert.Error(t, err)
	assert.False(t, client.IsClientSet())

	_, err = manager.Client(context.Background(), "B")
	assert.Error(t, err)
}

func TestClientManager_ExpandsToCoverNewlyNeededFacility(t *testing.T) {
	facilities := map[string][]string{
		"A": {"scopeA"},
		"B": {"scopeB"},
		"C": {"scopeC"},
	}
	// Only A is requested at startup by making B and C's scopes fail
	// initially, then "fixing" C before it's actually needed.
	fetcher := newFakeFetcher("scopeB", "scopeC")
	manager := NewClientManager(context.Background(), "id", "secret", facilities, fetcher.fetch)

	_, err := manager.Client(context.Background(), "A")
	require.NoError(t, err)

	fetcher.setBad("scopeB") // C is fixed, B still bad
	client, err := manager.Client(context.Background(), "A", "C")
	require.NoError(t, err)
	assert.True(t, client.IsClientSet())

	_, err = manager.Client(context.Background(), "B")
	assert.Error(t, err)
}

func TestClientManager_PreviouslyGoodFacilityStaysUsableUntilTokenExpires(t *testing.T) {
	fetcher := newFakeFetcher()
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), fetcher.fetch)

	fetcher.setBad("scopeB") // B goes bad, but the current token hasn't expired

	client, err := manager.Client(context.Background(), "A", "B")
	require.NoError(t, err)
	assert.True(t, client.IsClientSet())
	assert.Equal(t, 1, fetcher.callCount()) // no re-validation triggered
}

func TestClientManager_FacilityGoesBadOnTokenExpiry(t *testing.T) {
	fetcher := newFakeFetcher()
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), fetcher.fetch)

	fetcher.setBad("scopeB")
	manager.token.Expiry = time.Now().Add(-time.Second) // force the next use to refresh

	client, err := manager.Client(context.Background(), "A")
	require.NoError(t, err)
	assert.True(t, client.IsClientSet())

	_, err = manager.Client(context.Background(), "B")
	assert.Error(t, err)
}

func TestClientManager_FacilityRecoversAfterBeingBad(t *testing.T) {
	fetcher := newFakeFetcher("scopeB")
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), fetcher.fetch)

	_, err := manager.Client(context.Background(), "B")
	require.Error(t, err)

	fetcher.setBad()
	client, err := manager.Client(context.Background(), "A", "B")
	require.NoError(t, err)
	assert.True(t, client.IsClientSet())
}

func TestClientManager_UnknownFacilityName_ReturnsError(t *testing.T) {
	fetcher := newFakeFetcher()
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), fetcher.fetch)

	_, err := manager.Client(context.Background(), "doesNotExist")
	assert.Error(t, err)
}

func TestClientManager_UnparseableFailureDoesNotLoopForever(t *testing.T) {
	calls := 0
	always := func(context.Context, string, string, []string) (*oauth2.Token, error) {
		calls++
		return nil, fmt.Errorf("some other kind of failure")
	}
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), always)

	client, err := manager.Client(context.Background(), "A")
	assert.Error(t, err)
	assert.False(t, client.IsClientSet())
	// One call at startup, one more triggered by Client() finding A
	// missing - neither loops, since the failure can't be attributed to a
	// specific scope.
	assert.Equal(t, 2, calls)
}

func TestClientManager_EmptyFacilityScopesMap(t *testing.T) {
	fetcher := newFakeFetcher()
	manager := NewClientManager(context.Background(), "id", "secret", map[string][]string{}, fetcher.fetch)

	assert.Equal(t, 0, fetcher.callCount())
	client := manager.CurrentClient(context.Background())
	assert.False(t, client.IsClientSet())
}

func TestClientManager_CurrentClient_UsesWhateverIsCurrentlyGood(t *testing.T) {
	fetcher := newFakeFetcher("scopeB")
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), fetcher.fetch)

	client := manager.CurrentClient(context.Background())
	assert.True(t, client.IsClientSet())
}

func TestClientManager_ConcurrentAccess(t *testing.T) {
	fetcher := newFakeFetcher()
	manager := NewClientManager(context.Background(), "id", "secret", twoFacilityScopes(), fetcher.fetch)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = manager.Client(context.Background(), "A", "B")
		}()
		go func() {
			defer wg.Done()
			manager.CurrentClient(context.Background())
		}()
	}
	wg.Wait()
}
