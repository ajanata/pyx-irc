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
	"fmt"
	"github.com/ajanata/pyx-irc/pyx"
	"net"
	"regexp"
)

var validNickRegex = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]{2,29}$")

// it'd probably be better if this didn't talk directly to the pyx stuff from here...
type Client struct {
	socket     net.Conn
	addr       string
	reader     *bufio.Scanner
	writer     *bufio.Writer
	data       chan string
	close      chan bool
	registered bool
	password   string
	nick       string
	hasUser    bool
	pyx        *pyx.Client
	config     *Config
	n          *numerics
	gameId     *int
	// if we are spectating the game we are in
	gameIsSpectate bool
	// the host of the game we are in, so we can notice if they leave
	gameHost       string
	gameInProgress bool
	// the cards played in the most recently completed round
	gamePlayedCards *[][]pyx.WhiteCardData
}

type ChannelInfo struct {
	name       string
	totalUsers int
	topic      string
}

func NewClient(connection net.Conn, config *Config) *Client {
	addr, _, _ := net.SplitHostPort(connection.RemoteAddr().String())
	return &Client{
		socket: connection,
		addr:   addr,
		reader: bufio.NewScanner(connection),
		writer: bufio.NewWriter(connection),
		data:   make(chan string),
		close:  make(chan bool),
		config: config,
		n:      newNumerics(config),
	}
}

func (client *Client) handleIncoming(raw string) {
	msg := NewMessage(raw)
	if !client.registered {
		client.handleIncomingUnregistered(msg)
	} else {
		client.handleIncomingRegistered(msg)
	}
}

func (client *Client) handleIncomingUnregistered(msg Message) {
	handler, ok := UnregisteredHandlers[msg.cmd]
	if !ok {
		client.data <- client.n.formatSimpleReply(ErrNotRegistered, msg.cmd,
			"You have not registered")
	} else {
		handler(client, msg)
		if client.nick != "" && client.hasUser {
			log.Debugf("Client %s has fully registered as %s", client.socket.RemoteAddr(),
				client.nick)
			err := client.logInToPyx()
			if err != nil {
				log.Errorf("Unable to log in to PYX for %s: %v", client.nick, err)
				client.disconnect(err.Error())
			} else {
				client.registered = true
				client.sendWelcome()
			}
		}
	}
}

func (client *Client) logInToPyx() error {
	log.Debugf("Attempting to log into PYX for %s", client.nick)
	pyxClient, err := pyx.NewClient(client.nick, client.password, &client.config.Pyx)
	if err != nil {
		return err
	}

	client.pyx = pyxClient
	go client.dispatchPyxEvents()
	log.Infof("Logged in to PYX for %s", client.nick)
	return nil
}

func (client *Client) handleIncomingRegistered(msg Message) {
	handler, ok := RegisteredHandlers[msg.cmd]
	if !ok {
		client.data <- client.n.formatSimpleReply(ErrUnknownCommand, msg.cmd, "Unknown command")
	} else {
		handler(client, msg)
	}
}

func (client *Client) dispatchPyxEvents() {
	defer func() {
		// this is dumb and really should be refactored to avoid
		// this is also really bad cuz it'll eat segfaults
		if r := recover(); r != nil {
			log.Warningf("Recovered from panic, probably due to user quitting: %v", r)
		}
	}()
	for {
		event, ok := <-client.pyx.IncomingEvents
		if !ok {
			log.Infof("PYX event channel closed for %s", client.nick)
			client.disconnect("Disconnected from PYX.")
			return
		}

		handler, ok := EventHandlers[event.Event]
		if !ok {
			client.data <- fmt.Sprintf(":%s PRIVMSG %s :%+v", client.botNickUserAtHost(),
				client.nick, event)
		} else {
			handler(client, *event)
		}
	}
}
