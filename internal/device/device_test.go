package device

import (
	"testing"

	"github.com/danielpaulus/go-ios/ios"
)

func TestDescribeDevice(t *testing.T) {
	d := ios.DeviceEntry{Properties: ios.DeviceProperties{SerialNumber: "abc123"}}
	if got := DescribeDevice(d); got != "abc123" {
		t.Fatalf("DescribeDevice = %q, want abc123", got)
	}
}
