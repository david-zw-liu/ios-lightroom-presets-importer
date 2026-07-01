package device

import (
	"fmt"
	"testing"

	"github.com/danielpaulus/go-ios/ios"
)

func TestDescribeDevice(t *testing.T) {
	d := ios.DeviceEntry{Properties: ios.DeviceProperties{SerialNumber: "abc123"}}
	if got := DescribeDevice(d); got != "abc123" {
		t.Fatalf("DescribeDevice = %q, want abc123", got)
	}
}

func TestCollectVendableKeepsSuccessesInOrder(t *testing.T) {
	installed := map[string]bool{"com.adobe.lrmobile": true} // only iPad app present
	probe := func(id string) (*Session, error) {
		if installed[id] {
			return &Session{BundleID: id}, nil
		}
		return nil, fmt.Errorf("not installed")
	}
	got, err := collectVendable([]string{"com.adobe.lrmobilephone", "com.adobe.lrmobile"}, probe)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].BundleID != "com.adobe.lrmobile" {
		t.Fatalf("got %+v, want single com.adobe.lrmobile", got)
	}
}

func TestCollectVendableBothInstalled(t *testing.T) {
	probe := func(id string) (*Session, error) { return &Session{BundleID: id}, nil }
	got, err := collectVendable([]string{"com.adobe.lrmobilephone", "com.adobe.lrmobile"}, probe)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].BundleID != "com.adobe.lrmobilephone" {
		t.Fatalf("got %+v, want both, mobilephone first", got)
	}
}

func TestCollectVendableNoneInstalled(t *testing.T) {
	probe := func(id string) (*Session, error) { return nil, fmt.Errorf("nope") }
	if _, err := collectVendable([]string{"a", "b"}, probe); err == nil {
		t.Fatal("expected error when nothing vends")
	}
}
