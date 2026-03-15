//go:build darwin

package tape

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// Peer represents a remote tape participant.
type Peer struct {
	Addr     string
	NodeID   string
	conn     net.Conn
	encoder  *json.Encoder
	LastSeen time.Time
}

// Server accepts incoming tape connections and broadcasts frames.
type Server struct {
	mu       sync.RWMutex
	listener net.Listener
	peers    map[string]*Peer
	recorder *Recorder
	stop     chan struct{}
	done     chan struct{}
}

// NewServer creates a tape sharing server.
func NewServer(addr string, recorder *Recorder) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tape server listen: %w", err)
	}

	s := &Server{
		listener: ln,
		peers:    make(map[string]*Peer),
		recorder: recorder,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	// Wire up recorder broadcast
	recorder.OnFrame(func(f Frame) {
		s.Broadcast(f)
	})

	return s, nil
}

// Addr returns the listening address.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// Serve starts accepting connections. Blocks until Stop is called.
func (s *Server) Serve() {
	defer close(s.done)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
				continue
			}
		}
		go s.handleConn(conn)
	}
}

// Stop shuts down the server.
func (s *Server) Stop() {
	close(s.stop)
	s.listener.Close()

	s.mu.Lock()
	for _, p := range s.peers {
		p.conn.Close()
	}
	s.mu.Unlock()

	<-s.done
}

// Broadcast sends a frame to all connected peers.
func (s *Server) Broadcast(f Frame) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, p := range s.peers {
		if err := p.encoder.Encode(f); err != nil {
			// Peer disconnected, will be cleaned up
			_ = id
		}
	}
}

// PeerCount returns the number of connected peers.
func (s *Server) PeerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.peers)
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	// First message is a handshake with the peer's NodeID
	if !scanner.Scan() {
		return
	}

	var handshake struct {
		NodeID string `json:"node_id"`
		Label  string `json:"label"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &handshake); err != nil {
		return
	}

	peer := &Peer{
		Addr:     conn.RemoteAddr().String(),
		NodeID:   handshake.NodeID,
		conn:     conn,
		encoder:  json.NewEncoder(conn),
		LastSeen: time.Now(),
	}

	s.mu.Lock()
	s.peers[peer.NodeID] = peer
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.peers, peer.NodeID)
		s.mu.Unlock()
	}()

	// Read frames from peer and ingest into local recorder
	for scanner.Scan() {
		select {
		case <-s.stop:
			return
		default:
		}

		var f Frame
		if err := json.Unmarshal(scanner.Bytes(), &f); err != nil {
			continue
		}

		peer.LastSeen = time.Now()
		s.recorder.IngestRemoteFrame(f)
	}
}

// Client connects to a remote tape server and receives frames.
type Client struct {
	mu       sync.Mutex
	conn     net.Conn
	nodeID   string
	label    string
	encoder  *json.Encoder
	recorder *Recorder
	stop     chan struct{}
	done     chan struct{}
}

// Dial connects to a tape server and begins frame exchange.
func Dial(addr, nodeID, label string, recorder *Recorder) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("tape dial: %w", err)
	}

	c := &Client{
		conn:     conn,
		nodeID:   nodeID,
		label:    label,
		encoder:  json.NewEncoder(conn),
		recorder: recorder,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	// Send handshake
	handshake := struct {
		NodeID string `json:"node_id"`
		Label  string `json:"label"`
	}{NodeID: nodeID, Label: label}

	if err := c.encoder.Encode(handshake); err != nil {
		conn.Close()
		return nil, err
	}

	// Wire up recorder broadcast to send local frames to server
	recorder.OnFrame(func(f Frame) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.encoder.Encode(f) //nolint:errcheck
	})

	// Start receiving
	go c.receiveLoop()

	return c, nil
}

// Close disconnects from the server.
func (c *Client) Close() {
	close(c.stop)
	c.conn.Close()
	<-c.done
}

func (c *Client) receiveLoop() {
	defer close(c.done)

	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-c.stop:
			return
		default:
		}

		var f Frame
		if err := json.Unmarshal(scanner.Bytes(), &f); err != nil {
			continue
		}

		c.recorder.IngestRemoteFrame(f)
	}
}
