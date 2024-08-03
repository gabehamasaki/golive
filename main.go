package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	WsServer *Server = NewServer()
)

func main() {
	router := gin.Default()

	go WsServer.Start()

	router.GET("/ws", func(c *gin.Context) {

		chat_id := c.Query("chat_id")
		if chat_id == "" {
			c.JSON(400, gin.H{"error": "chat_id is required"})
			return
		}
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			fmt.Println(err)
			return
		}

		client := &Client{
			Id:     uuid.Must(uuid.New(), nil).String(),
			Conn:   conn,
			RoomID: chat_id,
			Send:   make(chan []byte),
		}

		WsServer.Register <- client

		go client.Read()
		go client.Write()
	})

	router.Run(":8080")
}

type Client struct {
	Id     string
	Conn   *websocket.Conn
	RoomID string
	Send   chan []byte
}

type Server struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
}

type Message struct {
	Sender   string `json:"sender,omitempty"`
	RoomID   string `json:"roomID,omitempty"`
	Content  string `json:"content,omitempty"`
	ServerIP string `json:"server_ip,omitempty"`
	SenderIP string `json:"sender_ip,omitempty"`
}

func NewServer() *Server {
	return &Server{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (server *Server) Start() {
	for {
		select {
		case conn := <-server.Register:
			// Register new client
			server.Clients[conn] = true
			fmt.Printf("Client %s connected\n", conn.Id)

			// Parse message JSON to byte
			jsonMessage, _ := json.Marshal(&Message{
				Content:  "New user connected",
				ServerIP: LocalIp(),
				SenderIP: conn.Conn.LocalAddr().String(),
			})

			// Send message to all clients
			server.Send(jsonMessage, conn)

		case conn := <-server.Unregister:
			if _, ok := server.Clients[conn]; ok {
				close(conn.Send)
				delete(server.Clients, conn)
				fmt.Printf("Client %s disconnected\n", conn.Id)

				jsonMessage, _ := json.Marshal(&Message{
					Content:  "User disconnected",
					ServerIP: LocalIp(),
					SenderIP: conn.Conn.LocalAddr().String(),
				})

				server.Send(jsonMessage, conn)
			}

		case message := <-server.Broadcast:
			unparsedMessage := &Message{}
			_ = json.Unmarshal(message, unparsedMessage)
			for conn := range server.Clients {
				if conn.RoomID == unparsedMessage.RoomID {
					conn.Send <- message
				}
			}
		}
	}
}

func (server *Server) Send(message []byte, ignore *Client) {
	for conn := range server.Clients {
		if conn != ignore {
			conn.Send <- message
		}
	}
}

func LocalIp() string {
	address, _ := net.InterfaceAddrs()
	var ip = "localhost"
	for _, addr := range address {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
			}
		}
	}
	return ip
}

func (c *Client) Write() {
	defer func() {
		_ = c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			_ = c.Conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func (c *Client) Read() {
	defer func() {
		WsServer.Unregister <- c
		_ = c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			WsServer.Unregister <- c
			_ = c.Conn.Close()
			break
		}

		// Parse message JSON to byte
		jsonMessage, _ := json.Marshal(&Message{
			Content:  string(message),
			RoomID:   c.RoomID,
			ServerIP: LocalIp(),
			SenderIP: c.Conn.LocalAddr().String(),
		})

		WsServer.Broadcast <- jsonMessage
	}
}
