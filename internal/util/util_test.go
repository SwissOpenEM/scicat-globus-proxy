package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type user struct {
	Name    string
	Profile struct {
		AccessGroups []string
		OidcClaims   map[string]any
	}
}

func TestCheckProperty(t *testing.T) {

	testuser := user{
		Name: "Bob",
		Profile: struct {
			AccessGroups []string
			OidcClaims   map[string]any
		}{
			AccessGroups: []string{"group1", "group2"},
			OidcClaims: map[string]any{
				"azp": "scicat",
				// client role example
				"resource_access": map[string]any{
					"scicat": map[string]any{
						"roles": []string{"scicat-user"},
					},
				},
				// realm role example
				"realm_access": map[string]any{
					"roles": []string{"facility-user"},
				},
				// Some IdPs map roles this way
				"roles": []string{"admin", "user"},
			},
		},
	}

	cases := []struct {
		name     string
		data     any
		path     string
		value    any
		expected bool // value is found
		throwErr bool // path is valid
	}{
		{"valid name", testuser, "Name", "Bob", true, false},
		{"invalid field", testuser, "Invalid", "Bob", false, true},
		{"valid nested slice", testuser, "profile.accessGroups", "group2", true, false},
		{"missing element", testuser, "profile.accessGroups", "group3", false, false},
		{"arbitrary oidc claim", testuser, "profile.oidcClaims.azp", "scicat", true, false},
		{"arbitrary oidc claim mismatch", testuser, "profile.oidcClaims.azp", "account", false, false},
		{"client role", testuser, "profile.oidcClaims.resource_access.scicat.roles", "scicat-user", true, false},
		{"client role mismatch", testuser, "profile.oidcClaims.resource_access.scicat.roles", "scicat-admin", false, false},
		{"realm role", testuser, "profile.oidcClaims.realm_access.roles", "facility-user", true, false},
		{"oidc roles claim", testuser, "profile.oidcClaims.roles", "user", true, false},
		{"missing oidc claim", testuser, "profile.oidcClaims.missing", "physics", false, true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result, err := CheckProperty(test.data, test.path, test.value)

			if test.throwErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.expected, result)
		})
	}
}
