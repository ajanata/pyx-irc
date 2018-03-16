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
	"strings"
	"time"
)

var validNickRegex = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]{2,29}$")

const GlobalChannel = "#global"
const BotNick = "Xyzzy"

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
}

type HandlerFunc func(*Client, Message)

var UnregisteredHandlers = map[string]HandlerFunc{
	"CAP":  handleCap,
	"NICK": handleUnregisteredNick,
	"PASS": handleUnregisteredPass,
	"USER": handleUnregisteredUser,
}
var RegisteredHandlers = map[string]HandlerFunc{
	"CAP":   handleCap,
	"MODE":  handleMode,
	"MOTD":  handleMotd,
	"NAMES": handleNames,
	"NICK":  handleRegisteredNick,
	"PASS":  handleRegisteredPassOrUser,
	"PING":  handlePing,
	"QUIT":  handleQuit,
	"TOPIC": handleTopic,
	"USER":  handleRegisteredPassOrUser,
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
	pyxClient, err := pyx.NewClient(client.nick, client.password)
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
		client.data <- formatSimpleReply(ErrUnknownCommand, msg.cmd, "Unknown command")
	} else {
		handler(client, msg)
	}
}

func handleCap(client *Client, msg Message) {
	// we don't support capabilities at all right now
	// we do this explicitly instead of the default handler since that replies 451 not registered
	client.data <- formatSimpleReply(ErrUnknownCommand, msg.cmd, "Unsupported command")
}

