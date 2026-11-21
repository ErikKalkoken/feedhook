package remote

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"time"
)

// DefaultTimeout is the default timeout for calls to the remote service.
const DefaultTimeout = 60 * time.Second

// Client represents a convenience client for accessing the remote service.
type Client struct {
	port    int
	timeout time.Duration
}

// NewClient returns a new client using the default port and timeout.
func NewClient() *Client {
	return &Client{port: DefaultPort, timeout: DefaultTimeout}
}

// SetPort sets the port of the remote service. It reports an error for invalid ports.
func (c *Client) SetPort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	c.port = port
	return nil
}

// SetTimeout sets the timeout for calls. It reports an error for invalid timeouts.
func (c *Client) SetTimeout(d time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("invalid timeout: %s", d)
	}
	c.timeout = d
	return nil
}

// PostLatestFeedItem posts the latest item of a feed to its webhook.
func (c *Client) PostLatestFeedItem(feedName string) error {
	var reply bool
	return c.call("RemoteService.PostLatestFeedItem", SendLatestArgs{FeedName: feedName}, &reply)
}

// Restart restarts the service.
func (c *Client) Restart() error {
	var reply bool
	return c.call("RemoteService.Restart", EmptyArgs{}, &reply)
}

// Statistics returns the service's statistics as text tables.
func (c *Client) Statistics() (string, error) {
	var reply string
	if err := c.call("RemoteService.Statistics", EmptyArgs{}, &reply); err != nil {
		return "", err
	}
	return reply, nil
}

// SendPing sends a test message to a webhook.
func (c *Client) SendPing(webhookName string) error {
	var reply bool
	return c.call("RemoteService.SendPing", SendPingArgs{WebhookName: webhookName}, &reply)
}

// call calls a method of the remote service and reports timeouts with a clear error.
func (c *Client) call(method string, args any, reply any) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", c.port), c.timeout)
	if err != nil {
		return c.replaceTimeoutError(fmt.Errorf("dialing: %w", err))
	}
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		conn.Close()
		return err
	}
	rc := rpc.NewClient(conn)
	defer rc.Close()
	return c.replaceTimeoutError(rc.Call(method, args, reply))
}

// replaceTimeoutError returns a custom error when err is a timeout, else err unchanged.
func (c *Client) replaceTimeoutError(err error) error {
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		return fmt.Errorf("no answer from feedhook service on port %d within %s", c.port, c.timeout)
	}
	return err
}
