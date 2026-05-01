package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/xboard/xboard/internal/config"
	"gorm.io/gorm"
)

const (
	heartbeatInterval = 10 * time.Second
	pingInterval      = 55 * time.Second
	authTimeout       = 10 * time.Second
	maxMessageSize    = 65536
	writeWait         = 10 * time.Second
)

// NodeType describes the type of node connection
type NodeType int

const (
	NodeTypeSingle NodeType = iota
	NodeTypeMachine
)

// Message represents a WebSocket message
type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// NodeConnection represents an authenticated node connection
type NodeConnection struct {
	ID            string
	Conn          *websocket.Conn
	Authenticated bool
	NodeType      NodeType
	MachineID     uint
	NodeIDs       []uint
	LastPong      time.Time
	mu            sync.Mutex
}

// Server manages WebSocket connections from service nodes
type Server struct {
	config   *config.Config
	db       *gorm.DB
	rdb      *redis.Client
	upgrader websocket.Upgrader
	mu       sync.RWMutex
	nodes    map[string]*NodeConnection
	machines map[uint]*NodeConnection
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewServer(cfg *config.Config, db *gorm.DB, rdb *redis.Client) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		config: cfg,
		db:     db,
		rdb:    rdb,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		nodes:    make(map[string]*NodeConnection),
		machines: make(map[uint]*NodeConnection),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// HandleConnection handles a new WebSocket connection
func (s *Server) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	nc := &NodeConnection{
		ID:       fmt.Sprintf("conn_%d", time.Now().UnixNano()),
		Conn:     conn,
		LastPong: time.Now(),
	}

	go func() {
		time.Sleep(authTimeout)
		nc.mu.Lock()
		if !nc.Authenticated {
			conn.Close()
		}
		nc.mu.Unlock()
	}()

	go s.handleConnection(nc)
}

func (s *Server) handleConnection(nc *NodeConnection) {
	defer func() {
		nc.mu.Lock()
		nc.Conn.Close()
		nc.mu.Unlock()
		s.removeNode(nc)
	}()

	nc.Conn.SetReadLimit(maxMessageSize)
	nc.Conn.SetPongHandler(func(string) error {
		nc.mu.Lock()
		nc.LastPong = time.Now()
		nc.mu.Unlock()
		return nil
	})

	for {
		_, message, err := nc.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}
		s.handleMessage(nc, &msg)
	}
}

func (s *Server) handleMessage(nc *NodeConnection, msg *Message) {
	switch msg.Type {
	case "auth":
		s.handleAuth(nc, msg.Data)
	case "online":
		s.handleOnline(nc, msg.Data)
	case "traffic":
		s.handleTraffic(nc, msg.Data)
	case "heartbeat":
		s.handleHeartbeat(nc, msg.Data)
	}
}

func (s *Server) handleAuth(nc *NodeConnection, data json.RawMessage) {
	var authReq struct {
		NodeID    uint   `json:"node_id"`
		MachineID uint   `json:"machine_id"`
		Token     string `json:"token"`
		Version   string `json:"version"`
	}
	if err := json.Unmarshal(data, &authReq); err != nil {
		s.sendJSON(nc, Message{Type: "auth_error"})
		return
	}

	nc.Authenticated = true
	if authReq.MachineID > 0 {
		nc.NodeType = NodeTypeMachine
		nc.MachineID = authReq.MachineID
	}
	if authReq.NodeID > 0 {
		nc.NodeIDs = append(nc.NodeIDs, authReq.NodeID)
	}

	s.registerNode(nc)
	s.sendJSON(nc, Message{Type: "auth_success", Data: json.RawMessage(`{"ok":true}`)})
}

