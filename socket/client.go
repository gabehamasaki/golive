package socket

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	Id     string
	Conn   *websocket.Conn
	RoomID string
	Send   chan []byte
	Server *Server
}

func (server *Server) NewClient(conn *websocket.Conn, roomID string) *Client {
	client := &Client{
		Id:     uuid.Must(uuid.New(), nil).String(),
		Conn:   conn,
		RoomID: roomID,
		Send:   make(chan []byte),
		Server: server,
	}

	server.Register <- client

	return client
}

func (c *Client) Write() {
	defer func() {
		_ = c.Conn.Close()
	}()

	for raw := range c.Send {
		message := &Message{}
		_ = json.Unmarshal(raw, &message)

		_ = c.Conn.WriteJSON(gin.H{
			"sender":   message.Sender,
			"serverIP": message.ServerIP,
			"senderIP": message.SenderIP,
			"payload":  message.Payload,
		})

		fmt.Printf("Sent message to %s in channel %s\n", c.Id, c.RoomID)
	}
}

func (c *Client) Read() {
	defer func() {
		c.Server.Unregister <- c
		_ = c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			c.Server.Unregister <- c
			_ = c.Conn.Close()
			break
		}

		var jsonMap map[string]interface{}
		_ = json.Unmarshal(message, &jsonMap)

		jsonMessage, _ := json.Marshal(&Message{
			Sender:   c.Id,
			RoomID:   c.RoomID,
			Payload:  &jsonMap,
			ServerIP: LocalIp(),
			SenderIP: c.Conn.LocalAddr().String(),
		})

		fmt.Printf("Received message from %s in channel %s\n", c.Id, c.RoomID)

		c.Server.Broadcast <- jsonMessage
	}
}
