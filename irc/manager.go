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
	"bufio"
	"net"
)

type Manager struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
}

func NewManager(listener net.Listener) {
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
		client := &Client{
			socket: connection,
			reader: bufio.NewScanner(connection),
			writer: bufio.NewWriter(connection),
			data:   make(chan string),
		}
		manager.register <- client
		go manager.receive(client)
		go manager.send(client)
	}
}

func (manager *Manager) listenForConnections() {
	for {
		select {
		case connection := <-manager.register:
			manager.clients[connection] = true
			log.Info("Received new connection from " + connection.socket.RemoteAddr().String())
		case connection := <-manager.unregister:
			if _, ok := manager.clients[connection]; ok {
				log.Info("Closed connection for " + connection.socket.RemoteAddr().String())
				close(connection.data)
				delete(manager.clients, connection)
			}
		}
	}
}

func (manager *Manager) receive(client *Client) {
	for {
		if !client.reader.Scan() {
			manager.unregister <- client
			client.socket.Close()
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
				return
			}
			log.Debugf("Sending to %s: %s", client.socket.RemoteAddr().String(), message)
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
