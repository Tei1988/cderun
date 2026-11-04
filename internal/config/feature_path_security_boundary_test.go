package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Path_DNSValidation_Boundaries(t *testing.T) {
	t.Parallel()

	t.Run("ValidateDNS", func(t *testing.T) {
		t.Parallel()

		// Empty string is allowed (no-op)
		assert.NoError(t, ValidateDNS(""))

		// Valid IPv4 and IPv6
		assert.NoError(t, ValidateDNS("8.8.8.8"))
		assert.NoError(t, ValidateDNS("1.1.1.1"))
		assert.NoError(t, ValidateDNS("2001:db8::1"))
		assert.NoError(t, ValidateDNS("::1"))

		// Invalid IP addresses
		assert.ErrorContains(t, ValidateDNS("invalid-ip"), "invalid DNS IP")
		assert.ErrorContains(t, ValidateDNS("999.999.999.999"), "invalid DNS IP")
		assert.ErrorContains(t, ValidateDNS("8.8.8.8.8"), "invalid DNS IP")

		// Control characters rejected by validatePathChars
		assert.ErrorContains(t, ValidateDNS("8.8.8.8\n"), "invalid character")
		assert.ErrorContains(t, ValidateDNS("8.8.8.8\x00"), "invalid character")
	})

	t.Run("ValidateAddHost", func(t *testing.T) {
		t.Parallel()

		// Empty string is allowed
		assert.NoError(t, ValidateAddHost(""))

		// Valid mappings
		assert.NoError(t, ValidateAddHost("example.com:127.0.0.1"))
		assert.NoError(t, ValidateAddHost("api.internal:10.0.0.1"))
		assert.NoError(t, ValidateAddHost("host.docker.internal:host-gateway"))
		assert.NoError(t, ValidateAddHost("my-host:2001:db8::1"))

		// Missing colon delimiter
		assert.ErrorContains(t, ValidateAddHost("example.com"), "expected host:ip")

		// Empty host part
		assert.ErrorContains(t, ValidateAddHost(":127.0.0.1"), "empty host")

		// Invalid hostname format
		assert.ErrorContains(t, ValidateAddHost("-invalid-.com:127.0.0.1"), "invalid host in add-host")
		assert.ErrorContains(t, ValidateAddHost("invalid..host:127.0.0.1"), "invalid host in add-host")

		// Invalid IP part
		assert.ErrorContains(t, ValidateAddHost("example.com:999.999.999.999"), "invalid IP in add-host")
		assert.ErrorContains(t, ValidateAddHost("example.com:not-an-ip"), "invalid IP in add-host")

		// Control characters
		assert.ErrorContains(t, ValidateAddHost("example.com\n:127.0.0.1"), "invalid character")
		assert.ErrorContains(t, ValidateAddHost("example.com:127.0.0.1\x00"), "invalid character")
	})

	t.Run("ValidateDNSOption", func(t *testing.T) {
		t.Parallel()

		// Empty string is allowed
		assert.NoError(t, ValidateDNSOption(""))

		// Valid DNS options
		assert.NoError(t, ValidateDNSOption("ndots:5"))
		assert.NoError(t, ValidateDNSOption("timeout:2"))
		assert.NoError(t, ValidateDNSOption("attempts:3"))
		assert.NoError(t, ValidateDNSOption("edns0"))
		assert.NoError(t, ValidateDNSOption("rotate"))
		assert.NoError(t, ValidateDNSOption("single-request_reopen"))

		// Parent directory traversal rejection
		assert.ErrorContains(t, ValidateDNSOption("../ndots:5"), "contains parent directory references")
		assert.ErrorContains(t, ValidateDNSOption("opt:.."), "contains parent directory references")

		// Forbidden characters (semicolons, spaces, symbols)
		assert.ErrorContains(t, ValidateDNSOption("ndots:5; rm -rf /"), "invalid characters in DNS option")
		assert.ErrorContains(t, ValidateDNSOption("ndots:5 space"), "invalid characters in DNS option")
		assert.ErrorContains(t, ValidateDNSOption("opt$val"), "invalid characters in DNS option")

		// Control characters
		assert.ErrorContains(t, ValidateDNSOption("ndots:5\n"), "invalid character")
	})
}

