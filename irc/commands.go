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

// IRC command handlers

package irc

import (
	"fmt"
	"github.com/ajanata/pyx-irc/pyx"
	"github.com/ajanata/pyx-irc/util"
	"strconv"
	"strings"
)

type IrcHandlerFunc func(*Client, Message)

var UnregisteredHandlers = map[string]IrcHandlerFunc{
	"CAP":  handleCap,
	"NICK": handleUnregisteredNick,
	"PASS": handleUnregisteredPass,
	"USER": handleUnregisteredUser,
}
var RegisteredHandlers = map[string]IrcHandlerFunc{
	"CAP":     handleCap,
	"JOIN":    handleJoin,
	"LIST":    handleList,
	"LUSERS":  handleLUsers,
	"MODE":    handleMode,
	"MOTD":    handleMotd,
	"NAMES":   handleNames,
	"NICK":    handleRegisteredNick,
	"PART":    handlePart,
	"PASS":    handleRegisteredPassOrUser,
	"PING":    handlePing,
	"PRIVMSG": handlePrivmsg,
	"QUIT":    handleQuit,
	"TOPIC":   handleTopic,
	"USER":    handleRegisteredPassOrUser,
	"WHO":     handleWho,
	"WHOIS":   handleWhois,
	"WHOWAS":  handleWhowas,
}

func handleCap(client *Client, msg Message) {
	// we don't support capabilities at all right now
	// we do this explicitly instead of the default handler since that replies 451 not registered
	client.data <- client.n.formatSimpleReply(ErrUnknownCommand, msg.cmd, "Unsupported command")
}

func handleUnregisteredNick(client *Client, msg Message) {
	if len(msg.args) < 1 {
		client.data <- client.n.formatSimpleReply(ErrNoNicknameGiven, msg.cmd, "No nickname given")
	} else {
		// TODO talk to pyx anyway so we can get the error message it gives?
		if validNickRegex.MatchString(msg.args[0]) {
			client.nick = msg.args[0]
			// TODO talk to pyx to verify it?
		} else {
			client.data <- client.n.formatSimpleReply(ErrErroneousNickname, msg.cmd,
				"Erroneous Nickname")
		}
	}
}

func handleRegisteredNick(client *Client, msg Message) {
	client.data <- client.n.formatSimpleReply(ErrNoNickChange, msg.cmd,
		"Nickname change not supported.")
}

func handleUnregisteredPass(client *Client, msg Message) {
	if len(msg.args) < 1 {
		client.data <- client.n.formatSimpleReply(ErrNeedMoreParams, msg.cmd,
			"Not enough parameters")
	} else {
		// FIXME pyx has a length requirement on this, we probably should check it here and report
		// the error now instead of after the nick/pass combination
		client.password = msg.args[0]
	}
}

func handleRegisteredPassOrUser(client *Client, msg Message) {
	client.data <- client.n.formatSimpleReply(ErrAlreadyRegistered, msg.cmd, "Already registered")
}

func handleUnregisteredUser(client *Client, msg Message) {
	// we don't care about anything in this message, other than requiring it for flow
	client.hasUser = true
}

func handleMotd(client *Client, msg Message) {
	client.data <- client.n.formatSimpleReply(ErrNoMotd, client.nick, "No MOTD configured.")
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
	client.data <- client.n.format(RplWelcome, client.nick,
		":Welcome to the PYX IRC network %s!%s@%s", client.nick, client.nick, client.addr)
	client.data <- client.n.format(RplYourHost, client.nick,
		":Your host is %s, running version pyx-irc-%s-%s", client.config.AdvertisedName,
		util.GitBranch, util.GitSummary)
	// user modes, channel modes
	client.data <- client.n.format(RplMyInfo, client.nick, "%s pyx-irc-%s-%s Bor alvontk",
		client.config.AdvertisedName, util.GitBranch, util.GitSummary)
	client.data <- client.n.format(RplISupport, client.nick,
		"MAXCHANNELS=2 CHANLIMIT=#:2 NICKLEN=30 "+
			"CHANNELLEN=9 TOPICLEN=307 AWAYLEN=0 MAXTARGETS=1 MODES=1 CHANTYPES=# PREFIX=(aov)&@+ "+
			"CHANMODES=,k,lL,voantk NETWORK=PYX CASEMAPPING=ascii :are supported by this server")

	client.sendLUsers()
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

	client.joinChannel(client.config.GlobalChannel)
}

