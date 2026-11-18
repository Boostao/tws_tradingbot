//go:build testing

package ibapi

import "sync/atomic"

// SetConnected sets the EClient to a connected state for testing purposes.
func SetConnected(c *EClient) {
	c.setConnState(CONNECTED)
	conn := &Connection{}
	atomic.StoreInt32(&conn.isConnected, 1)
	c.connPtr.Store(conn)
}
