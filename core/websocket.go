package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WsRequestFunc func(req map[string]interface{}) (id string)
type WsUserEventHandler func(msg []byte)
type WsAuthFunc func(ws *websocket.Conn) (int64, error)
type WsExtractIDFunc func(root map[string]json.RawMessage) (id string, found bool)
type WsRequestIDFunc func(map[string]interface{}) (interface{}, bool)
type WsExtractErrFunc func(root map[string]json.RawMessage) error
type WsAfterConnectFunc func(*WsClient) error

type wsResponse struct {
	Root map[string]json.RawMessage
	Err  error
}

type wsConnWrapper struct {
	*websocket.Conn
}

type WsClient struct {
	url       string
	wsConn    *wsConnWrapper
	wsMu      sync.Mutex
	connected bool
	Ctx       context.Context
	cancel    context.CancelFunc
	started   time.Time
	lifeTime  time.Duration

	requestsMu   sync.Mutex
	requests     map[string]chan wsResponse
	getRequestID WsRequestIDFunc // returns id value (any type) and true if set

	authFn          WsAuthFunc         // exchange-specific auth
	userDataHandler WsUserEventHandler // exchange-specific user data/event
	extractID       WsExtractIDFunc
	extractErr      WsExtractErrFunc
	afterConnect    WsAfterConnectFunc
}

func NewWsClient(url string, lifeTime time.Duration, authFn WsAuthFunc,
	userdataHandler WsUserEventHandler,
	getRequestID WsRequestIDFunc,
	extractID WsExtractIDFunc,
	extractErr WsExtractErrFunc,
	afterConnect WsAfterConnectFunc,
) *WsClient {
	return &WsClient{
		url:             url,
		lifeTime:        lifeTime,
		authFn:          authFn,
		userDataHandler: userdataHandler,
		getRequestID:    getRequestID,
		extractID:       extractID,
		extractErr:      extractErr,
		afterConnect:    afterConnect,
	}
}

func (c *WsClient) Connect(ctx context.Context) (int64, error) {
	c.connected = true
	c.Ctx, c.cancel = context.WithCancel(ctx)

	ws, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return 0, err
	}
	c.wsMu = sync.Mutex{}
	c.wsMu.Lock()
	c.wsConn = &wsConnWrapper{ws}
	c.started = time.Now()
	c.wsMu.Unlock()

	var delta int64
	if c.authFn != nil {
		delta, err = c.authFn(ws)
		if err != nil {
			return 0, err
		}
	}

	c.requestsMu = sync.Mutex{}
	c.requestsMu.Lock()
	c.requests = make(map[string]chan wsResponse)
	c.requestsMu.Unlock()

	go c.wsMainHandler()
	go c.pingPongHandler()
	go c.sessionLifetimeWatcher()

	if c.afterConnect != nil {
		if err := c.afterConnect(c); err != nil {
			return 0, err
		}
	}

	return delta, nil
}

func (c *WsClient) Reconnect() {
	c.Close()
	time.Sleep(3 * time.Second)
	c.Connect(c.Ctx)
}

func (c *WsClient) Close() error {
	c.wsMu.Lock()
	defer c.wsMu.Unlock()
	if c.wsConn != nil {
		c.wsConn.Close()
		c.wsConn = nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func (c *WsClient) SendRequest(req map[string]interface{}) (map[string]json.RawMessage, error) {
	if !c.connected {
		return nil, fmt.Errorf("WebSocket is not connected.")
	}
	var id interface{}
	var ok bool

	if c.getRequestID != nil {
		id, ok = c.getRequestID(req)
	}
	if !ok {
		return nil, fmt.Errorf("WsClient: failed to set request ID (missing or IDGenFunc not set)")
	}

	idStr := fmt.Sprint(id) // used for map key (channel lookup)
	respCh := make(chan wsResponse, 1)
	c.requestsMu.Lock()
	c.requests[idStr] = respCh
	c.requestsMu.Unlock()

	c.wsMu.Lock()
	err := c.wsConn.WriteJSON(req)
	c.wsMu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case resp := <-respCh:
		if resp.Err != nil {
			return nil, resp.Err
		}
		return resp.Root, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for WS response")
	}
}

func (c *WsClient) wsMainHandler() {
	for {
		select {
		case <-c.Ctx.Done():
			return
		default:
		}
		_, msg, err := c.wsConn.ReadMessage()
		if err != nil {
			fmt.Printf("[wsclient] WS read error: %v\n", err)
			c.Reconnect()
			return
		}
		var root map[string]json.RawMessage
		if err := json.Unmarshal(msg, &root); err != nil {
			fmt.Printf("[wsclient] WS json unmarshal error: %v\n", err)
			continue
		}

		if c.extractID != nil {
			if id, found := c.extractID(root); found && id != "" {
				c.requestsMu.Lock()
				ch, ok := c.requests[id]
				if ok {
					delete(c.requests, id)
				}
				c.requestsMu.Unlock()
				if ok && ch != nil {
					err := c.extractErr(root)
					if err != nil {
						ch <- wsResponse{
							Root: root,
							Err:  err,
						}
					} else {
						ch <- wsResponse{
							Root: root,
						}
					}
				}
				continue
			}
		}
		// Let the user handler deal with user-data/events
		if c.userDataHandler != nil {
			c.userDataHandler(msg)
		}
	}
}

func (c *WsClient) pingPongHandler() {
	c.wsMu.Lock()
	c.wsConn.Conn.SetPingHandler(func(pingData string) error {
		return c.wsConn.Conn.WriteControl(
			websocket.PongMessage,
			[]byte(pingData),
			time.Now().Add(10*time.Second),
		)
	})
	c.wsMu.Unlock()
}

func (c *WsClient) sessionLifetimeWatcher() {
	timer := time.NewTimer(c.lifeTime)
	defer timer.Stop()
	select {
	case <-c.Ctx.Done():
		return
	case <-timer.C:
		fmt.Println("[wsclient] WS session lifetime reached, reconnecting...")
		c.Reconnect()
	}
}