func handleUnregisteredNick(client *Client, msg Message) {
	if len(msg.args) < 1 {
		client.data <- formatSimpleReply(ErrNoNicknameGiven, msg.cmd, "No nickname given")
	} else {
		// TODO talk to pyx anyway so we can get the error message it gives?
		if validNickRegex.MatchString(msg.args[0]) {
			client.nick = msg.args[0]
			// TODO talk to pyx to verify it?
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
		// FIXME pyx has a length requirement on this, we probably should check it here and report
		// the error now instead of after the nick/pass combination
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

func (client *Client) disconnect(why string) {
	s := fmt.Sprintf("ERROR :Closing Link: %s[%s] (%s)", client.nick, client.addr, why)
	// have to do this differently to ensure the client actually gets this before we close the
	// connection
	client.writer.WriteString(s + "\r\n")
	client.writer.Flush()

	client.close <- true

	if client.pyx != nil {
		client.pyx.LogOut()
	}
}

func handleQuit(client *Client, msg Message) {
	client.disconnect(fmt.Sprintf("Quit: %s", client.nick))
}

func (client *Client) sendWelcome() {
	client.data <- formatFmt(RplWelcome, client.nick, ":Welcome to the PYX IRC network %s!%s@%s",
		client.nick, client.nick, client.addr)
	// TODO version in both of these
	client.data <- formatFmt(RplYourHost, client.nick, ":Your host is %s, running version TODO",
		MyServerName)
	// user modes, channel modes
	client.data <- formatFmt(RplMyInfo, client.nick, "%s TODO or lvontk", MyServerName)
	client.data <- formatFmt(RplISupport, client.nick, "MAXCHANNELS=2 CHANLIMIT=#:2 NICKLEN=30 "+
		"CHANNELLEN=9 TOPICLEN=307 AWAYLEN=0 MAXTARGETS=1 MODES=1 CHANTYPES=# PREFIX=(aov)&@+ "+
		"CHANMODES=,k,l,voantk NETWORK=PYX CASEMAPPING=ascii :are supported by this server")

	client.sendLUser()
	handleMotd(client, Message{})

	// this is NOT the same as just handleModeImpl: We are explicitly setting the mode
	modes := "+"
	if client.pyx.User.IsAdmin() {
		modes = modes + "o"
	}
	if len(client.pyx.User.IdCode) > 0 {
		modes = modes + "r"
	}
	if "+" != modes {
		client.data <- fmt.Sprintf(":%s MODE %s :%s", client.nick, client.nick, modes)
	}

	client.joinChannel(GlobalChannel)
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

func (client *Client) joinChannel(channel string) error {
	if channel != GlobalChannel {
		// TODO actually join the game on pyx
	}
	client.data <- fmt.Sprintf(":%s JOIN :%s", client.nick, channel)

	handleTopicImpl(client, channel)

	handleNamesImpl(client, channel)

	// unreal doesn't shove these down automatically but... why not
	handleModeImpl(client, channel)

	client.data <- fmt.Sprintf(":%s PRIVMSG %s :hello!", BotNick, channel)

	return nil
}

func handleNames(client *Client, msg Message) {
	handleNamesImpl(client, msg.args...)
}

func handleNamesImpl(client *Client, args ...string) {
	if len(args) == 0 {
		client.data <- formatFmt(ErrNeedMoreParams, client.nick, "NAMES :Not enough parameters")
		return
	}

	if args[0] == GlobalChannel {
		names, err := client.pyx.GetNames()
		if err != nil {
			log.Errorf("Unable to retrieve names for %s: %v", args[0], err)
		}
		// TODO a proper length based on 512 minus broilerplate
		for _, line := range joinIntoLines(300, append(names, "&"+BotNick)) {
			client.data <- formatFmt(RplNames, client.nick, "= %s :%s", args[0], line)
		}
		client.data <- formatFmt(RplEndNames, client.nick, "%s :End of /NAMES list", args[0])
	} else {
		// TODO per-game channels
	}
}

func handleTopic(client *Client, msg Message) {
	handleTopicImpl(client, msg.args...)
}

func handleTopicImpl(client *Client, args ...string) {
	if len(args) == 0 {
		// error to not specify channel
		client.data <- formatFmt(ErrNeedMoreParams, client.nick, "TOPIC :Not enough parameters")
	} else if len(args) == 1 {
		// show topic
		var topic string
		if args[0] == GlobalChannel {
			if client.pyx.GlobalChatEnabled {
				topic = "Global chat"
			} else {
				topic = "Global chat (disabled)"
			}
		} else {
			// TODO have to get the game info for the topic
			topic = "TODO"
		}
		client.data <- formatFmt(RplTopic, client.nick, "%s :%s", args[0], topic)
		// TODO something better here than "now"
		// TODO and for games, maybe the host should be setting the topic
		client.data <- formatFmt(RplTopicWhoTime, client.nick, "%s %s %d", args[0], BotNick,
			time.Now().Unix())
	} else {
		// error to try to change topic
		// TODO is there a better numeric for this? we don't want to let ANYONE change it like this
		client.data <- formatFmt(ErrChanOpPrivsNeeded, client.nick, "TOPIC :You can't do that.")
	}
}

func handleMode(client *Client, msg Message) {
	handleModeImpl(client, msg.args...)
}

func handleModeImpl(client *Client, args ...string) {
	// TODO handle if the user is trying to chage modes
	if len(args) == 0 {
		client.data <- formatFmt(ErrNeedMoreParams, client.nick, "MODE :Not enough parameters")
	} else if strings.HasPrefix(args[0], "#") {
		if len(args) == 1 {
			var modes string
			if args[0] == GlobalChannel {
				// TODO it'd be nice to know if join/quit is being broadcast, if they are we can +n too
				if client.pyx.GlobalChatEnabled {
					modes = "+t"
				} else {
					modes = "+mt"
				}
			} else {
				// TODO we need game info here too for limit and key and stuff
				modes = "+nt"
			}
			client.data <- formatFmt(RplChannelModeIs, client.nick, "%s %s", args[0], modes)
			// TODO better than "now"
			client.data <- formatFmt(RplCreationTime, client.nick, "%s %d", args[0],
				time.Now().Unix())
		} else {
			// TODO handle if the user is trying to chage modes
			// TODO but if they are the game host, we need to let them change some of the settings
			client.data <- formatFmt(ErrChanOpPrivsNeeded, client.nick, "MODE :You can't do that.")
		}
	} else if args[0] == client.nick {
		if len(args) == 1 {
			// show modes
			// default to no modes. this is how unreal reports it
			modes := "+"
			if client.pyx.User.IsAdmin() {
				modes = modes + "o"
			}
			if len(client.pyx.User.IdCode) > 0 {
				modes = modes + "r"
			}
			client.data <- formatFmt(RplUModeIs, client.nick, modes)
		} else {
			// error to change modes
			// but unreal doesn't reply _at all_ for bad mode changes
		}
	} else {
		// error to look at someone else's modes
		// but unreal doesn't reply _at all_ for this
	}
}

func handlePing(client *Client, msg Message) {
	arg := ""
	if len(msg.args) > 0 {
		arg = msg.args[0]
	}
	client.data <- fmt.Sprintf(":%s PONG %s :%s", MyServerName, MyServerName, arg)
}

// handle the PYX stuff coming in

func (client *Client) dispatchPyxEvents() {
	defer func() {
		// this is dumb and really should be refactored to avoid
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

		// TODO actually dispatch this somehow
		client.data <- fmt.Sprintf(":%s PRIVMSG %s :%v", BotNick, client.nick, event)
	}
}