func TestUnit_Path_PortValidation_Boundaries(t *testing.T) {
	t.Parallel()

	// Empty string is allowed
	assert.NoError(t, ValidatePort(""))

	// Container port only
	assert.NoError(t, ValidatePort("80"))
	assert.NoError(t, ValidatePort("8080-8085"))
	assert.NoError(t, ValidatePort("8080/tcp"))
	assert.NoError(t, ValidatePort("8080/udp"))

	// Rejection of invalid container port / ranges
	assert.ErrorContains(t, ValidatePort("0"), "port cannot be zero")
	assert.ErrorContains(t, ValidatePort("65536"), "port out of range")
	assert.ErrorContains(t, ValidatePort("8085-8080"), "invalid port range: 8085 > 8080")
	assert.ErrorContains(t, ValidatePort("abc"), "invalid port")
	assert.ErrorContains(t, ValidatePort("80/sctp"), "invalid protocol")

	// HostPort:ContainerPort
	assert.NoError(t, ValidatePort("8080:80"))
	assert.NoError(t, ValidatePort("8080-8082:80-82"))
	assert.NoError(t, ValidatePort("0:8080")) // host port 0 is allowed for dynamic allocation

	// Mismatched range lengths
	assert.ErrorContains(t, ValidatePort("8080-8082:80-81"), "range lengths must match: 3 vs 2")
	assert.ErrorContains(t, ValidatePort("8080:80-81"), "range lengths must match: 1 vs 2")

	// IP:ContainerPort
	assert.NoError(t, ValidatePort("127.0.0.1:8080"))
	assert.NoError(t, ValidatePort("10.0.0.1:8080/tcp"))
	assert.ErrorContains(t, ValidatePort("invalid-ip:8080"), "invalid host port or IP")

	// IP:HostPort:ContainerPort
	assert.NoError(t, ValidatePort("127.0.0.1:8080:80"))
	assert.NoError(t, ValidatePort("127.0.0.1:0:8080"))
	assert.NoError(t, ValidatePort("127.0.0.1::8080")) // empty host port
	assert.NoError(t, ValidatePort("127.0.0.1:8080-8082:80-82/tcp"))

	// IP:HostPort:ContainerPort errors
	assert.ErrorContains(t, ValidatePort("127.0.0.1:8080-8082:80-81"), "range lengths must match")
	assert.ErrorContains(t, ValidatePort("999.999.999.999:8080:80"), "invalid IP")
	assert.ErrorContains(t, ValidatePort("127.0.0.1:8080:0"), "port cannot be zero")
	assert.ErrorContains(t, ValidatePort("127.0.0.1:8080:80:extra"), "invalid port format")

	// Control character check
	assert.ErrorContains(t, ValidatePort("8080\n"), "invalid character")
	assert.ErrorContains(t, ValidatePort("127.0.0.1:8080\x00"), "invalid character")
}

