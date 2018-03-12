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
	"net"
	"regexp"
)

var validNickRegex = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]{2,29}$")

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
}

type HandlerFunc func(*Client, Message)

var UnregisteredHandlers = map[string]HandlerFunc{
	"NICK": handleUnregisteredNick,
	"USER": handleUnregisteredUser,
	"PASS": handleUnregisteredPass,
}
var RegisteredHandlers = map[string]HandlerFunc{
	"NICK": handleRegisteredNick,
	"USER": handleRegisteredPassOrUser,
	"PASS": handleRegisteredPassOrUser,
	"MOTD": handleMotd,
	"QUIT": handleQuit,
}

func NewClient(connection net.Conn) *Client {
	addr, _, _ := net.SplitHostPort(connection.RemoteAddr().String())
	return &Client{
		socket: connection,
		addr:   addr,
		reader: bufio.NewScanner(connection),
		writer: bufio.NewWriter(connection),
		data:   make(chan string),
		close:  make(chan bool),
	}
}

func (client *Client) handleIncoming(raw string) {
	// TODO actual implementation, just echo it back for now
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
		client.data <- formatSimpleReply(ErrNotRegistered, msg.cmd, "You have not registered")
	} else {
		handler(client, msg)
		if client.nick != "" && client.hasUser {
			log.Debugf("Client %s has fully registered as %s", client.socket.RemoteAddr(), client.nick)
			// TODO we definitely need to be talking to pyx by now
			client.registered = true
			client.sendWelcome()
		}
	}
}

func (client *Client) handleIncomingRegistered(msg Message) {
	handler, ok := RegisteredHandlers[msg.cmd]
	if !ok {
		client.data <- formatSimpleReply(ErrUnknownCommand, msg.cmd, "Unknown command")
	} else {
		handler(client, msg)
	}
}

func handleUnregisteredNick(client *Client, msg Message) {
	if len(msg.args) < 1 {
		client.data <- formatSimpleReply(ErrNeedMoreParams, msg.cmd, "Not enough parameters")
	} else {
		// TODO talk to pyx anyway so we can get the error message it gives?
		if validNickRegex.MatchString(msg.args[0]) {
			client.nick = msg.args[0]
			// TODO talk to pyx to verify it
		} else {
			client.data <- formatSimpleReply(ErrErroneousNickname, msg.cmd, "Erroneous Nickname")
		}
	}
}

func handleRegisteredNick(client *Client, msg Message) {
	client.data <- formatSimpleReply(ErrNoNickChange, msg.cmd, "Nickname change not supported.")
}

func handleUnregisteredPass(client *Client, msg Message) {
	if len(msg.args) < 1 {
		client.data <- formatSimpleReply(ErrNeedMoreParams, msg.cmd, "Not enough parameters")
	} else {
		client.password = msg.args[0]
	}
}

func handleRegisteredPassOrUser(client *Client, msg Message) {
	client.data <- formatSimpleReply(ErrAlreadyRegistered, msg.cmd, "Already registered")
}

func handleUnregisteredUser(client *Client, msg Message) {
	// we don't care about anything in this message, other than requiring it for flow
	client.hasUser = true
}

func handleMotd(client *Client, msg Message) {
	client.data <- formatSimpleReply(ErrNoMotd, client.nick, "No MOTD configured.")
}

func handleQuit(client *Client, msg Message) {
	s := fmt.Sprintf("ERROR :Closing Link: %s[%s] (Quit: %s)", client.nick, client.addr,
		client.nick)
	// have to do this differently to ensure the client actually gets this before we close the
	// connection
	client.writer.WriteString(s + "\r\n")
	client.writer.Flush()

	client.close <- true
}

func (client *Client) sendWelcome() {
	client.data <- formatFmt(RplWelcome, client.nick, ":Welcome to the PYX IRC network %s!%s@%s",
		client.nick, client.nick, client.addr)
	// TODO version in both of these
	client.data <- formatFmt(RplYourHost, client.nick, ":Your host is %s, running version TODO",
		MyServerName)
	// user modes, channel modes
	client.data <- formatFmt(RplMyInfo, client.nick, "%s TODO o lvontk", MyServerName)
	client.data <- formatFmt(RplISupport, client.nick, "MAXCHANNELS=2 CHANLIMIT=#:2 NICKLEN=30 "+
		"CHANNELLEN=9 TOPICLEN=307 AWAYLEN=0 MAXTARGETS=1 MODES=1 CHANTYPES=# PREFIX=(ov)@+ "+
		"CHANMODES=,k,l,vontk NETWORK=PYX CASEMAPPING=ascii :are supported by this server")

	client.sendLUser()
	handleMotd(client, Message{})
}

func (client *Client) sendLUser() {
	// TODO real counts
	// TODO maybe keep track of how many users are using the bridge and count them as "local"
	// and everyone else as "global"?
	client.data <- formatFmt(RplLUserClient, client.nick, ":There are %d users on 1 server", 1)
	client.data <- formatFmt(RplLUserOp, client.nick, "%d :operator(s) online", 0)
	client.data <- formatFmt(RplLUserChannels, client.nick, "%d :channels formed", 0)
	client.data <- formatFmt(RplLUserMe, client.nick, ":I have %d clients and %d servers", 1, 0)
	client.data <- formatFmt(RplLocalUsers, client.nick, ":Current Local Users: %d  Max: %d", 1, 1)
	client.data <- formatFmt(RplGlobalUsers, client.nick, ":Current Global Users: %d  Max: %d", 1,
		1)
}
