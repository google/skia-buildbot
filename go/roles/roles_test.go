// Roles are part of the Skia Infra Authorization system.
//
// See go/proxy-auth-skia.
package roles

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromHeader_RoundTripToFromHeader_InvalidRolesAreRemoved(t *testing.T) {
	roles := FromHeader(AllRoles.ToHeader())
	require.Equal(t, AllValidRoles, roles)
}

func TestRoleFromString_NotValidRole_ReturnsInvalidRole(t *testing.T) {
	require.Equal(t, InvalidRole, RoleFromString("this-is-not-a-valid-role"))
}

func TestRolesHas_DoesContainRole_ReturnsTrue(t *testing.T) {
	require.True(t, Roles{Viewer}.Has(Viewer))
}

func TestRolesHas_DoesNotContainRole_ReturnsFalse(t *testing.T) {
	require.False(t, Roles{Viewer}.Has(Editor))
}

func TestRolesHas_RolesIsEmpty_ReturnsFalse(t *testing.T) {
	require.False(t, Roles{}.Has(Editor))
}
