// Package globusauth maintains a Globus service-account client that stays
// usable even when some configured facilities have an invalid collection
// scope (e.g. because the underlying Globus collection was deleted).
//
// Globus Auth's client-credentials token endpoint rejects a token request
// outright if any one of its requested scopes is unknown. Rather than
// proactively validating every facility up front, ClientManager keeps a
// single token that grows to cover whatever facilities have actually been
// used: it starts by requesting every facility's scope, dropping whichever
// ones Globus reports as unknown; later, a request naming a facility not
// yet covered triggers a new token request for the previously-granted
// scopes plus the newly needed ones. A facility is only concluded
// unavailable when it's still missing after that attempt.
package globusauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/SwissOpenEM/globus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const globusAuthTokenURL = "https://auth.globus.org/v2/oauth2/token"

// TokenFetcher requests a Globus access token scoped to the given scopes.
// Defaults to a real call against Globus Auth's token endpoint; tests
// inject a fake to avoid real network calls.
type TokenFetcher func(ctx context.Context, clientID string, clientSecret string, scopes []string) (*oauth2.Token, error)

func fetchGlobusToken(ctx context.Context, clientID string, clientSecret string, scopes []string) (*oauth2.Token, error) {
	conf := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     globusAuthTokenURL,
		Scopes:       scopes,
	}
	return conf.Token(ctx)
}

// ClientManager holds a Globus client scoped to whichever facilities have
// been successfully used so far, without letting one invalid facility
// block access to the rest. It is safe for concurrent use.
type ClientManager struct {
	clientID       string
	clientSecret   string
	facilityScopes map[string][]string // facility name -> its rendered scopes
	scopeFacility  map[string]string   // rendered scope -> facility name
	tokenFetcher   TokenFetcher

	mu            sync.Mutex
	token         *oauth2.Token
	grantedScopes map[string]bool
	badFacilities map[string]error
}

// NewClientManager builds a manager and requests an initial token covering
// every facility. It never fails: a facility whose scope is rejected by
// Globus is logged and marked unavailable rather than blocking startup.
// tokenFetcher defaults to a real Globus Auth call when nil.
func NewClientManager(ctx context.Context, clientID string, clientSecret string, facilityScopes map[string][]string, tokenFetcher TokenFetcher) *ClientManager {
	if tokenFetcher == nil {
		tokenFetcher = fetchGlobusToken
	}

	scopesCopy := make(map[string][]string, len(facilityScopes))
	scopeFacility := make(map[string]string, len(facilityScopes))
	for name, scopes := range facilityScopes {
		scopesCopy[name] = append([]string(nil), scopes...)
		for _, scope := range scopes {
			scopeFacility[scope] = name
		}
	}

	m := &ClientManager{
		clientID:       clientID,
		clientSecret:   clientSecret,
		facilityScopes: scopesCopy,
		scopeFacility:  scopeFacility,
		tokenFetcher:   tokenFetcher,
		grantedScopes:  map[string]bool{},
		badFacilities:  map[string]error{},
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked(ctx, allScopes(scopesCopy))

	return m
}

// Client returns a Globus client whose token covers every named facility,
// requesting a broader token first if the current one doesn't already
// cover them. If a facility can't be covered - because Globus rejects its
// scope - it returns an error naming which one(s).
func (m *ClientManager) Client(ctx context.Context, facilityNames ...string) (globus.GlobusClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	needed, err := m.scopesForLocked(facilityNames)
	if err != nil {
		return globus.GlobusClient{}, err
	}

	if !m.tokenCoversLocked(needed) {
		m.refreshLocked(ctx, union(setKeys(m.grantedScopes), needed))
	}

	if !m.tokenCoversLocked(needed) {
		return globus.GlobusClient{}, fmt.Errorf("facility unavailable: %s", strings.Join(m.unavailableLocked(facilityNames), ", "))
	}

	return m.buildClientLocked(ctx), nil
}

// CurrentClient returns a Globus client covering whatever facilities are
// currently known-good, refreshing an expired token if needed but without
// trying to add coverage for any new facility. It's meant for monitoring
// or cancelling an already-submitted transfer, which doesn't need to name
// its facilities again - whichever facilities were valid at submission
// time are, by construction, already part of the granted scope set, and
// stay that way until something actively invalidates them.
func (m *ClientManager) CurrentClient(ctx context.Context) globus.GlobusClient {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.token == nil || !m.token.Valid() {
		m.refreshLocked(ctx, setKeys(m.grantedScopes))
	}

	return m.buildClientLocked(ctx)
}

func (m *ClientManager) buildClientLocked(ctx context.Context) globus.GlobusClient {
	if m.token == nil {
		return globus.GlobusClient{}
	}
	return globus.HttpClientToGlobusClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(m.token)))
}

func (m *ClientManager) scopesForLocked(names []string) ([]string, error) {
	scopes := make([]string, 0, len(names))
	for _, name := range names {
		facilityScopes, ok := m.facilityScopes[name]
		if !ok {
			return nil, fmt.Errorf("unknown facility %q", name)
		}
		scopes = append(scopes, facilityScopes...)
	}
	return scopes, nil
}

