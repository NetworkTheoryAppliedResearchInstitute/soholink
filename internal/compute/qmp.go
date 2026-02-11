package compute

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// QMPClient communicates with QEMU via the QMP (QEMU Machine Protocol) socket.
type QMPClient struct {
	socketPath string
}

// NewQMPClient creates a QMP client for the given socket path.
func NewQMPClient(socketPath string) *QMPClient {
	return &QMPClient{socketPath: socketPath}
}

type qmpCommand struct {
	Execute   string      `json:"execute"`
	Arguments interface{} `json:"arguments,omitempty"`
}

type qmpResponse struct {
	Return json.RawMessage `json:"return,omitempty"`
	Error  *qmpError       `json:"error,omitempty"`
}

type qmpError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

// Execute sends a QMP command and returns the response.
func (c *QMPClient) Execute(command string, args interface{}) (json.RawMessage, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("qmp connect failed: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	// Read QMP greeting
	buf := make([]byte, 4096)
	if _, err := conn.Read(buf); err != nil {
		return nil, fmt.Errorf("qmp greeting failed: %w", err)
	}

	// Send qmp_capabilities to enter command mode
	capCmd := qmpCommand{Execute: "qmp_capabilities"}
	if err := json.NewEncoder(conn).Encode(capCmd); err != nil {
		return nil, fmt.Errorf("qmp capabilities failed: %w", err)
	}
	if _, err := conn.Read(buf); err != nil {
		return nil, fmt.Errorf("qmp capabilities response failed: %w", err)
	}

	// Send actual command
	cmd := qmpCommand{Execute: command, Arguments: args}
	if err := json.NewEncoder(conn).Encode(cmd); err != nil {
		return nil, fmt.Errorf("qmp command send failed: %w", err)
	}

	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("qmp response read failed: %w", err)
	}

	var resp qmpResponse
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		return nil, fmt.Errorf("qmp response parse failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("qmp error: %s - %s", resp.Error.Class, resp.Error.Desc)
	}

	return resp.Return, nil
}

// SystemPowerdown sends ACPI shutdown signal.
func (c *QMPClient) SystemPowerdown() error {
	_, err := c.Execute("system_powerdown", nil)
	return err
}

// Cont resumes a paused VM.
func (c *QMPClient) Cont() error {
	_, err := c.Execute("cont", nil)
	return err
}

// Stop pauses a VM.
func (c *QMPClient) Stop() error {
	_, err := c.Execute("stop", nil)
	return err
}

// Quit forcefully terminates QEMU.
func (c *QMPClient) Quit() error {
	_, err := c.Execute("quit", nil)
	return err
}