func handleLUsers(client *Client, msg Message) {
	client.sendLUsers()
}

func (client *Client) sendLUsers() {
	channels, err := client.getChannels()
	if err != nil {
		log.Errorf("Unable to retrieve game list for /lusers: %v", err)
		client.data <- client.n.format(ErrServiceConfused, client.nick,
			":Error retrieving game list: %s", err)
		return
	}
	channelCount := len(channels)

	names, err := client.pyx.Names()
	if err != nil {
		log.Errorf("Unable to retrieve user list for /lusers: %v", err)
		client.data <- client.n.format(ErrServiceConfused, client.nick,
			":Error retrieving user list: %s", err)
		return
	}
	userCount := len(names)

	// TODO maybe keep track of how many users are using the bridge and count them as "local"
	// and everyone else as "global"?
	client.data <- client.n.format(RplLUserClient, client.nick, ":There are %d users on 1 server",
		userCount)
	client.data <- client.n.format(RplLUserOp, client.nick, "%d :operator(s) online", 0)
	client.data <- client.n.format(RplLUserChannels, client.nick, "%d :channels formed",
		channelCount)
	client.data <- client.n.format(RplLUserMe, client.nick,
		":I have %d clients and %d servers", userCount, 0)
	client.data <- client.n.format(RplLocalUsers, client.nick,
		":Current Local Users: %d  Max: %d", userCount, userCount)
	client.data <- client.n.format(RplGlobalUsers, client.nick,
		":Current Global Users: %d  Max: %d", userCount, userCount)
}

// Send the stuff to the IRC client required when joining a channel. Assumes that the channel is
// valid to join.
func (client *Client) joinChannel(channel string) {
	client.data <- fmt.Sprintf(":%s JOIN :%s", client.getNickUserAtHost(client.nick), channel)

	client.handleTopicImpl(channel)
	client.handleNamesImpl(channel)
}

func handleNames(client *Client, msg Message) {
	client.handleNamesImpl(msg.args...)
}

func (client *Client) handleNamesImpl(args ...string) {
	if len(args) == 0 {
		client.data <- client.n.format(ErrNeedMoreParams, client.nick,
			"NAMES :Not enough parameters")
		return
	}

	if args[0] == client.config.GlobalChannel {
		names, err := client.pyx.Names()
		if err != nil {
			log.Errorf("Unable to retrieve names for %s: %v", args[0], err)
		}
		// TODO a proper length based on 512 minus broilerplate
		for _, line := range joinIntoLines(300, append(names, "&"+client.config.BotNick), " ") {
			client.data <- client.n.format(RplNames, client.nick, "= %s :%s", args[0], line)
		}
	} else {
		gameId, _, err := client.getGameFromChannel(args[0])
		if err != nil || gameId != *client.gameId {
			client.data <- client.n.format(ErrNotOnChannel, client.nick, "%s :Not in channel",
				args[0])
			return
		}
		resp, err := client.pyx.GameInfo(gameId)
		if err != nil {
			client.data <- client.n.format(ErrServiceConfused, client.nick,
				"%s :Cannot retrieve names: %s", args[0], err)
			return
		}
		players := []string{}
		for _, player := range resp.GameInfo.Players {
			if player == resp.GameInfo.Host {
				players = append(players, "@"+player)
				// this is a dumb place to do it, but we have the required info here...
				client.gameHost = player
			} else {
				players = append(players, "+"+player)
			}
		}
		// TODO a proper length based on 512 minus broilerplate
		for _, line := range joinIntoLines(300, append(append(players, resp.GameInfo.Spectators...),
			"&"+client.config.BotNick), " ") {
			client.data <- client.n.format(RplNames, client.nick, "= %s :%s", args[0], line)
		}
	}
	client.data <- client.n.format(RplEndNames, client.nick, "%s :End of /NAMES list", args[0])
}

