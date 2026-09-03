package config

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
)

func TestMinimalConfig(t *testing.T) {
	content := `
scicatUrl: "http://backend.localhost"
port: 1234
facilities:
  - name: "TestFacility"
    collection: aaaa1111-22bb-cc44-dd5e-666667777777
`

	var typed Config
	err := yaml.Unmarshal([]byte(content), &typed)
	assert.Nil(t, err)

	conf, err := ReadConfigFromBytes([]byte(content))
	assert.Nil(t, err)
	assert.Equal(t, "http://backend.localhost", conf.ScicatUrl)
	assert.EqualValues(t, 1234, conf.Port)
	assert.Equal(t, 1, len(conf.Facilities))

	fac := conf.Facilities[0]
	assert.Equal(t, "TestFacility", fac.Name)
	assert.Equal(t, "aaaa1111-22bb-cc44-dd5e-666667777777", fac.Collection)
	assert.Equal(t, DirectionBoth, fac.Direction) // Default
	assert.Equal(t, 1, len(fac.Scopes))
	if assert.NotNil(t, fac.AccessPath) {
		assert.Equal(t, "profile.accessGroups", *fac.AccessPath) // Default
	}
	if assert.NotNil(t, fac.AccessValue) {
		assert.Equal(t, "{{ .Name }}", *fac.AccessValue) // Default
	}

	scopes, err := conf.GetGlobusScopes()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(scopes))
	assert.Equal(t, []string{"urn:globus:auth:scope:transfer.api.globus.org:all[*https://auth.globus.org/scopes/aaaa1111-22bb-cc44-dd5e-666667777777/data_access]"}, scopes)
}

func TestEmptyAccessPathOverridesDefault(t *testing.T) {
	content := `
scicatUrl: "http://backend.localhost"
port: 1234
facilities:
  - name: "TestFacility"
    collection: aaaa1111-22bb-cc44-dd5e-666667777777
    accessPath: ""
    accessValue: ""
`

	conf, err := ReadConfigFromBytes([]byte(content))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(conf.Facilities))

	fac := conf.Facilities[0]
	if assert.NotNil(t, fac.AccessPath) {
		assert.Equal(t, "", *fac.AccessPath)
	}
	if assert.NotNil(t, fac.AccessValue) {
		assert.Equal(t, "", *fac.AccessValue)
	}
}

func TestGetGlobusScopesByFacility(t *testing.T) {
	content := `
scicatUrl: "http://backend.localhost"
port: 1234
facilities:
  - name: "FacilityA"
    collection: aaaa1111-22bb-cc44-dd5e-666667777777
  - name: "FacilityB"
    collection: bbbb1111-22bb-cc44-dd5e-666667777777
`

	conf, err := ReadConfigFromBytes([]byte(content))
	assert.Nil(t, err)

	scopesByFacility, err := conf.GetGlobusScopesByFacility()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(scopesByFacility))
	assert.Equal(t,
		[]string{"urn:globus:auth:scope:transfer.api.globus.org:all[*https://auth.globus.org/scopes/aaaa1111-22bb-cc44-dd5e-666667777777/data_access]"},
		scopesByFacility["FacilityA"],
	)
	assert.Equal(t,
		[]string{"urn:globus:auth:scope:transfer.api.globus.org:all[*https://auth.globus.org/scopes/bbbb1111-22bb-cc44-dd5e-666667777777/data_access]"},
		scopesByFacility["FacilityB"],
	)
}

func TestGetGlobusScopesPreservesFacilityOrder(t *testing.T) {
	content := `
scicatUrl: "http://backend.localhost"
port: 1234
facilities:
  - name: "FacilityA"
    collection: aaaa1111-22bb-cc44-dd5e-666667777777
  - name: "FacilityB"
    collection: bbbb1111-22bb-cc44-dd5e-666667777777
`

	conf, err := ReadConfigFromBytes([]byte(content))
	assert.Nil(t, err)

	scopes, err := conf.GetGlobusScopes()
	assert.Nil(t, err)
	assert.Equal(t, []string{
		"urn:globus:auth:scope:transfer.api.globus.org:all[*https://auth.globus.org/scopes/aaaa1111-22bb-cc44-dd5e-666667777777/data_access]",
		"urn:globus:auth:scope:transfer.api.globus.org:all[*https://auth.globus.org/scopes/bbbb1111-22bb-cc44-dd5e-666667777777/data_access]",
	}, scopes)
}

func TestYamlMerging(t *testing.T) {
	content := `
scicatUrl: "http://backend.localhost"
port: 1234
defaults:
  - &template
    destinationPath: '/archive/{{ replace .Pid "." "-" }}/{{ .SourceFolder }}'

facilities:
  - <<: *template
    name: "TestFacility"
    collection: aaaa1111-22bb-cc44-dd5e-666667777777
`

	var typed Config
	err := yaml.Unmarshal([]byte(content), &typed)
	assert.Nil(t, err)

	conf, err := ReadConfigFromBytes([]byte(content))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(conf.Facilities))

	fac := conf.Facilities[0]
	assert.Equal(t, "TestFacility", fac.Name)
	assert.Equal(t, "aaaa1111-22bb-cc44-dd5e-666667777777", fac.Collection)
	assert.Equal(t, `/archive/{{ replace .Pid "." "-" }}/{{ .SourceFolder }}`, fac.DestinationPath)
}