func (s *Server) handleOnline(nc *NodeConnection, data json.RawMessage) {
	var onlineData struct {
		NodeID  uint   `json:"node_id"`
		UserIDs []uint `json:"user_ids"`
	}
	if err := json.Unmarshal(data, &onlineData); err != nil {
		return
	}
	key := fmt.Sprintf("node:%d:online", onlineData.NodeID)
	userData, _ := json.Marshal(onlineData.UserIDs)
	s.rdb.Set(s.ctx, key, userData, heartbeatInterval*2)
}

func (s *Server) handleTraffic(nc *NodeConnection, data json.RawMessage) {
	var trafficData []struct {
		NodeID uint  `json:"node_id"`
		UserID uint  `json:"user_id"`
		U      int64 `json:"u"`
		D      int64 `json:"d"`
	}
	if err := json.Unmarshal(data, &trafficData); err != nil {
		return
	}
	trafficJSON, _ := json.Marshal(trafficData)
	s.rdb.LPush(s.ctx, "queue:traffic", trafficJSON)
}

func (s *Server) handleHeartbeat(nc *NodeConnection, data json.RawMessage) {
	var hb struct {
		Load   float64 `json:"load"`
		Uptime uint64  `json:"uptime"`
		NodeID uint    `json:"node_id"`
	}
	json.Unmarshal(data, &hb)
	hbData, _ := json.Marshal(hb)
	s.rdb.Set(s.ctx, fmt.Sprintf("node:%d:heartbeat", hb.NodeID), hbData, heartbeatInterval*3)
}

func (s *Server) sendJSON(nc *NodeConnection, msg Message) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	nc.Conn.WriteJSON(msg)
}

func (s *Server) registerNode(nc *NodeConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if nc.MachineID > 0 {
		s.machines[nc.MachineID] = nc
	}
	for _, nodeID := range nc.NodeIDs {
		s.nodes[fmt.Sprintf("%d", nodeID)] = nc
	}
}

func (s *Server) removeNode(nc *NodeConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, nodeID := range nc.NodeIDs {
		delete(s.nodes, fmt.Sprintf("%d", nodeID))
	}
	if nc.MachineID > 0 {
		delete(s.machines, nc.MachineID)
	}
}

// PushToNode sends data to a specific node
func (s *Server) PushToNode(nodeID uint, data interface{}) error {
	s.mu.RLock()
	nc, ok := s.nodes[fmt.Sprintf("%d", nodeID)]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("node %d not connected", nodeID)
	}
	msg := Message{Type: "push"}
	msg.Data, _ = json.Marshal(data)
	s.sendJSON(nc, msg)
	return nil
}

// Start begins the background maintenance loop
func (s *Server) Start() {
	go s.maintenanceLoop()
	go s.redisSubscribe()
}

// Stop gracefully stops the server
func (s *Server) Stop() {
	s.cancel()
}

func (s *Server) maintenanceLoop() {
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()
	for {
		select {
		case <-pingTicker.C:
			s.pingAll()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) pingAll() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, nc := range s.nodes {
		nc.mu.Lock()
		nc.Conn.SetWriteDeadline(time.Now().Add(writeWait))
		nc.Conn.WriteMessage(websocket.PingMessage, nil)
		nc.mu.Unlock()
	}
}

func (s *Server) redisSubscribe() {
	pubsub := s.rdb.Subscribe(s.ctx, "node:push")
	defer pubsub.Close()
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			var pushMsg struct {
				NodeID uint            `json:"node_id"`
				Data   json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(msg.Payload), &pushMsg); err == nil {
				s.PushToNode(pushMsg.NodeID, pushMsg.Data)
			}
		case <-s.ctx.Done():
			return
		}
	}
}

// NodeRegistry provides access to the websocket server for external modules
type NodeRegistry struct {
	pushFunc func(nodeID uint, data interface{}) error
}

func NewNodeRegistry(s *Server) *NodeRegistry {
	return &NodeRegistry{
		pushFunc: s.PushToNode,
	}
}

func (r *NodeRegistry) PushToNode(nodeID uint, data interface{}) error {
	return r.pushFunc(nodeID, data)
}
