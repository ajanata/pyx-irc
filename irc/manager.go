/**
 * Copyright (c) 2018, Andy Janata
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without modification, are permitted
 * provided that the following conditions are met:
 *
 * * Redistributions of source code must retain the above copyright notice, this list of conditions
 *   and the following disclaimer.
 * * Redistributions in binary form must reproduce the above copyright notice, this list of
 *   conditions and the following disclaimer in the documentation and/or other materials provided
 *   with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR
 * IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
 * FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR
 * CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
 * WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY
 * WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package irc

import (
	"net"
)

type Manager struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
}

func NewManager(listener net.Listener, config *Config) {
	manager := Manager{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	go manager.listenForConnections()

	for {
		connection, error := listener.Accept()
		if error != nil {
			log.Error(error)
			return
		}
		client := NewClient(connection, config)
		manager.register <- client
		go manager.receive(client)
		go manager.send(client)
		go manager.close(client)
	}
}

func (manager *Manager) listenForConnections() {
	for {
		select {
		case client := <-manager.register:
			manager.clients[client] = true
			log.Infof("Received new connection from %s", client.socket.RemoteAddr())
		case client := <-manager.unregister:
			if _, ok := manager.clients[client]; ok {
				log.Infof("Closed connection for %s", client.socket.RemoteAddr())
				close(client.data)
				close(client.close)
				delete(manager.clients, client)
			}
		}
	}
}

func (manager *Manager) receive(client *Client) {
	for {
		if !client.reader.Scan() {
			log.Debugf("Unable to read from client %s, closing connection.",
				client.socket.RemoteAddr())
			manager.unregister <- client
			client.socket.Close()
			return
		}
		message := client.reader.Text()
		if len(message) > 0 {
			log.Debug("Received: " + message)
			client.handleIncoming(message)
		}
	}
}

func (manager *Manager) send(client *Client) {
	defer client.socket.Close()
	for {
		select {
		case message, ok := <-client.data:
			if !ok {
				log.Debugf("Unable to read from send channel for client %s, stopping goroutine.",
					client.socket.RemoteAddr())
				return
			}
			log.Debugf("Sending to %s: %s", client.socket.RemoteAddr(), message)
			_, error := client.writer.WriteString(message + "\r\n")
			if error != nil {
				log.Error(error)
			}
			error = client.writer.Flush()
			if error != nil {
				log.Error(error)
			}
		}
	}
}

func (manager *Manager) close(client *Client) {
	for {
		close, ok := <-client.close
		if close || !ok {
			log.Infof("Close requested for client %s (auto: %s)", client.socket.RemoteAddr(), !ok)
			manager.unregister <- client
			client.socket.Close()
			return
		}
	}
}
