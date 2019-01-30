package mpv

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

// Response received from mpv. Can be an event or a user requested response.
type Response struct {
	Err       string          `json:"error"`
	Data      json.RawMessage `json:"data"` // May contain float64, bool or string
	Event     string          `json:"event"`
	RequestID int             `json:"request_id"`
}

// request sent to mpv. Includes request_id for mapping the response.
type request struct {
	Command   []interface{}  `json:"command"`
	RequestID int            `json:"request_id"`
	Response  chan *Response `json:"-"`
}

func newRequest(cmd ...interface{}) *request {
	return &request{
		Command:   cmd,
		RequestID: rand.Intn(10000),
		Response:  make(chan *Response, 1),
	}
}

// LLClient is the most low level interface
type LLClient interface {
	Exec(command ...interface{}) (*Response, error)
	Close() error
}

// IPCClient is a low-level IPC client to communicate with the mpv player via socket.
type IPCClient struct {
	cx      context.Context
	conn    net.Conn
	cancel  context.CancelFunc
	socket  string
	timeout time.Duration
	comm    chan *request

	mu     sync.Mutex
	reqMap map[int]*request // Maps RequestIDs to Requests for response association
}

// NewIPCClient creates a new IPCClient connected to the given socket.
func NewIPCClient(cx context.Context, socket string) (*IPCClient, error) {
	ctx, cancel := context.WithCancel(cx)
	c := &IPCClient{
		cx:      ctx,
		cancel:  cancel,
		socket:  socket,
		timeout: 2 * time.Second,
		comm:    make(chan *request),
		reqMap:  make(map[int]*request),
	}
	err := c.run(ctx)
	return c, err
}

// dispatch dispatches responses to the corresponding request
func (c *IPCClient) dispatch(cx context.Context, resp *Response) {
	if resp.Event == "" { // No Event
		c.mu.Lock()
		// Lookup requestID in request map
		if req, ok := c.reqMap[resp.RequestID]; ok {
			delete(c.reqMap, resp.RequestID)
			c.mu.Unlock()
			select {
			case req.Response <- resp:
			case <-cx.Done():
			}
			return
		}
		c.mu.Unlock()
		// Discard response
	} else { // Event
		// TODO: Implement Event support
	}

}

func (c *IPCClient) run(cx context.Context) error {
	dl := &net.Dialer{
		Timeout: c.timeout,
	}
	var err error
	c.conn, err = dl.DialContext(cx, "unix", c.socket)
	if err != nil {
		return err
	}
	go c.readloop(cx, c.conn)
	go func() {
		if err := c.writeloop(cx, c.conn); err != nil {
			fmt.Printf("%#v", err)
		}
	}()
	return nil
}

func (c *IPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancel()
	err := c.conn.Close()
	return err
}

func (c *IPCClient) writeloop(cx context.Context, conn io.Writer) error {
	for {
		select {
		default:
		case <-cx.Done():
			return cx.Err()
		}
		var ok bool
		var req *request
		select {
		case req, ok = <-c.comm:
			if !ok {
				return errors.New("Communication channel closed")
			}
		case <-cx.Done():
			return cx.Err()
		}
		b, err := json.Marshal(req)
		if err != nil {
			// TODO: Discard request, maybe send error downstream
			log.Printf("Discard request %v with error: %s", req, err)
			continue
		}
		c.mu.Lock()
		c.reqMap[req.RequestID] = req
		c.mu.Unlock()
		b = append(b, '\n')
		_, err = conn.Write(b)
		if err != nil {
			// TODO: Discard request, maybe send error downstream
			// TODO: Remove from reqMap?
		}
	}
}

func (c *IPCClient) readloop(cx context.Context, conn io.Reader) {
	rd := bufio.NewReader(conn)
	for {
		select {
		default:
		case <-cx.Done():
			return
		}
		data, err := rd.ReadBytes('\n')
		if err != nil {
			// TODO: Handle error
			continue
		}
		var resp Response
		err = json.Unmarshal(data, &resp)
		if err != nil {
			// TODO: Handle error
			continue
		}
		c.dispatch(cx, &resp)
	}
}

// Timeout errors while communicating via IPC
var (
	ErrTimeoutSend = errors.New("Timeout while sending command")
	ErrTimeoutRecv = errors.New("Timeout while receiving response")
)

// The client should be restarted
var ChannelErr = errors.New("Response channel closed")

// Exec executes a command via ipc and returns the response.
// A request can timeout while sending or while waiting for the response.
// An error is only returned if there was an error in the communication.
// The client has to check for `response.Error` in case the server returned
// an error.
func (c *IPCClient) Exec(command ...interface{}) (*Response, error) {
	req := newRequest(command...)
	timer := time.NewTimer(c.timeout)
	select {
	case <-c.cx.Done():
		timer.Stop()
		return nil, c.cx.Err()
	case c.comm <- req:
		timer.Stop()
	case <-timer.C:
		return nil, ErrTimeoutSend
	}
	timer = time.NewTimer(c.timeout)
	select {
	case <-c.cx.Done():
		timer.Stop()
		return nil, c.cx.Err()
	case res, ok := <-req.Response:
		timer.Stop()
		if !ok {
			return nil, ChannelErr
		}
		return res, nil
	case <-timer.C:
		return nil, ErrTimeoutRecv
	}
}
