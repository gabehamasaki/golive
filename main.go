package main

import (
	"fmt"
	"net/http"

	"github.com/gabrielhamasaki/golive/socket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	WsServer *socket.Server = socket.NewServer()
)

func main() {
	router := gin.Default()

	go WsServer.Start()

	router.GET("/ws/:channel", handleChannel)

	router.Run(":4321")
}

func handleChannel(c *gin.Context) {
	channel := c.Param("channel")
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	client := WsServer.NewClient(conn, channel)
	go client.Read()
	go client.Write()
}
