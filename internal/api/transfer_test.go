package api

import (
	"testing"

	"github.com/SwissOpenEM/scicat-globus-proxy/internal/config"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/scicat"
	"github.com/SwissOpenEM/scicat-globus-proxy/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFacility(t *testing.T, name string, accessPath string, accessValue string) Facility {
	t.Helper()
	pathTpl, err := util.NewTypedTemplate[accessPathContext](accessPath)
	require.NoError(t, err)
	valueTpl, err := util.NewTypedTemplate[accessPathContext](accessValue)
	require.NoError(t, err)
	return Facility{
		Name:        name,
		AccessPath:  pathTpl,
		AccessValue: valueTpl,
		Direction:   config.DirectionBoth,
	}
}

func testUser(accessGroups []string) scicat.User {
	var user scicat.User
	user.Profile.Username = "alice"
	user.Profile.AccessGroups = accessGroups
	return user
}

func testDataset(ownerGroup string) scicat.ScicatDataset {
	return scicat.ScicatDataset{
		Pid:        "20.500.123/test-dataset",
		OwnerGroup: ownerGroup,
	}
}

func TestCheckAuthorizationSkipsEmptyFacilityAccessPath(t *testing.T) {
	src := testFacility(t, "SRC", "", "unused")
	dst := testFacility(t, "DST", "", "unused")
	user := testUser([]string{"dataset-group"})
	dataset := testDataset("dataset-group")

	ok, msg, err := checkAuthorization(&user, &src, &dst, &dataset)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "", msg)
}

func TestCheckAuthorizationEnforcesSourceAccessPath(t *testing.T) {
	src := testFacility(t, "SRC", "profile.accessGroups", "{{ .Name }}")
	dst := testFacility(t, "DST", "", "unused")
	user := testUser([]string{"DST", "dataset-group"})
	dataset := testDataset("dataset-group")

	ok, msg, err := checkAuthorization(&user, &src, &dst, &dataset)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "No access to facility SRC", msg)
}

func TestCheckAuthorizationEnforcesDestinationAccessPath(t *testing.T) {
	src := testFacility(t, "SRC", "", "unused")
	dst := testFacility(t, "DST", "profile.accessGroups", "{{ .Name }}")
	user := testUser([]string{"SRC", "dataset-group"})
	dataset := testDataset("dataset-group")

	ok, msg, err := checkAuthorization(&user, &src, &dst, &dataset)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "No access to facility DST", msg)
}

func TestCheckAuthorizationInvalidAccessPathReturnsError(t *testing.T) {
	src := testFacility(t, "SRC", "profile.missingField", "{{ .Name }}")
	dst := testFacility(t, "DST", "", "unused")
	user := testUser([]string{"SRC", "dataset-group"})
	dataset := testDataset("dataset-group")

	ok, msg, err := checkAuthorization(&user, &src, &dst, &dataset)
	require.Error(t, err)
	assert.False(t, ok)
	assert.Equal(t, "", msg)
}

func TestCheckAuthorizationAlwaysChecksDatasetOwnerGroup(t *testing.T) {
	src := testFacility(t, "SRC", "", "unused")
	dst := testFacility(t, "DST", "", "unused")
	user := testUser([]string{"some-other-group"})
	dataset := testDataset("dataset-group")

	ok, msg, err := checkAuthorization(&user, &src, &dst, &dataset)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "No access to dataset 20.500.123/test-dataset", msg)
}
