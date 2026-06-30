// Package device resolves a USB iOS device and opens a house_arrest AFC session.
package device

import (
	"fmt"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/house_arrest"

	"github.com/davidliu/lrpush/internal/afcfs"
)

// Session is an open AFC connection to one app's container.
type Session struct {
	FS     afcfs.FS
	Label  string
	closer func() error
}

func (s *Session) Close() error {
	if s.closer != nil {
		return s.closer()
	}
	return nil
}

// DescribeDevice returns a short label for a device (its udid/serial).
func DescribeDevice(d ios.DeviceEntry) string {
	return d.Properties.SerialNumber
}

// Connect resolves the target device (empty udid -> first device) and opens a
// house_arrest AFC client for bundleID.
func Connect(udid, bundleID string) (*Session, error) {
	d, err := ios.GetDevice(udid)
	if err != nil {
		return nil, fmt.Errorf("resolve device: %w", err)
	}
	client, err := house_arrest.New(d, bundleID)
	if err != nil {
		return nil, fmt.Errorf("house_arrest connect to %s: %w", bundleID, err)
	}
	return &Session{
		FS:     afcfs.Wrap(client),
		Label:  DescribeDevice(d),
		closer: client.Close,
	}, nil
}
