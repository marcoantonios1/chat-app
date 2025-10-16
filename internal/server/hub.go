package server

import (
    "github.com/gorilla/websocket"
)

type Client struct {
    ID   string
    Conn *websocket.Conn
    Send chan []byte
}

type Hub struct {
    clients    map[*Client]bool
    byID       map[string]*Client
    register   chan *Client
    unregister chan *Client
    broadcast  chan []byte
    targeted   chan targetedMessage
}

type targetedMessage struct {
    to  string
    msg []byte
}

var hub = &Hub{
    clients:    make(map[*Client]bool),
    byID:       make(map[string]*Client),
    register:   make(chan *Client),
    unregister: make(chan *Client),
    broadcast:  make(chan []byte),
    targeted:   make(chan targetedMessage),
}

func RunHub() {
    for {
        select {
        case c := <-hub.register:
            hub.clients[c] = true
            if c.ID != "" { hub.byID[c.ID] = c }
        case c := <-hub.unregister:
            delete(hub.clients, c)
            if c.ID != "" { delete(hub.byID, c.ID) }
            close(c.Send)
        case msg := <-hub.broadcast:
            for c := range hub.clients {
                select {
                case c.Send <- msg:
                default:
                    // drop or cleanup slow client
                    close(c.Send)
                    delete(hub.clients, c)
                }
            }
        case t := <-hub.targeted:
            if c, ok := hub.byID[t.to]; ok {
                select {
                case c.Send <- t.msg:
                default:
                    // handle slow client
                }
            }
        }
    }
}