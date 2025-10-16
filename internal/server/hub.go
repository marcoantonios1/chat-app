package server

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID   string
	Conn *websocket.Conn
	Send chan []byte
}

type Hub struct {
	clients     map[*Client]bool
	byID        map[string]*Client
	register    chan *Client
	unregister  chan *Client
	broadcast   chan []byte
	targeted    chan targetedMessage
	undelivered map[string][][]byte
	shutdown    chan struct{}
}

type targetedMessage struct {
	to   string
	msg  []byte
	from string
}

var hub = &Hub{
	clients:     make(map[*Client]bool),
	byID:        make(map[string]*Client),
	register:    make(chan *Client),
	unregister:  make(chan *Client),
	broadcast:   make(chan []byte),
	targeted:    make(chan targetedMessage),
	undelivered: make(map[string][][]byte),
	shutdown:    make(chan struct{}),
}

func RunHub() {
	log.Println("hub: started")
	for {
		select {
		case c := <-hub.register:
			if c.ID != "" {
				if existing, ok := hub.byID[c.ID]; ok {
					log.Printf("hub: replacing existing client for id=%s (closing old conn=%p)\n", c.ID, existing)
					_ = existing.Conn.Close()
					delete(hub.clients, existing)
					delete(hub.byID, c.ID)
				}
			}

			hub.clients[c] = true
			if c.ID != "" {
				hub.byID[c.ID] = c
				log.Printf("hub: registered id=%s client=%p\n", c.ID, c)
				// deliver queued messages (if any)
				if queued, ok := hub.undelivered[c.ID]; ok {
					for _, m := range queued {
						select {
						case c.Send <- m:
						default:
							log.Printf("hub: unable to deliver queued msg to id=%s (client busy)", c.ID)
						}
					}
					delete(hub.undelivered, c.ID)
				}
			} else {
				log.Printf("hub: registered anonymous client=%p\n", c)
			}
		case c := <-hub.unregister:
			delete(hub.clients, c)
			if c.ID != "" {
				delete(hub.byID, c.ID)
				log.Printf("hub: unregistered id=%s client=%p\n", c.ID, c)
			} else {
				log.Printf("hub: unregistered anonymous client=%p\n", c)
			}
			close(c.Send)
		case msg := <-hub.broadcast:
			log.Printf("hub: broadcast msg(len=%d)\n", len(msg))
			for c := range hub.clients {
				select {
				case c.Send <- msg:
				default:
					// drop or cleanup slow client
					log.Printf("hub: dropping slow client=%p id=%s\n", c, c.ID)
					close(c.Send)
					delete(hub.clients, c)
					if c.ID != "" {
						delete(hub.byID, c.ID)
					}
				}
			}
		case t := <-hub.targeted:
			if dest, ok := hub.byID[t.to]; ok {
				// deliver to recipient
				select {
				case dest.Send <- t.msg:
					log.Printf("hub: targeted delivered to id=%s\n", t.to)
					// send ack to sender if present
					if t.from != "" {
						if sender, ok := hub.byID[t.from]; ok {
							ack := messagePayload{Type: "ack", Recipient: t.to, Body: "delivered"}
							if b, err := jsonMarshal(ack); err == nil {
								select {
								case sender.Send <- b:
								default:
								}
							}
						}
					}
				default:
					log.Printf("hub: target busy id=%s, queueing msg\n", t.to)
					// queue for recipient
					hub.undelivered[t.to] = append(hub.undelivered[t.to], t.msg)
					// ack as queued to sender
					if t.from != "" {
						if sender, ok := hub.byID[t.from]; ok {
							ack := messagePayload{Type: "ack", Recipient: t.to, Body: "queued"}
							if b, err := jsonMarshal(ack); err == nil {
								select {
								case sender.Send <- b:
								default:
								}
							}
						}
					}
				}
			} else {
				// recipient offline — queue message
				log.Printf("hub: target not found id=%s, queuing\n", t.to)
				hub.undelivered[t.to] = append(hub.undelivered[t.to], t.msg)
				// ack queued to sender if possible
				if t.from != "" {
					if sender, ok := hub.byID[t.from]; ok {
						ack := messagePayload{Type: "ack", Recipient: t.to, Body: "queued"}
						if b, err := jsonMarshal(ack); err == nil {
							select {
							case sender.Send <- b:
							default:
							}
						}
					}
				}
			}
		case <-hub.shutdown:
			log.Println("hub: shutdown initiated")
			// close all client connections and send channels
			for c := range hub.clients {
				_ = c.Conn.Close()
				close(c.Send)
				delete(hub.clients, c)
				if c.ID != "" {
					delete(hub.byID, c.ID)
				}
			}
			// exit RunHub
			log.Println("hub: stopped")
			return
		}
	}
}

// small helper to avoid importing encoding/json in this file twice
func jsonMarshal(v interface{}) ([]byte, error) {
	// avoid circular import by using encoding/json here
	type js interface{}
	return json.Marshal(v)
}

// ShutdownHub requests hub to stop (call from main/startup shutdown handler)
func ShutdownHub() {
	select {
	case <-hub.shutdown:
		// already closed
	default:
		close(hub.shutdown)
	}
}
