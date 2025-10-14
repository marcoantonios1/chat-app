# 📡 Go Chat — Secure & Concurrent CLI Chat App

Go Chat is a lightweight, end-to-end encrypted command-line chat application built with Go.
It aims to demonstrate modern networking concepts including concurrency, gRPC, peer-to-peer communication, and secure encryption — all from the terminal.

## ✨ Features (planned & implemented)
-	🖥️ CLI server and client — no external dependencies, easy to run anywhere.
-	🔁 Concurrent connections using goroutines and channels.
-	🔐 End-to-end encryption (planned).
-	📡 gRPC streaming for structured messaging (planned).
-	🌍 Peer-to-peer mode with NAT traversal (planned).
-	🧰 Clean and scalable architecture.

## 🏗️ Project goals
-	Showcase strong backend and network programming skills in Go.
-	Build a practical example of real-time messaging using modern protocols.
-	Evolve step-by-step from a simple TCP chat to a secure, distributed system.

## 🚀 Getting Started

### build server and client
	go build -o chat-server ./cmd/server
	go build -o chat-client ./cmd/client

### run server
	./chat-server

### run client in another terminal
	./chat-client