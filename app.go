package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/liviosoares/go-watson-sdk/watson"
	"github.com/liviosoares/go-watson-sdk/watson/dialog"
)

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func findDialog(client dialog.Client, name string) string {
	dialogs, err := client.ListDialogs()
	if err != nil {
		fmt.Printf("Getting dialogs list failed %s", err)
		return ""
	}
	for _, d := range dialogs {
		if d.Name == name {
			return d.DialogId
		}
	}
	return ""
}

func main() {
	port := ":8080"

	if appEnv, err := cfenv.Current(); err != nil {
		if os.Getenv("PORT") != "" {
			port = fmt.Sprintf(":%s", os.Getenv("PORT"))
		}
	} else {
		port = fmt.Sprintf(":%d", appEnv.Port)
	}
	client, err := dialog.NewClient(watson.Config{})
	if err != nil {
		fmt.Printf("Dialog service auth failed %s", err)
		os.Exit(1)
	}

	dialogId := findDialog(client, "dialog")
	if dialogId == "" {
		file, err := os.Open("dialog.xml")
		if err != nil {
			fmt.Printf("Opening dialog.xml failed %s", err)
			os.Exit(1)
		}
		defer file.Close()

		_, err = client.CreateDialog("dialog", "dialog.xml", file)
		if err != nil {
			fmt.Printf("Creating dialog failed %s", err)
			os.Exit(1)
		}
		dialogId = findDialog(client, "dialog")
	}

	r := gin.Default()
	r.LoadHTMLGlob("./static/*.html")

	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})
	r.GET("/msg", func(c *gin.Context) {
		conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			fmt.Println("Failed to set websocket upgrade: %+v", err)
			return
		}
		conv, err := client.StartConversation(dialogId)
		if err != nil {
			fmt.Println("Failed to start conversation: %+v", err)
			return
		}
		for {
			if len(conv.Response) > 0 {
				conn.WriteMessage(websocket.TextMessage, []byte(strings.Join(conv.Response, "<br />")))
			}
			_, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Failed to read message")
				break
			}
			conv, err = client.UpdateConversation(dialogId, conv.ConversationId, conv.ClientId, string(msg))
			if err != nil {
				fmt.Println("Failed to update conversation: %+v", err)
				break
			}
		}

	})
	r.Static("/static", "./static")
	r.Run(port)
}