func handleTopic(client *Client, msg Message) {
	client.handleTopicImpl(msg.args...)
}

func (client *Client) handleTopicImpl(args ...string) {
	if len(args) == 0 {
		// error to not specify channel
		client.data <- client.n.format(ErrNeedMoreParams, client.nick,
			"TOPIC :Not enough parameters")
	} else if len(args) == 1 {
		// show topic
		var topic string
		var set int64
		var setBy string
		if args[0] == client.config.GlobalChannel {
			topic = client.getTopic(args[0], nil)
			set = client.pyx.ServerStarted
			setBy = client.botNickUserAtHost()
		} else if client.gameId == nil {
			// user isn't in a game so they can't request a topic for a game
			client.data <- client.n.format(ErrNotOnChannel, client.nick, "%s :Not in channel.",
				args[0])
			return
		} else {
			requestedId, _, err := client.getGameFromChannel(args[0])
			if err != nil {
				client.data <- client.n.format(ErrNotOnChannel, client.nick, "%s :%s", args[0], err)
				return
			}
			if requestedId != *client.gameId {
				// user isn't in the game they asked for so they can't see it
				client.data <- client.n.format(ErrNotOnChannel, client.nick, "%s :Not in channel.",
					args[0])
				return
			}
			// okay, so the user is definitely in this game, so we can actually ask the pyx server
			// for the information we need
			resp, err := client.pyx.GameInfo(requestedId)
			if err != nil {
				log.Errorf("Unable to retrieve game %d info for /topic request: %s", requestedId,
					err)
				client.data <- client.n.format(ErrNotOnChannel, client.nick, "%s :%s", args[0], err)
				return
			}
			topic = client.getTopic(args[0], &resp.GameInfo)
			set = resp.GameInfo.Created
			setBy = client.getNickUserAtHost(resp.GameInfo.Host)
		}
		client.data <- client.n.format(RplTopic, client.nick, "%s :%s", args[0], topic)
		client.data <- client.n.format(RplTopicWhoTime, client.nick, "%s %s %d", args[0], setBy,
			set/1000)
	} else {
		// error to try to change topic
		// TODO is there a better numeric for this? we don't want to let ANYONE change it like this
		client.data <- client.n.format(ErrChanOpPrivsNeeded, client.nick,
			"TOPIC :You can't do that.")
	}
}

// Make the topic for a channel. gameInfo may be nil if the channel being passed is known to be
// the global channel.
func (client *Client) getTopic(channel string, gameInfo *pyx.GameInfo) string {
	if channel == client.config.GlobalChannel {
		if client.pyx.GlobalChatEnabled {
			return "Global chat"
		} else {
			return "Global chat (disabled)"
		}
	} else if gameInfo != nil {
		return makeGameTopic(gameInfo)
	} else {
		log.Errorf("Topic for channel %s requested but gameInfo is nil!", channel)
		return "(error generating topic)"
	}
}

func handleMode(client *Client, msg Message) {
	client.handleModeImpl(msg.args...)
}

