package client

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

type messagePayload struct {
	Type         string `json:"type,omitempty"`
	ID           string `json:"id"`
	Recipient    string `json:"recipient"`
	Body         string `json:"body,omitempty"`
	MsgID        string `json:"msg_id,omitempty"`
	PublicKey    string `json:"public_key,omitempty"`
	EncryptedKey string `json:"encrypted_key,omitempty"`
}

type sentMsg struct {
	Text      string
	Timestamp time.Time
	Status    string // "sent", "delivered", "read"
}

var (
	printMu       sync.Mutex
	timeFormat    = "15:04"
	meColor       = color.New(color.FgGreen).SprintFunc()
	incomingColor = color.New(color.FgCyan).SprintFunc()
	sysColor      = color.New(color.FgYellow).SprintFunc()
	errColor      = color.New(color.FgRed).SprintFunc()
	statusIcon    = map[string]string{
		"sent":      "✅",
		"delivered": "📬",
		"read":      "🟢",
	}
)

func printPrompt() {
	printMu.Lock()
	fmt.Print("[You]: ")
	printMu.Unlock()
}

func printIncoming(sender, msg, key string) {
	printMu.Lock()
	if key != "" {
		if kb, err := hex.DecodeString(key); err == nil {
			decrypted, err := Decrypt(kb, msg)
			if err == nil {
				msg = decrypted
			} else {
				printError(fmt.Sprintf("decrypt error: %v", err))
			}
		} else {
			printError(fmt.Sprintf("key decode error: %v", err))
		}
	}
	fmt.Print("\r")
	fmt.Printf("%s %s %s\n", color.HiBlackString(time.Now().Format(timeFormat)), incomingColor(sender+":"), msg)
	printMu.Unlock()
	printPrompt()
}

func printSystem(msg string) {
	printMu.Lock()
	fmt.Print("\r")
	fmt.Println(sysColor("ℹ️ " + msg))
	printMu.Unlock()
	printPrompt()
}

func printError(msg string) {
	printMu.Lock()
	fmt.Print("\r")
	fmt.Println(errColor("❌ " + msg))
	printMu.Unlock()
	printPrompt()
}

func (msg sentMsg) printSent() {
	printMu.Lock()
	icon := statusIcon[msg.Status]
	if icon == "" {
		icon = "…"
	}
	fmt.Print("\r")
	fmt.Printf("%s %s %s %s\n",
		color.HiBlackString(msg.Timestamp.Format(timeFormat)),
		meColor("You:"),
		msg.Text,
		icon,
	)
	printMu.Unlock()
	printPrompt()
}

func SendAndReceive(rawURL string, initialMsg string, id string, recipient string) error {
	var mu sync.Mutex
	sentMessages := make(map[string]*sentMsg)

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	q := u.Query()
	q.Set("id", id)
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	defer conn.Close()

	printSystem(fmt.Sprintf("Connected as %s. Type /quit to exit.", meColor(id)))

	sendPayload := func(body, typ, msgID, to, key string) error {
		payload := messagePayload{Type: typ, ID: id, Body: body, Recipient: to, MsgID: msgID, EncryptedKey: key}
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal error: %w", err)
		}
		return conn.WriteMessage(websocket.TextMessage, b)
	}

	// send a message
	sendBody := func(body, typ, msgID, key string) error {
		kb, err := hex.DecodeString(key)
		if err != nil {
			return fmt.Errorf("key decode error: %w", err)
		}
		ciphertext, err := Encrypt(kb, []byte(body))
		if err != nil {
			return err
		}
		return sendPayload(ciphertext, typ, msgID, recipient, key)
	}

	// if initial message
	if initialMsg != "" {
		msgID := fmt.Sprintf("%d", time.Now().UnixNano())
		key := "Hi"
		t := time.Now()
		_ = sendBody(initialMsg, "keyexg", msgID, key)
		mu.Lock()
		sentMessages[msgID] = &sentMsg{Text: initialMsg, Timestamp: t, Status: "sent"}
		mu.Unlock()
		sentMessages[msgID].printSent()
	}

	// read loop
	go func() {
		for {
			_, m, err := conn.ReadMessage()
			if err != nil {
				printError(fmt.Sprintf("read error: %v", err))
				return
			}
			var payload messagePayload
			if err := json.Unmarshal(m, &payload); err != nil {
				printIncoming("Server", string(m), "")
				continue
			}
			if payload.ID == id && payload.Type != "ack" {
				continue
			}
			if payload.Recipient != "" && payload.Recipient != id {
				continue
			}

			switch payload.Type {
			case "ack":
				if payload.MsgID != "" {
					mu.Lock()
					if msg, ok := sentMessages[payload.MsgID]; ok {
						msg.Status = payload.Body
						msg.printSent()
					}
					mu.Unlock()
				}
			default:
				printIncoming(payload.ID, payload.Body, payload.EncryptedKey)
				_ = sendPayload("delivered", "ack", payload.MsgID, payload.ID, "")
				// simulate read after receiving
				go func(mid string, sender string) {
					time.Sleep(1 * time.Second)
					_ = sendPayload("read", "ack", mid, sender, "")
				}(payload.MsgID, payload.ID)
			}
		}
	}()

	// write loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		printPrompt()
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if text == "/quit" {
			printSystem("Goodbye 👋")
			break
		}

		msgID := fmt.Sprintf("%d", time.Now().UnixNano())
		t := time.Now()
		kb := make([]byte, 32)
		if _, err := rand.Read(kb); err != nil {
			printError(fmt.Sprintf("key gen error: %v", err))
			continue
		}
		keyHex := hex.EncodeToString(kb)
		if err := sendBody(text, "msg", msgID, keyHex); err != nil {
			printError(fmt.Sprintf("write error: %v", err))
			break
		}

		mu.Lock()
		sentMessages[msgID] = &sentMsg{Text: text, Timestamp: t, Status: "sent"}
		m := *sentMessages[msgID]
		mu.Unlock()

		m.printSent()
	}

	return scanner.Err()
}

func Listen(url string) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	defer conn.Close()

	fmt.Println("📡 Connected. Waiting for messages...")

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
		fmt.Println("📨 New message:", string(msg))
	}
}

var ErrIDTaken = fmt.Errorf("id already taken")

func Register(registerURL, id string) error {
	body := map[string]string{"id": id}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	resp, err := http.Post(registerURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("post error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusConflict {
		return ErrIDTaken
	}
	return fmt.Errorf("register failed: %s", resp.Status)
}
