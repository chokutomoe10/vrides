package messaging

import (
	"errors"
	"log"
	"net/http"
	"vrides/shared/contracts"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	ErrConnectionNotFound = errors.New("connection not found")
	upgrades              = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type connWrapper struct {
	conn  *websocket.Conn
	mutex sync.Mutex
}

type ConnectionManager struct {
	connections map[string]*connWrapper
	mutex       sync.RWMutex
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*connWrapper),
	}
}

func (c *ConnectionManager) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	conn, err := upgrades.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *ConnectionManager) Add(id string, conn *websocket.Conn) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.connections[id] = &connWrapper{
		conn:  conn,
		mutex: sync.Mutex{},
	}

	log.Printf("Added connection for user %s", id)
}

func (c *ConnectionManager) Remove(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.connections, id)
}

func (c *ConnectionManager) Get(id string) (*websocket.Conn, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	wrapper, exist := c.connections[id]

	if !exist {
		return nil, false
	}

	return wrapper.conn, true
}

func (c *ConnectionManager) SendMessage(id string, msg contracts.WSMessage) error {
	c.mutex.RLock()
	wrapper, exist := c.connections[id]
	c.mutex.RUnlock()

	if !exist {
		return ErrConnectionNotFound
	}

	wrapper.mutex.Lock()
	defer wrapper.mutex.Unlock()

	return wrapper.conn.WriteJSON(msg)
}
