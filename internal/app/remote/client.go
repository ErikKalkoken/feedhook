package remote

import (
	"fmt"
	"net/rpc"
)

// Client represents a convenience client for accessing the remote service.
type Client struct {
	port int
}

// NewClient returns a new client using the default port.
func NewClient() *Client {
	return &Client{port: DefaultPort}
}

// SetPort sets the port of the remote service. It reports an error for invalid ports.
func (c *Client) SetPort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	c.port = port
	return nil
}

// CheckConfig reports an error when the service's config file is invalid.
func (c *Client) CheckConfig() error {
	rc, err := c.dial()
	if err != nil {
		return err
	}
	defer rc.Close()
	var reply bool
	return rc.Call("RemoteService.CheckConfig", EmptyArgs{}, &reply)
}

// PostLatestFeedItem posts the latest item of a feed to its webhook.
func (c *Client) PostLatestFeedItem(feedName string) error {
	rc, err := c.dial()
	if err != nil {
		return err
	}
	defer rc.Close()
	args := SendLatestArgs{FeedName: feedName}
	var reply bool
	return rc.Call("RemoteService.PostLatestFeedItem", args, &reply)
}

// Restart restarts the service.
func (c *Client) Restart() error {
	rc, err := c.dial()
	if err != nil {
		return err
	}
	defer rc.Close()
	var reply bool
	return rc.Call("RemoteService.Restart", EmptyArgs{}, &reply)
}

// Statistics returns the service's statistics as text tables.
func (c *Client) Statistics() (string, error) {
	rc, err := c.dial()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	var reply string
	if err := rc.Call("RemoteService.Statistics", EmptyArgs{}, &reply); err != nil {
		return "", fmt.Errorf("call: %w", err)
	}
	return reply, nil
}

// SendPing sends a test message to a webhook.
func (c *Client) SendPing(webhookName string) error {
	rc, err := c.dial()
	if err != nil {
		return err
	}
	defer rc.Close()
	args := SendPingArgs{WebhookName: webhookName}
	var reply bool
	return rc.Call("RemoteService.SendPing", args, &reply)
}

func (c *Client) dial() (*rpc.Client, error) {
	rc, err := rpc.DialHTTP("tcp", fmt.Sprintf("localhost:%d", c.port))
	if err != nil {
		return nil, fmt.Errorf("dialing: %w", err)
	}
	return rc, nil
}