func (m *ClientManager) tokenCoversLocked(scopes []string) bool {
	if m.token == nil || !m.token.Valid() {
		return false
	}
	for _, scope := range scopes {
		if !m.grantedScopes[scope] {
			return false
		}
	}
	return true
}

func (m *ClientManager) unavailableLocked(names []string) []string {
	var unavailable []string
	for _, name := range names {
		for _, scope := range m.facilityScopes[name] {
			if !m.grantedScopes[scope] {
				unavailable = append(unavailable, name)
				break
			}
		}
	}
	return unavailable
}

// refreshLocked requests a token for requestedScopes, dropping whichever
// scopes Globus reports as unknown and retrying, until it either succeeds
// or runs out of scopes to try. On success it replaces m.token and
// m.grantedScopes wholesale with the scopes that actually worked; on
// total failure it leaves them untouched, so a previously-good client
// keeps being served. mu must be held by the caller.
func (m *ClientManager) refreshLocked(ctx context.Context, requestedScopes []string) {
	remaining := append([]string(nil), requestedScopes...)
	sort.Strings(remaining)

	var token *oauth2.Token
	for len(remaining) > 0 {
		var err error
		token, err = m.tokenFetcher(ctx, m.clientID, m.clientSecret, remaining)
		if err == nil {
			break
		}

		badScopes, ok := parseUnknownScopes(err)
		if !ok || len(badScopes) == 0 {
			slog.Error("globus token request failed and the invalid scope(s) couldn't be identified", "error", err, "scopes", remaining)
			return
		}
		remaining = m.dropAndMarkBadLocked(remaining, badScopes)
	}

	if token == nil {
		return // every requested scope was rejected; keep whatever token we had before
	}

	m.token = token
	m.grantedScopes = toSet(remaining)
	for _, scope := range remaining {
		if name, ok := m.scopeFacility[scope]; ok {
			delete(m.badFacilities, name)
		}
	}
}

// dropAndMarkBadLocked removes bad scopes from requested, marking their
// owning facility unavailable, and returns what's left to retry with.
func (m *ClientManager) dropAndMarkBadLocked(requested []string, bad []string) []string {
	badSet := toSet(bad)
	kept := make([]string, 0, len(requested))
	for _, scope := range requested {
		if !badSet[scope] {
			kept = append(kept, scope)
			continue
		}
		name, ok := m.scopeFacility[scope]
		if !ok {
			continue // a scope we don't recognize - can't attribute it to a facility
		}
		err := fmt.Errorf("globus rejected scope for facility %q: %s", name, scope)
		slog.Error("globus facility scope is invalid, facility will be unavailable", "facility", name, "error", err)
		m.badFacilities[name] = err
	}
	return kept
}

func allScopes(facilityScopes map[string][]string) []string {
	scopes := make([]string, 0, len(facilityScopes))
	for _, s := range facilityScopes {
		scopes = append(scopes, s...)
	}
	return scopes
}

func union(a []string, b []string) []string {
	set := toSet(a)
	for _, s := range b {
		set[s] = true
	}
	return setKeys(set)
}

func setKeys(set map[string]bool) []string {
	scopes := make([]string, 0, len(set))
	for s := range set {
		scopes = append(scopes, s)
	}
	return scopes
}

func toSet(scopes []string) map[string]bool {
	set := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		set[s] = true
	}
	return set
}

// unknownScopesBlock finds the bracketed list in Globus Auth's "Unknown
// scope(s)" error detail, e.g.:
//
//	requested unknown scopes: ['https://auth.globus.org/scopes/.../data_access']
var unknownScopesBlock = regexp.MustCompile(`unknown scopes:\s*\[([^\]]*)\]`)
var quotedScope = regexp.MustCompile(`'([^']+)'`)

type globusErrorBody struct {
	Errors []struct {
		Detail string `json:"detail"`
	} `json:"errors"`
}

// parseUnknownScopes extracts the specific scope string(s) Globus reported
// as unknown from a token-request error. ok is false when err isn't an
// oauth2 retrieve error, or its body doesn't contain a recognizable
// "unknown scopes" detail - callers should not guess which scope was at
// fault in that case.
func parseUnknownScopes(err error) (scopes []string, ok bool) {
	var retrieveErr *oauth2.RetrieveError
	if !errors.As(err, &retrieveErr) {
		return nil, false
	}

	var body globusErrorBody
	if jsonErr := json.Unmarshal(retrieveErr.Body, &body); jsonErr != nil {
		return nil, false
	}

	var found []string
	for _, e := range body.Errors {
		block := unknownScopesBlock.FindStringSubmatch(e.Detail)
		if block == nil {
			continue
		}
		for _, m := range quotedScope.FindAllStringSubmatch(block[1], -1) {
			found = append(found, m[1])
		}
	}
	if len(found) == 0 {
		return nil, false
	}
	return found, true
}
