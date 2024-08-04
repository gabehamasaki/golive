package socket

import (
	"encoding/json"
	"fmt"
	"net"
)

type Server struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
}

type Message struct {
	Sender   string                  `json:"sender,omitempty"`
	Channel  string                  `json:"roomID,omitempty"`
	Payload  *map[string]interface{} `json:"payload,omitempty"`
	ServerIP string                  `json:"server_ip,omitempty"`
	SenderIP string                  `json:"sender_ip,omitempty"`
}

func NewServer() *Server {
	return &Server{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan []byte, 5),
		Register:   make(chan *Client, 5),
		Unregister: make(chan *Client, 5),
	}
}

func (server *Server) Start() {
	for {
		select {
		case conn := <-server.Register:
			// Register new client
			server.Clients[conn] = true
			fmt.Printf("Client %s connected in channel %s\n", conn.Id, conn.Channel)

			// Parse message JSON to byte
			jsonMessage, _ := json.Marshal(&Message{
				Sender:  "Server",
				Channel: conn.Channel,
				Payload: &map[string]interface{}{
					"message":   fmt.Sprintf("Client %s connected", conn.Id),
					"client_id": conn.Id,
				},
				ServerIP: LocalIp(),
				SenderIP: conn.Conn.LocalAddr().String(),
			})

			// Send message to all clients
			server.Send(jsonMessage, nil)

		case conn := <-server.Unregister:
			if _, ok := server.Clients[conn]; ok {
				close(conn.Send)
				delete(server.Clients, conn)
				fmt.Printf("Client %s disconnected\n", conn.Id)

				jsonMessage, _ := json.Marshal(&Message{
					Sender:   "Server",
					Channel:  conn.Channel,
					Payload:  &map[string]interface{}{"message": fmt.Sprintf("Client %s disconnected", conn.Id)},
					ServerIP: LocalIp(),
					SenderIP: conn.Conn.LocalAddr().String(),
				})

				server.Send(jsonMessage, conn)
			}

		case message := <-server.Broadcast:
			unparsedMessage := &Message{}
			_ = json.Unmarshal(message, unparsedMessage)
			for conn := range server.Clients {
				if conn.Channel == unparsedMessage.Channel {
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
