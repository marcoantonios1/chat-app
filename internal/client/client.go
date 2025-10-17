package client

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/hkdf"
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
	peerKeysMu sync.RWMutex
	peerKeys   = make(map[string][]byte)

	peerPubMu sync.RWMutex
	peerPub   = make(map[string][]byte)
)

func printPrompt() {
	printMu.Lock()
	fmt.Print("[You]: ")
	printMu.Unlock()
}

func printIncoming(sender, msg, key string) {
	printMu.Lock()
	defer func() {
		printMu.Unlock()
		printPrompt()
	}()

	if key != "" {
		// try hex-decoded symmetric key first
		if kb, err := hex.DecodeString(key); err == nil {
			decrypted, err := Decrypt(kb, msg)
			if err == nil {
				msg = decrypted
			} else {
				printError(fmt.Sprintf("decrypt error: %v", err))
			}
		} else {
			// try base64 -> treat as KEM encapsulated ciphertext
			if ct, err2 := base64.StdEncoding.DecodeString(key); err2 == nil {
				// need our private key to decapsulate
				_, priv, err := LoadKeyPair()
				if err != nil || len(priv) == 0 {
					printError(fmt.Sprintf("no private key for decapsulation: %v", err))
				} else {
					shared, err := DecapsulateWithPriv(priv, ct)
					if err != nil {
						printError(fmt.Sprintf("decapsulate error: %v", err))
					} else {
						// derive AEAD key from shared secret via HKDF-SHA256
						h := hkdf.New(sha256.New, shared, nil, nil)
						derived := make([]byte, 32)
						if _, err := io.ReadFull(h, derived); err != nil {
							printError(fmt.Sprintf("hkdf error: %v", err))
						} else {
							// cache derived key for this sender
							peerKeysMu.Lock()
							peerKeys[sender] = append([]byte(nil), derived...)
							peerKeysMu.Unlock()

							// try decrypting with derived key
							if dec, err := Decrypt(derived, msg); err == nil {
								msg = dec
							} else {
								printError(fmt.Sprintf("decrypt-with-derived-key error: %v", err))
							}
						}
					}
				}
			} else {
				printError(fmt.Sprintf("key decode error: %v / %v", err, err2))
			}
		}
	}

	fmt.Print("\r")
	fmt.Printf("%s %s %s\n", color.HiBlackString(time.Now().Format(timeFormat)), incomingColor(sender+":"), msg)
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

func SendAndReceive(rawURL string, id string, recipient string) error {
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

	sendPayload := func(body, typ, msgID, to, encryptedKey, publicKey string) error {
		payload := messagePayload{Type: typ, ID: id, Body: body, Recipient: to, MsgID: msgID, EncryptedKey: encryptedKey, PublicKey: publicKey}
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal error: %w", err)
		}
		return conn.WriteMessage(websocket.TextMessage, b)
	}

	// send a message
	sendBody := func(body, typ, msgID, key string) error {
		// 1) if we already have a derived symmetric key for this peer, use it
		peerKeysMu.RLock()
		derived, hasDerived := peerKeys[recipient]
		peerKeysMu.RUnlock()
		if hasDerived && len(derived) > 0 {
			ciphertext, err := Encrypt(derived, []byte(body))
			if err != nil {
				return err
			}
			return sendPayload(ciphertext, typ, msgID, recipient, "", "")
		}

		// 2) if we have the peer's public key, encapsulate on-demand, cache derived key,
		//    send KEM ciphertext (base64) in EncryptedKey and send encrypted message

		peerPubMu.RLock()
		pubb, hasPub := peerPub[recipient]
		peerPubMu.RUnlock()
		if hasPub && len(pubb) > 0 {
			ctKEM, shared, err := EncapsulateWithPub(pubb)
			if err != nil {
				return fmt.Errorf("encapsulate error: %w", err)
			}
			// derive symmetric key (32 bytes) from KEM shared secret via HKDF-SHA256
			h := hkdf.New(sha256.New, shared, nil, nil)
			newDerived := make([]byte, 32)
			if _, err := io.ReadFull(h, newDerived); err != nil {
				return fmt.Errorf("hkdf derive error: %w", err)
			}
			// cache derived key
			peerKeysMu.Lock()
			peerKeys[recipient] = append([]byte(nil), newDerived...)
			peerKeysMu.Unlock()

			// send encap key to peer (base64) so they can decapsulate
			enc := base64.StdEncoding.EncodeToString(ctKEM)
			if err := sendPayload("", "encap_key", "", recipient, enc, ""); err != nil {
				return fmt.Errorf("send encap_key error: %w", err)
			}

			// encrypt and send actual message with derived key
			ciphertext, err := Encrypt(newDerived, []byte(body))
			if err != nil {
				return err
			}
			return sendPayload(ciphertext, typ, msgID, recipient, "", "")
		}

		// 3) fallback: use provided symmetric key hex (existing behavior)
		if key == "" {
			return fmt.Errorf("no key available and no peer public key to encapsulate")
		}
		kb, err := hex.DecodeString(key)
		if err != nil {
			return fmt.Errorf("key decode error: %w", err)
		}
		ciphertext, err := Encrypt(kb, []byte(body))
		if err != nil {
			return err
		}
		return sendPayload(ciphertext, typ, msgID, recipient, key, "")
	}

	pub, _, err := GetKeyPair()
	if err != nil || len(pub) == 0 {
		pub, priv, genErr := GenerateKyberKeyPair()
		if genErr != nil {
			printError(fmt.Sprintf("key gen error: %v", genErr))
			return genErr
		}
		if saveErr := SaveKeyPair(pub, priv); saveErr != nil {
			printError(fmt.Sprintf("key save error: %v", saveErr))
		}
	}

	b64Pub := base64.StdEncoding.EncodeToString(pub)
	pubMsg := messagePayload{Type: "pubkey", ID: id, Recipient: recipient, PublicKey: b64Pub}
	if b, err := json.Marshal(pubMsg); err == nil {
		if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
			printError(fmt.Sprintf("pubkey send error: %v", err))
		} else {
			printSystem("Public key sent to " + meColor(recipient))
		}
	} else {
		printError(fmt.Sprintf("pubkey marshal error: %v", err))
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

			case "pubkey":
				printSystem(fmt.Sprintf("Received public key from %s", meColor(payload.ID)))

				ctBytes, err := base64.StdEncoding.DecodeString(payload.EncryptedKey)
				if err != nil {
					printError(fmt.Sprintf("public key decode error: %v", err))
					break
				}

				peerPubMu.Lock()
				peerPub[payload.ID] = append([]byte(nil), ctBytes...)
				peerPubMu.Unlock()
				printSystem(fmt.Sprintf("Cached public key for %s", meColor(payload.ID)))

			case "encap_key":
				printSystem(fmt.Sprintf("Received encapsulated key from %s", meColor(payload.ID)))
				ctBytes, err := base64.StdEncoding.DecodeString(payload.PublicKey)
				if err != nil {
					printError(fmt.Sprintf("encap_key decode error from %s: %v", payload.ID, err))
					break
				}
				_, priv, err := LoadKeyPair()
				if err != nil || len(priv) == 0 {
					printError(fmt.Sprintf("no private key for decapsulation: %v", err))
					break
				}

				shared, err := DecapsulateWithPriv(priv, ctBytes)
				if err != nil {
					printError(fmt.Sprintf("decapsulate error from %s: %v", payload.ID, err))
					break
				}

				h := hkdf.New(sha256.New, shared, nil, nil)
				derived := make([]byte, 32)
				if _, err := io.ReadFull(h, derived); err != nil {
					printError(fmt.Sprintf("hkdf derive error from %s: %v", payload.ID, err))
					break
				}
				peerKeysMu.Lock()
				peerKeys[payload.ID] = append([]byte(nil), derived...)
				peerKeysMu.Unlock()
				printSystem(fmt.Sprintf("Established shared key with %s", meColor(payload.ID)))
			default:
				printIncoming(payload.ID, payload.Body, payload.EncryptedKey)
				_ = sendPayload("delivered", "ack", payload.MsgID, payload.ID, "", "")
				// simulate read after receiving
				go func(mid string, sender string) {
					time.Sleep(1 * time.Second)
					_ = sendPayload("read", "ack", mid, sender, "", "")
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