func (client *Client) handleModeImpl(args ...string) {
	// TODO handle if the user is trying to change modes
	if len(args) == 0 {
		client.data <- client.n.format(ErrNeedMoreParams, client.nick,
			"MODE :Not enough parameters")
	} else if strings.HasPrefix(args[0], "#") {
		if len(args) == 1 {
			var modes string
			var created int64
			if args[0] == client.config.GlobalChannel {
				created = client.pyx.ServerStarted
				modes = "+t"
				if !client.pyx.GlobalChatEnabled {
					modes = modes + "m"
				}
				if client.pyx.BroadcastingUsers {
					modes = modes + "n"
				}
			} else if client.gameId == nil {
				// user isn't in a game so they can't view modes for a game
				client.data <- client.n.format(ErrNotOnChannel, client.nick,
					"%s :Not in channel.", args[0])
				return
			} else {
				requestedId, _, err := client.getGameFromChannel(args[0])
				if err != nil {
					client.data <- client.n.format(ErrNotOnChannel, client.nick, "%s :%s", args[0],
						err)
					return
				}
				if requestedId != *client.gameId {
					// user isn't in the game they asked for so they can't see it
					client.data <- client.n.format(ErrNotOnChannel, client.nick,
						"%s :Not in channel.", args[0])
					return
				}
				// okay, so the user is definitely in this game, so we can actually ask the pyx server
				// for the information we need
				resp, err := client.pyx.GameInfo(requestedId)
				if err != nil {
					log.Errorf("Unable to retrieve game %d info for /mode request: %s", requestedId,
						err)
					client.data <- client.n.format(ErrNotOnChannel, client.nick, "%s :%s", args[0],
						err)
					return
				}
				created = resp.GameInfo.Created

				modes = "+nt"
				if resp.GameInfo.HasPassword {
					modes = modes + "k"
				}
				modes = fmt.Sprintf("%slL %d %d", modes, resp.GameInfo.GameOptions.PlayerLimit+1,
					resp.GameInfo.GameOptions.SpectatorLimit+1)
			}
			client.data <- client.n.format(RplChannelModeIs, client.nick, "%s %s", args[0], modes)
			client.data <- client.n.format(RplCreationTime, client.nick, "%s %d", args[0],
				created/1000)
		} else {
			if args[1] == "b" {
				// irssi likes to request the ban list
				client.data <- client.n.format(RplEndOfBanList, client.nick,
					"%s :End of Channel Ban List", args[0])
			} else {
				// TODO handle if the user is trying to change modes
				// TODO but if they are the game host, they could change some of the settings
				client.data <- client.n.format(ErrChanOpPrivsNeeded, client.nick,
					"MODE :You can't do that.")
			}
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
			client.data <- client.n.format(RplUModeIs, client.nick, modes)
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
	client.data <- fmt.Sprintf(":%s PONG %s :%s", client.config.AdvertisedName,
		client.config.AdvertisedName, arg)
}

func handleWho(client *Client, msg Message) {
	if len(msg.args) == 0 || msg.args[0] == client.config.GlobalChannel {
		names, err := client.pyx.Names()
		if err != nil {
			log.Errorf("Unable to retrieve names for %s: %v", client.config.GlobalChannel, err)
		}

		client.data <- client.n.format(RplWho, client.nick, "%s %s %s %s %s HrB& :0 %s",
			client.config.GlobalChannel, client.config.BotUsername, client.config.AdvertisedName,
			client.config.AdvertisedName, client.config.BotNick, client.config.BotNick)
		for _, name := range names {
			modes := "H"
			if name[0:1] == pyx.Sigil_ADMIN {
				// technically admins might not be using an id code but we can't tell the difference
				// here
				modes = modes + "r"
			}
			if name[0:1] == pyx.Sigil_ID_CODE {
				modes = modes + "r"
			}
			// this doesn't apply to the server-wide who variant
			if len(msg.args) > 0 && name[0:1] == pyx.Sigil_ADMIN || name[0:1] == pyx.Sigil_ID_CODE {
				modes = modes + name[0:1]
				name = name[1:]
			}

			client.data <- client.n.format(RplWho, client.nick, "%s %s %s %s %s %s :0 %s",
				client.config.GlobalChannel, getUser(name), client.getHost(name),
				client.config.AdvertisedName, name, modes, name)
		}

		target := "*"
		if len(msg.args) > 0 {
			target = client.config.GlobalChannel
		}
		client.data <- client.n.format(RplEndOfWho, client.nick, "%s :End of /WHO list", target)
	} else if msg.args[0] == client.getGameChannel() {
		// TODO per-game channels, send something so irssi doesn't keep waiting
		client.data <- client.n.format(RplEndOfWho, client.nick, "%s :End of /WHO list",
			msg.args[0])
	} else {
		client.data <- client.n.format(ErrNotOnChannel, client.nick, "%s :Not in channel",
			msg.args[0])
	}
}

func handlePrivmsg(client *Client, msg Message) {
	if len(msg.args) == 0 {
		client.data <- client.n.format(ErrNeedMoreParams, client.nick,
			"PRIVMSG :Not enough parameters")
		return
	}
	if len(msg.args) == 1 || len(msg.args[1]) == 0 {
		client.data <- client.n.format(ErrNoTextToSend, client.nick, ":No text to send")
		return
	}

	channel := msg.args[0]
	isEmote, text := isEmote(msg.args[1])
	var err error
	if channel == client.config.GlobalChannel {
		err = client.pyx.SendGlobalChat(text, isEmote)
	} else if !strings.HasPrefix(channel, "#") {
		// trying to send a private message... we don't support that
		// unreal uses this for either
		client.data <- client.n.format(ErrNoSuchNick, client.nick, "%s :No such nick/channel",
			channel)
		return
	} else {
		// we need to let err belong to the outer scope
		var gameId int
		gameId, _, err = client.getGameFromChannel(channel)
		if err != nil || gameId != *client.gameId {
			// unreal uses this for either
			client.data <- client.n.format(ErrNoSuchNick, client.nick, "%s :No such nick/channel",
				channel)
			return
		}
		err = client.pyx.SendGameChat(gameId, text, isEmote)
	}

	if err != nil {
		client.data <- client.n.format(ErrCannotSendToChan, client.nick,
			"%s :Cannot send to channel: %s", channel, err)
	}
}

func handleWhois(client *Client, msg Message) {
	if len(msg.args) == 0 {
		client.data <- client.n.format(ErrNeedMoreParams, client.nick,
			"WHOIS :Not enough parameters")
		return
	}

	if strEqCI(client.config.BotNick, msg.args[0]) {
		client.data <- client.n.format(RplWhoisUser, client.nick, "%s %s %s * %s",
			client.config.BotNick, client.config.BotUsername, client.config.BotHostname,
			client.config.BotNick)
		channels := "&" + client.config.GlobalChannel
		if client.gameId != nil {
			channels = channels + " &" + client.getGameChannel()
		}
		client.data <- client.n.format(RplWhoisChannels, client.nick, "%s :%s",
			client.config.BotNick, channels)
		client.data <- client.n.format(RplWhoisServer, client.nick, "%s %s :%s",
			client.config.BotNick, client.config.AdvertisedName, client.config.Pyx.BaseAddress)
		client.data <- client.n.format(RplWhoisOperator, client.nick, "%s :is an Administrator",
			client.config.BotNick)
		client.data <- client.n.format(RplWhoisBot, client.nick, "%s :is a Bot",
			client.config.BotNick)

		client.data <- client.n.format(RplEndOfWhois, client.nick, "%s :End of /WHOIS list.",
			client.config.BotNick)
		return
	}

	// TODO special case for bot nick
	resp, err := client.pyx.Whois(msg.args[0])
	if err != nil {
		if resp.ErrorCode == pyx.ErrorCode_NO_SUCH_USER {
			client.data <- client.n.format(ErrNoSuchNick, client.nick, "%s :No such nick/channel",
				msg.args[0])
		} else {
			// I don't think we'd ever get here without something that would abort the connection
			client.data <- client.n.format(ErrNoSuchNick, client.nick, "%s :%s", msg.args[0], err)
		}
		client.data <- client.n.format(RplEndOfWhois, client.nick, "%s :End of /WHOIS list.",
			msg.args[0])
		return
	}

	nick := resp.Nickname
	sigil := resp.Sigil

	client.data <- client.n.format(RplWhoisUser, client.nick, "%s %s %s * :%s", nick,
		getUser(nick), client.getHost(nick), nick)
	if len(resp.IpAddress) > 0 {
		client.data <- client.n.format(RplWhoisHost, client.nick, "%s :is connecting from %s", nick,
			resp.IpAddress)
	}

	channels := sigil + client.config.GlobalChannel
	if resp.GameId != nil {
		channel := ""
		if resp.GameInfo.Host == nick {
			channel = "@"
		}
		prefix := client.config.GameChannelPrefix
		for _, spectator := range resp.GameInfo.Spectators {
			if spectator == nick {
				prefix = client.config.SpectateGameChannelPrefix
				break
			}
		}
		channel = channel + prefix + strconv.Itoa(*resp.GameId)
		channels = channels + " " + channel
	}
	client.data <- client.n.format(RplWhoisChannels, client.nick, "%s :%s", nick, channels)

	client.data <- client.n.format(RplWhoisServer, client.nick, "%s %s :%s", nick,
		client.config.AdvertisedName, client.config.Pyx.BaseAddress)
	if sigil == pyx.Sigil_ADMIN {
		client.data <- client.n.format(RplWhoisOperator, client.nick, "%s :is an Administrator",
			nick)
	}
	if len(resp.IdCode) > 0 {
		client.data <- client.n.format(RplWhoisSpecial, client.nick, "%s :Verification code: %s",
			nick, resp.IdCode)
	}
	if len(resp.ClientName) > 0 {
		client.data <- client.n.format(RplWhoisSpecial, client.nick, "%s :Client: %s", nick,
			resp.ClientName)
	}
	client.data <- client.n.format(RplWhoisIdle, client.nick, "%s %d %d :seconds idle, signon time",
		nick, resp.Idle/1000, resp.ConnectedAt/1000)
	client.data <- client.n.format(RplEndOfWhois, client.nick, "%s :/End of /WHOIS list.", nick)
}

func handleList(client *Client, msg Message) {
	channels, err := client.getChannels()
	if err != nil {
		log.Errorf("Unable to retrieve game list for /list: %v", err)
		client.data <- client.n.format(ErrServiceConfused, client.nick,
			":Error retrieving game list: %s", err)
		return
	}

	client.data <- client.n.format(RplListStart, client.nick, "Channel :Users  Name")
	for _, channel := range channels {
		client.data <- client.n.format(RplList, client.nick, "%s %d :%s", channel.name,
			channel.totalUsers, channel.topic)
	}
	client.data <- client.n.format(RplListEnd, client.nick, ":End of /LIST")
}

func handlePart(client *Client, msg Message) {
	if len(msg.args) == 0 {
		client.data <- client.n.format(ErrNeedMoreParams, client.nick,
			"PART :Not enough parameters")
		return
	}
	if msg.args[0] == client.config.GlobalChannel {
		// don't let them do that. might have to send a response to the irc client?
		log.Debugf("User %s tried to leave %s", client.nick, client.config.GlobalChannel)
		return
	}
	game, _, err := client.getGameFromChannel(msg.args[0])
	if err != nil || game != *client.gameId {
		client.data <- client.n.format(ErrNoSuchChannel, client.nick, "%s :No such channel",
			msg.args[0])
		return
	}

	resp, err := client.pyx.LeaveGame(game)
	// if the server thinks they're not in the game, then we want to process a successful removal
	// because this is a really weird state that shouldn't happen but we need to synchronize.
	// We probably would only ever see INVALID_GAME here
	if err != nil && resp.ErrorCode != pyx.ErrorCode_NOT_IN_THAT_GAME &&
		resp.ErrorCode != pyx.ErrorCode_INVALID_GAME {
		client.data <- client.n.format(ErrServiceConfused, client.nick,
			"%s :Unable to leave channel: %s", msg.args[0], err)
	} else {
		client.gameId = nil
		client.data <- fmt.Sprintf(":%s PART %s", client.getNickUserAtHost(client.nick),
			msg.args[0])
	}
}

func handleJoin(client *Client, msg Message) {
	if len(msg.args) == 0 {
		client.data <- client.n.format(ErrNeedMoreParams, client.nick,
			"JOIN :Not enough parameters")
		return
	}
	if client.gameId != nil {
		// only allowed to have one game at a time
		client.data <- client.n.format(ErrTooManyChannels, client.nick,
			"%s :Too many joined channels.", msg.args[0])
		return
	}

	gameId, spectate, err := client.getGameFromChannel(msg.args[0])
	if err != nil {
		client.data <- client.n.format(ErrForbiddenChannel, client.nick,
			"%s :Forbidden channel: %s", msg.args[0], err)
		return
	}

	key := ""
	if len(msg.args) >= 2 {
		key = msg.args[1]
	}

	// TODO create a new game
	var resp *pyx.AjaxResponse
	if spectate {
		resp, err = client.pyx.SpectateGame(gameId, key)
		// TODO move this out to be common code once playable games are supported
		if err != nil {
			switch resp.ErrorCode {
			case pyx.ErrorCode_CANNOT_JOIN_ANOTHER_GAME:
				// we're in a desynchronized state at this point, since we didn't know the user was
				// in a game...
				log.Errorf("Desync detected: User %s, pyx server said they're already in a game",
					client.nick)
				client.data <- client.n.format(ErrTooManyChannels, client.nick,
					"%s :Too many joined channels", msg.args[0])
			case pyx.ErrorCode_GAME_FULL:
				client.data <- client.n.format(ErrChannelIsFull, client.nick, "%s :Channel is full",
					msg.args[0])
			case pyx.ErrorCode_INVALID_GAME:
				// we will support a special channel name to create a new game, since the server
				// assigns the game IDs
				client.data <- client.n.format(ErrNoSuchChannel, client.nick, "%s :No such channel",
					msg.args[0])
			case pyx.ErrorCode_WRONG_PASSWORD:
				client.data <- client.n.format(ErrBadChannelKey, client.nick, "%s :Wrong key",
					msg.args[0])
			default:
				client.data <- client.n.format(ErrServiceConfused, client.nick,
					"%s :Cannot join game: %s", msg.args[0], err)
			}
			return
		}
		client.gameId = &gameId
		// TODO move
		client.gameIsSpectate = spectate
		client.gameInProgress = false
		client.joinChannel(msg.args[0])
	} else {
		// TODO support playable games
		// resp, err := client.pyx.JoinGame(gameId, key)
		client.data <- client.n.format(ErrForbiddenChannel, client.nick,
			"%s :Cannot join game playing channels", msg.args[0])
	}
}

func (client *Client) getChannels() ([]ChannelInfo, error) {
	resp, err := client.pyx.GameList()
	if err != nil {
		return []ChannelInfo{}, err
	}

	names, err := client.pyx.Names()
	if err != nil {
		return []ChannelInfo{}, err
	}
	userCount := len(names)

	games := []ChannelInfo{{
		name:       client.config.GlobalChannel,
		totalUsers: userCount + 1,
		topic:      client.getTopic(client.config.GlobalChannel, nil),
	}}
	for _, game := range resp.Games {
		info := ChannelInfo{
			name:       client.config.GameChannelPrefix + strconv.Itoa(game.Id),
			totalUsers: totalUserCount(&game),
			topic:      makeGameTopic(&game),
		}
		games = append(games, info)
		if game.GameOptions.SpectatorLimit > 0 {
			info = ChannelInfo{
				name:       client.config.SpectateGameChannelPrefix + strconv.Itoa(game.Id),
				totalUsers: totalUserCount(&game),
				topic:      "SPECTATE: " + makeGameTopic(&game),
			}
			games = append(games, info)
		}
	}
	return games, nil
}

func handleWhowas(client *Client, msg Message) {
	if len(msg.args) == 0 {
		client.data <- client.n.format(ErrNeedMoreParams, client.nick,
			"WHOWAS :Not enough parameters")
		return
	}
	client.data <- client.n.format(ErrWasNoSuchNick, client.nick, "%s :WHOWAS is not supported.",
		msg.args[0])
	client.data <- client.n.format(RplEndOfWhowas, client.nick, "%s :/End of WHOWAS", msg.args[0])
}
