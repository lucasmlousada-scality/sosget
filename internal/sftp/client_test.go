package sftp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeviceIndex(t *testing.T) {
	prompt := "Second factor devices found:\n0: Google Authenticator\n1: OneLogin Protect\nEnter a number:"

	// Matches the requested device by its menu index.
	assert.Equal(t, "0", deviceIndex(prompt, "Google Authenticator"))
	assert.Equal(t, "1", deviceIndex(prompt, "OneLogin Protect"))

	// Unknown device falls back to "0".
	assert.Equal(t, "0", deviceIndex(prompt, "Nonexistent Device"))

	// Empty device name defaults to Google Authenticator (index 0 here).
	assert.Equal(t, "0", deviceIndex(prompt, ""))
}

func TestIsDeviceChooser(t *testing.T) {
	assert.True(t, isDeviceChooser("second factor devices found"))
	assert.True(t, isDeviceChooser("please choose a device"))
	assert.True(t, isDeviceChooser("enter a number"))
	assert.False(t, isDeviceChooser("password:"))
	assert.False(t, isDeviceChooser("verification code:"))
}

func TestContainsAny(t *testing.T) {
	assert.True(t, containsAny("the password field", "password", "passwd"))
	assert.True(t, containsAny("enter passwd now", "password", "passwd"))
	assert.False(t, containsAny("token code", "password", "passwd"))
}

func TestCustomerPathForUser(t *testing.T) {
	got := CustomerPathForUser("/customers", "jane.doe")
	assert.Equal(t, "/customers/chroot-jane.doe/home/jane.doe", got)
}
