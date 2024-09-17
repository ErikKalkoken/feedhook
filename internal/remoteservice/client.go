package remoteservice

import (
	"fmt"
	"net/rpc"
)

// Client represents a convenience client for accessing the remote service.
type Client struct {
	port int
}

func NewClient(port int) Client {
	c := Client{port: port}
	return c
}

func (c Client) Statistics() (string, error) {
	rc, err := c.dial()
	if err != nil {
		return "", err
	}
	args := EmptyArgs{}
	var reply string
	if err := rc.Call("RemoteService.Statistics", args, &reply); err != nil {
		return "", fmt.Errorf("call: %w", err)
	}
	return reply, nil
}

func (c Client) SendPing(webhookName string) error {
	rc, err := c.dial()
	if err != nil {
		return err
	}
	args := SendPingArgs{Name: webhookName}
	var reply bool
	if err = rc.Call("RemoteService.SendPing", args, &reply); err != nil {
		return fmt.Errorf("dialing: %w", err)
	}
	return nil
}

func (c Client) dial() (*rpc.Client, error) {
	rc, err := rpc.DialHTTP("tcp", fmt.Sprintf("localhost:%d", c.port))
	if err != nil {
		return nil, fmt.Errorf("dialing: %w", err)
	}
	return rc, nil
}
