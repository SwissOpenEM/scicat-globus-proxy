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
	assert.Equal(t, "/", fac.CollectionRootPath)
	assert.Equal(t, 1, len(fac.Scopes))

	scopes, err := conf.GetGlobusScopes()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(scopes))
	assert.Equal(t, []string{"urn:globus:auth:scope:transfer.api.globus.org:all[*https://auth.globus.org/scopes/aaaa1111-22bb-cc44-dd5e-666667777777/data_access]"}, scopes)
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
	assert.Equal(t, 1, len(conf.Facilities))

	fac := conf.Facilities[0]
	assert.Equal(t, "TestFacility", fac.Name)
	assert.Equal(t, "aaaa1111-22bb-cc44-dd5e-666667777777", fac.Collection)
	assert.Equal(t, `/archive/{{ replace .Pid "." "-" }}/{{ .SourceFolder }}`, fac.DestinationPath)
}