func TestUnit_Path_CapabilityAndSecurityOpt_Boundaries(t *testing.T) {
	t.Parallel()

	t.Run("ValidateCapability", func(t *testing.T) {
		t.Parallel()

		// Empty string is allowed
		assert.NoError(t, ValidateCapability(""))

		// Valid capabilities
		assert.NoError(t, ValidateCapability("SYS_ADMIN"))
		assert.NoError(t, ValidateCapability("NET_BIND_SERVICE"))
		assert.NoError(t, ValidateCapability("CHOWN"))
		assert.NoError(t, ValidateCapability("ALL"))
		assert.NoError(t, ValidateCapability("CAP1"))

		// Invalid format
		assert.ErrorContains(t, ValidateCapability("cap_sys_admin"), "invalid Linux capability")
		assert.ErrorContains(t, ValidateCapability("SYS-ADMIN"), "invalid Linux capability")
		assert.ErrorContains(t, ValidateCapability("SYS__ADMIN"), "invalid Linux capability")
		assert.ErrorContains(t, ValidateCapability("_SYS_ADMIN"), "invalid Linux capability")
		assert.ErrorContains(t, ValidateCapability("SYS_ADMIN_"), "invalid Linux capability")
		assert.ErrorContains(t, ValidateCapability("1SYS_ADMIN"), "invalid Linux capability")

		// Control character check
		assert.ErrorContains(t, ValidateCapability("SYS_ADMIN\n"), "invalid character")
	})

	t.Run("ValidateSecurityOpt", func(t *testing.T) {
		t.Parallel()

		// Empty string is allowed
		assert.NoError(t, ValidateSecurityOpt(""))

		// Valid security options
		assert.NoError(t, ValidateSecurityOpt("no-new-privileges:true"))
		assert.NoError(t, ValidateSecurityOpt("apparmor=unconfined"))
		assert.NoError(t, ValidateSecurityOpt("seccomp=/etc/cderun/seccomp.json"))
		assert.NoError(t, ValidateSecurityOpt("label=disable"))
		assert.NoError(t, ValidateSecurityOpt("mask=/proc/sys"))

		// Parent directory traversal rejection
		assert.ErrorContains(t, ValidateSecurityOpt("../seccomp.json"), "contains parent directory references")
		assert.ErrorContains(t, ValidateSecurityOpt("seccomp:../profile.json"), "contains parent directory references")

		// Forbidden characters (semicolons, backticks, spaces)
		assert.ErrorContains(t, ValidateSecurityOpt("apparmor;reboot"), "invalid characters in security option")
		assert.ErrorContains(t, ValidateSecurityOpt("label=disable "), "invalid characters in security option")
		assert.ErrorContains(t, ValidateSecurityOpt("seccomp=`whoami`"), "invalid characters in security option")
	})

	t.Run("ValidateSysctlKeyAndValue", func(t *testing.T) {
		t.Parallel()

		// Empty key rejection
		assert.ErrorContains(t, ValidateSysctlKey(""), "sysctl key cannot be empty")

		// Valid keys
		assert.NoError(t, ValidateSysctlKey("net.ipv4.ip_forward"))
		assert.NoError(t, ValidateSysctlKey("kernel.msgmax"))
		assert.NoError(t, ValidateSysctlKey("fs.mqueue.queues_max"))
		assert.NoError(t, ValidateSysctlKey("net.core.somaxconn-1"))

		// Invalid keys
		assert.ErrorContains(t, ValidateSysctlKey(".net.ipv4"), "leading, trailing, or consecutive dots")
		assert.ErrorContains(t, ValidateSysctlKey("net.ipv4."), "leading, trailing, or consecutive dots")
		assert.ErrorContains(t, ValidateSysctlKey("net..ipv4"), "leading, trailing, or consecutive dots")
		assert.ErrorContains(t, ValidateSysctlKey("net.ipv4;cmd"), "invalid characters in sysctl key")
		assert.ErrorContains(t, ValidateSysctlKey("net.ipv4\n"), "invalid character")

		// Valid values
		assert.NoError(t, ValidateSysctlValue("1"))
		assert.NoError(t, ValidateSysctlValue("1024 2048"))
		assert.NoError(t, ValidateSysctlValue("val-1_2,3"))
		assert.NoError(t, ValidateSysctlValue(""))

		// Invalid values
		assert.ErrorContains(t, ValidateSysctlValue("1; rm -rf"), "invalid characters in sysctl value")
		assert.ErrorContains(t, ValidateSysctlValue("1\n"), "invalid character")
	})
}

func TestUnit_Path_MountAndTraversal_Boundaries(t *testing.T) {
	t.Parallel()

	t.Run("ValidateMountType", func(t *testing.T) {
		t.Parallel()

		// Valid mount types
		assert.NoError(t, ValidateMountType(""))
		assert.NoError(t, ValidateMountType("bind"))
		assert.NoError(t, ValidateMountType("volume"))
		assert.NoError(t, ValidateMountType("tmpfs"))

		// Unsupported mount types
		assert.ErrorContains(t, ValidateMountType("overlay"), "unsupported mount type")
		assert.ErrorContains(t, ValidateMountType("nfs"), "unsupported mount type")

		// Control characters
		assert.ErrorContains(t, ValidateMountType("bind\n"), "invalid character")
	})

	t.Run("ContainsNumericGID", func(t *testing.T) {
		t.Parallel()

		// Empty slice
		assert.False(t, ContainsNumericGID(nil))
		assert.False(t, ContainsNumericGID([]string{}))

		// Numeric GIDs
		assert.True(t, ContainsNumericGID([]string{"1000"}))
		assert.True(t, ContainsNumericGID([]string{"0"}))
		assert.True(t, ContainsNumericGID([]string{"root", "1000", "docker"}))

		// Non-numeric GIDs
		assert.False(t, ContainsNumericGID([]string{"root"}))
		assert.False(t, ContainsNumericGID([]string{"root", "docker"}))
		assert.False(t, ContainsNumericGID([]string{"1000a", ""}))
	})

	t.Run("HasParentTraversal", func(t *testing.T) {
		t.Parallel()

		// Safe paths
		assert.False(t, HasParentTraversal(""))
		assert.False(t, HasParentTraversal("."))
		assert.False(t, HasParentTraversal("/usr/local/bin"))
		assert.False(t, HasParentTraversal(".../test"))
		assert.False(t, HasParentTraversal("foo..bar"))

		// Traversal paths
		assert.True(t, HasParentTraversal(".."))
		assert.True(t, HasParentTraversal("../test"))
		assert.True(t, HasParentTraversal("test/.."))
		assert.True(t, HasParentTraversal("/a/../b"))
		assert.True(t, HasParentTraversal(`a\..\b`))
		assert.True(t, HasParentTraversal("source:../dest"))
		assert.True(t, HasParentTraversal(`C:\..\windows`))
	})
}
