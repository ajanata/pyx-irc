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
	"regexp"
)

// FIXME
const MyServerName = "localhost"

var validNickRegex = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]{2,29}$")

type Client struct {
	socket     net.Conn
	reader     *bufio.Scanner
	writer     *bufio.Writer
	data       chan string
	registered bool
	password   string
	nick       string
	hasUser    bool
}

type HandlerFunc func(*Client, Message)

var UnregisteredHandlers = map[string]HandlerFunc{
	"NICK": handleUnregisteredNick,
	"USER": handleUnregisteredUser,
	"PASS": handleUnregisteredPass,
}
var RegisteredHandlers = map[string]HandlerFunc{}

func NewClient(connection net.Conn) *Client {
	return &Client{
		socket: connection,
		reader: bufio.NewScanner(connection),
		writer: bufio.NewWriter(connection),
		data:   make(chan string),
	}
}

func (client *Client) handleIncoming(raw string) {
	// TODO actual implementation, just echo it back for now
	msg := NewMessage(raw)
	if !client.registered {
		client.handleIncomingUnregistered(msg)
	}
	client.data <- raw
}

func (client *Client) handleIncomingUnregistered(msg Message) {
	handler, ok := UnregisteredHandlers[msg.cmd]
	if !ok {
		client.data <- formatSimpleError(MyServerName, ErrNotRegistered, msg.cmd, "You have not registered")
	} else {
		handler(client, msg)
		if client.nick != "" && client.hasUser {
			log.Debugf("Client %s has fully registered as %s", client.socket.RemoteAddr(), client.nick)
			client.registered = true
		}
	}
}

func handleUnregisteredNick(client *Client, msg Message) {
	if len(msg.args) < 1 {
		client.data <- formatSimpleError(MyServerName, ErrNeedMoreParams, msg.cmd, "Not enough parameters")
	} else {
		// TODO talk to pyx anyway so we can get the error message it gives?
		if validNickRegex.MatchString(msg.args[0]) {
			client.nick = msg.args[0]
			// TODO talk to pyx to verify it
		} else {
			client.data <- formatSimpleError(MyServerName, ErrErroneousNickname, msg.cmd, "Erroneous Nickname")
		}
	}
}

func handleUnregisteredPass(client *Client, msg Message) {
	if len(msg.args) < 1 {
		client.data <- formatSimpleError(MyServerName, ErrNeedMoreParams, msg.cmd, "Not enough parameters")
	} else {
		client.password = msg.args[0]
	}
}

func handleUnregisteredUser(client *Client, msg Message) {
	// we don't care about anything in this message, other than requiring it for flow
	client.hasUser = true
}
