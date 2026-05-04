package guacamole

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// GuacClient implements the Guacamole protocol (guacp) over TCP to guacd.
// Protocol format: each instruction is length-prefixed fields separated by commas, terminated by semicolon.
// Example: "6.select,3.vnc;" means opcode="select", arg1="vnc"
type GuacClient struct {
	conn   net.Conn
	reader *bufio.Reader
	ConnID string
}

// NewGuacClient connects to guacd at the given address (e.g. "localhost:4822").
func NewGuacClient(guacdAddr string) (*GuacClient, error) {
	conn, err := net.DialTimeout("tcp", guacdAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("guacd connect: %w", err)
	}
	return &GuacClient{
		conn:   conn,
		reader: bufio.NewReaderSize(conn, 64*1024),
	}, nil
}

// Connect performs the guacd handshake: select protocol, then send connect with params.
func (c *GuacClient) Connect(protocol string, params map[string]string) error {
	// Step 1: send "select" instruction
	if err := c.WriteInstruction(encodeInstruction("select", protocol)); err != nil {
		return fmt.Errorf("select: %w", err)
	}

	// Step 2: read "args" instruction — lists the parameter names guacd expects
	inst, err := c.ReadInstruction()
	if err != nil {
		return fmt.Errorf("read args: %w", err)
	}

	fields := decodeInstruction(inst)
	if len(fields) == 0 || fields[0] != "args" {
		return fmt.Errorf("expected args instruction, got: %s", inst)
	}

	// Step 3: build "connect" instruction with parameter values in order
	argNames := fields[1:]
	connectArgs := make([]string, len(argNames))
	for i, name := range argNames {
		connectArgs[i] = params[name]
	}

	if err := c.WriteInstruction(encodeInstruction("connect", connectArgs...)); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	// Step 4: read "ready" instruction with connection ID
	inst, err = c.ReadInstruction()
	if err != nil {
		return fmt.Errorf("read ready: %w", err)
	}

	fields = decodeInstruction(inst)
	if len(fields) < 2 || fields[0] != "ready" {
		return fmt.Errorf("expected ready instruction, got: %s", inst)
	}

	c.ConnID = fields[1]
	return nil
}

// ReadInstruction reads a single guacp instruction (terminated by ';').
func (c *GuacClient) ReadInstruction() (string, error) {
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var buf strings.Builder
	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return "", err
		}
		buf.WriteByte(b)
		if b == ';' {
			return buf.String(), nil
		}
	}
}

// WriteInstruction sends a raw guacp instruction string to guacd.
func (c *GuacClient) WriteInstruction(instruction string) error {
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := io.WriteString(c.conn, instruction)
	return err
}

// RawConn returns the underlying TCP connection for bidirectional relay.
func (c *GuacClient) RawConn() net.Conn {
	return c.conn
}

// Close closes the connection to guacd.
func (c *GuacClient) Close() error {
	// Send disconnect
	c.WriteInstruction(encodeInstruction("disconnect"))
	return c.conn.Close()
}

// encodeInstruction builds a guacp instruction string.
// e.g. encodeInstruction("select", "rdp") => "6.select,3.rdp;"
func encodeInstruction(opcode string, args ...string) string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(len(opcode)))
	b.WriteByte('.')
	b.WriteString(opcode)
	for _, arg := range args {
		b.WriteByte(',')
		b.WriteString(strconv.Itoa(len(arg)))
		b.WriteByte('.')
		b.WriteString(arg)
	}
	b.WriteByte(';')
	return b.String()
}

// decodeInstruction parses a guacp instruction into its fields.
// e.g. "4.args,8.hostname,4.port;" => ["args", "hostname", "port"]
func decodeInstruction(inst string) []string {
	inst = strings.TrimSuffix(inst, ";")
	if inst == "" {
		return nil
	}

	var fields []string
	for len(inst) > 0 {
		// Parse length prefix
		dotIdx := strings.IndexByte(inst, '.')
		if dotIdx < 0 {
			break
		}
		length, err := strconv.Atoi(inst[:dotIdx])
		if err != nil {
			break
		}
		inst = inst[dotIdx+1:]
		if len(inst) < length {
			break
		}
		fields = append(fields, inst[:length])
		inst = inst[length:]
		// Skip comma separator
		if len(inst) > 0 && inst[0] == ',' {
			inst = inst[1:]
		}
	}
	return fields
}
