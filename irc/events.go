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

// PYX event handlers

package irc

import (
	"fmt"
	"github.com/ajanata/pyx-irc/pyx"
	"strconv"
	"strings"
)

type Event = pyx.LongPollResponse
type EventHandlerFunc func(*Client, Event)

var EventHandlers = map[string]EventHandlerFunc{
	pyx.LongPollEvent_BANNED:               eventBanned,
	pyx.LongPollEvent_CHAT:                 eventChat,
	pyx.LongPollEvent_KICKED:               eventKicked,
	pyx.LongPollEvent_FILTERED_CHAT:        eventFilteredChat,
	pyx.LongPollEvent_GAME_BLACK_RESHUFFLE: eventGameBlackShuffle,
	pyx.LongPollEvent_GAME_LIST_REFRESH:    eventIgnore,
	// TODO implement this? We can say when players played a card, if we want to...
	pyx.LongPollEvent_GAME_PLAYER_INFO_CHANGE: eventIgnore,
	pyx.LongPollEvent_GAME_PLAYER_JOIN:        eventGamePlayerJoin,
	pyx.LongPollEvent_GAME_PLAYER_KICKED_IDLE: eventGamePlayerKickedIdle,
	pyx.LongPollEvent_GAME_PLAYER_LEAVE:       eventGamePlayerLeave,
	pyx.LongPollEvent_GAME_PLAYER_SKIPPED:     eventGamePlayerSkipped,
	pyx.LongPollEvent_GAME_ROUND_COMPLETE:     eventGameRoundComplete,
	pyx.LongPollEvent_GAME_SPECTATOR_JOIN:     eventGamePlayerJoin,
	pyx.LongPollEvent_GAME_SPECTATOR_LEAVE:    eventGamePlayerLeave,
	pyx.LongPollEvent_GAME_STATE_CHANGE:       eventGameStateChange,
	pyx.LongPollEvent_GAME_WHITE_RESHUFFLE:    eventGameWhiteShuffle,
	pyx.LongPollEvent_NEW_PLAYER:              eventNewPlayer,
	pyx.LongPollEvent_PLAYER_LEAVE:            eventPlayerQuit,
}

func eventNewPlayer(client *Client, event Event) {
	if event.Nickname == client.pyx.User.Name {
		// we don't care about seeing ourselves connect
		return
	}
	client.data <- fmt.Sprintf(":%s JOIN :%s", client.getNickUserAtHost(event.Nickname),
		client.config.GlobalChannel)
	mode := "+"
	modeNames := ""
	if event.Sigil == pyx.Sigil_ADMIN {
		mode = mode + "o"
		modeNames = event.Nickname
	}
	if len(event.IdCode) > 0 {
		mode = mode + "v"
		modeNames = modeNames + " " + event.Nickname
	}
	if len(mode) > 1 {
		client.data <- fmt.Sprintf(":%s MODE %s %s %s", client.botNickUserAtHost(),
			client.config.GlobalChannel, mode, strings.TrimSpace(modeNames))
	}
}

func eventPlayerQuit(client *Client, event Event) {
	if event.Nickname == client.pyx.User.Name {
		// we don't care about seeing ourselves disconnect
		// TODO unless we got kicked or banned
		// actually those are different events entirely
		return
	}
	client.data <- fmt.Sprintf(":%s QUIT :%s", client.getNickUserAtHost(event.Nickname),
		pyx.DisconnectReasonMsgs[event.Reason])
}

func eventFilteredChat(client *Client, event Event) {
	if event.GameId != nil && (client.gameId == nil || *event.GameId != *client.gameId) {
		event.Message = fmt.Sprintf("(In game %d) %s", *event.GameId, event.Message)
		event.GameId = nil
	}
	event.Message = "(Filtered) " + event.Message
	eventChat(client, event)
}

func eventChat(client *Client, event Event) {
	if event.From == client.pyx.User.Name {
		// don't show our own chat
		return
	}
	if event.Wall {
		// global notice from admin, handle this completely differently
		client.data <- fmt.Sprintf(":%s NOTICE %s :Global notice: %s",
			client.getNickUserAtHost(event.From), client.nick, event.Message)
		return
	}

	var target string
	// game chat is the same event, but has the game id field
	if event.GameId != nil {
		if *event.GameId == *client.gameId {
			target = client.config.GameChannelPrefix + strconv.Itoa(*event.GameId)
			if client.gameIsSpectate {
				target = client.config.SpectateGameChannelPrefix + strconv.Itoa(*event.GameId)
			}
		} else {
			// uhhh wtf??
			log.Errorf("Received game chat for un-joined gamed %d (joined %d)", *event.GameId,
				*client.gameId)
			return
		}
	} else {
		target = client.config.GlobalChannel
	}
	text := event.Message
	if event.Emote {
		text = makeEmote(text)
	}
	client.data <- fmt.Sprintf(":%s PRIVMSG %s :%s", client.getNickUserAtHost(event.From), target,
		text)
}

func eventIgnore(client *Client, event Event) {
	// do nothing with this event.
}

func eventBanned(client *Client, event Event) {
	doKickOrBan(client, "You have been banned by the server administrator.")
}

func eventKicked(client *Client, event Event) {
	doKickOrBan(client, "You have been kicked by the server administrator.")
}

func doKickOrBan(client *Client, msg string) {
	s := fmt.Sprintf(":%s KILL %s :%s!%s (%s)", client.botNickUserAtHost(), client.nick,
		client.config.AdvertisedName, client.config.BotNick, msg)
	// have to do this differently to ensure the client actually gets this in the right order
	client.writer.WriteString(s + "\r\n")
	client.writer.Flush()

	client.disconnect(fmt.Sprintf("%s (Killed (%s (%s)))", client.config.AdvertisedName,
		client.config.BotNick, msg))
}

func (client *Client) sendTopicChangeForStartedGame() {
	// we still end up showing the topic change if the game was already in progress when the user
	// joined... oh well
	if !client.gameInProgress {
		client.gameInProgress = true
		client.sendTopicChange()
	}
}

func (client *Client) sendTopicChange() {
	channel := client.getGameChannel()
	resp, err := client.pyx.GameInfo(*client.gameId)
	if err != nil {
		log.Errorf("Unable to retrieve game %d info for player join topic update: %s",
			*client.gameId, err)
		return
	}
	topic := client.getTopic(channel, &resp.GameInfo)
	client.data <- fmt.Sprintf(":%s TOPIC %s :%s", client.botNickUserAtHost(), channel, topic)
}

func (client *Client) sendBotMessageToGame(format string, args ...interface{}) {
	// TODO deal with messages that are long than the IRC length limit?
	client.data <- fmt.Sprintf(":%s PRIVMSG %s :%s", client.botNickUserAtHost(),
		client.getGameChannel(), fmt.Sprintf(format, args...))
}

// also handles Game Spectator Join
func eventGamePlayerJoin(client *Client, event Event) {
	if event.Nickname == client.pyx.User.Name {
		// ignore join events for ourselves
		return
	}
	nick := event.Nickname
	channel := client.getGameChannel()
	client.data <- fmt.Sprintf(":%s JOIN %s", client.getNickUserAtHost(nick), channel)
	if event.Event == pyx.LongPollEvent_GAME_PLAYER_JOIN {
		client.data <- fmt.Sprintf(":%s MODE %s +v %s", client.botNickUserAtHost(), channel, nick)
	}

	client.sendTopicChange()
}

// also handles Game Spectator Leave
func eventGamePlayerLeave(client *Client, event Event) {
	if event.Nickname == client.pyx.User.Name {
		// ignore leave for ourselves
		return
	}
	client.data <- fmt.Sprintf(":%s PART %s :Leaving", client.getNickUserAtHost(event.Nickname),
		client.getGameChannel())
	client.processPlayerLeave(event)
}

func eventGamePlayerKickedIdle(client *Client, event Event) {
	// TODO handle us being kicked for idle once we can play in games
	client.data <- fmt.Sprintf(":%s KICK %s %s :Idle for too many rounds",
		client.botNickUserAtHost(), client.getGameChannel(), event.Nickname)
	client.processPlayerLeave(event)
}

func (client *Client) processPlayerLeave(event Event) {
	if event.Nickname == client.gameHost {
		resp, err := client.pyx.GameInfo(*client.gameId)
		if err != nil {
			if resp.ErrorCode == pyx.ErrorCode_INVALID_GAME {
				// the game has been destroyed since all non-spectators left. yes, the server
				// doesn't actually tell spectators about this...
				log.Debugf("We got kicked from game %d!", *client.gameId)
				client.data <- fmt.Sprintf(":%s KICK %s %s :Forcibly removed by server.",
					client.botNickUserAtHost(), client.getGameChannel(), client.nick)
				client.gameId = nil
				return
			} else {
				log.Errorf("Cannot retrieve game info for game %d to determine new host",
					*client.gameId)
			}
		} else {
			client.data <- fmt.Sprintf(":%s MODE %s +o %s", client.botNickUserAtHost(),
				client.getGameChannel(), resp.GameInfo.Host)
		}
	}
	client.sendTopicChange()
}

func eventGameStateChange(client *Client, event Event) {
	switch event.GameState {
	case pyx.GameState_LOBBY:
		client.sendTopicChange()
		client.sendBotMessageToGame("The game has been reset to the lobby state.")
		client.gameInProgress = false
	case pyx.GameState_PLAYING:
		client.sendTopicChangeForStartedGame()
		client.sendBotMessageToGame("The black card for the next round is: %s",
			blackCardText(event.BlackCard))
		resp, err := client.pyx.GameInfo(*event.GameId)
		if err != nil {
			log.Errorf("Unable to obtain status for game %d after state change", *event.GameId)
			return
		}
		judge := getJudge(&resp.PlayerInfo)
		if judge == client.pyx.User.Name {
			client.sendBotMessageToGame("You are judging this round.")
		} else {
			client.sendBotMessageToGame("The judge this round is %s.", judge)
			if !client.gameIsSpectate {
				// TODO show hand and ask for plays, and include the PLAY_TIMER
			}
		}
	case pyx.GameState_JUDGING:
		// save these for later
		client.gamePlayedCards = &event.WhiteCards
		cardPlural := ""
		if len(event.WhiteCards[0]) > 1 {
			cardPlural = "s"
		}
		client.sendBotMessageToGame("The white cards for this round are:")
		for i, cards := range event.WhiteCards {
			msg := fmt.Sprintf("(Selection %d)", i)
			for _, card := range cards {
				msg = fmt.Sprintf("%s [%s]", msg, whiteCardText(card))
			}
			client.sendBotMessageToGame(msg)
		}
		resp, err := client.pyx.GameInfo(*event.GameId)
		if err != nil {
			log.Errorf("Unable to obtain status for game %d after state change", *event.GameId)
			return
		}
		judge := getJudge(&resp.PlayerInfo)
		if judge == client.pyx.User.Name {
			// TODO ask for judging
		} else {
			client.sendBotMessageToGame("Please wait while %s selects the winning card%s.", judge,
				cardPlural)
		}
	default:
		log.Errorf("Unknown game state %s", event.GameState)
	}
}

func eventGameRoundComplete(client *Client, event Event) {
	// so the white card winning ID is only one of the cards if it's a pick-multiple...
	winningCard := ""
	for _, cards := range *client.gamePlayedCards {
		// the provided ID will always be the first card that a player played, so we can just check
		// that one
		if cards[0].Id == event.WinningCard {
			for _, card := range cards {
				winningCard = fmt.Sprintf("%s [%s]", winningCard, whiteCardText(card))
			}
			break
		}
	}
	// yes that missing space is intentional, it'll be provided by the above formatting
	client.sendBotMessageToGame("The round was won by %s by playing%s.", event.RoundWinner,
		winningCard)
	client.showScoreboard()
}

func (client *Client) showScoreboard() error {
	resp, err := client.pyx.GameInfo(*client.gameId)
	if err != nil {
		log.Errorf("Unable to obtain info about game %d to display scoreboard", *client.gameId)
		return err
	}

	scores := []string{}
	winner := ""
	for _, info := range resp.PlayerInfo {
		if info.Status == pyx.GamePlayerStatus_WINNER {
			winner = info.Name
		}
		plural := "s"
		if info.Score == 1 {
			plural = ""
		}
		scores = append(scores, fmt.Sprintf("%s with %d point%s", info.Name, info.Score, plural))
	}
	// TODO a proper length based on 512 minus broilerplate
	scoresAssembled := joinIntoLines(300, scores, ", ")
	if winner != "" {
		client.sendBotMessageToGame("The game was won by %s! The final scores are: %s.", winner,
			scoresAssembled[0])
	} else {
		client.sendBotMessageToGame("The current scores are: %s.", scoresAssembled[0])
	}
	if len(scoresAssembled) > 1 {
		for i := 1; i < len(scoresAssembled); i++ {
			client.sendBotMessageToGame(scoresAssembled[i])
		}
	}
	return nil
}

func eventGamePlayerSkipped(client *Client, event Event) {
	client.sendBotMessageToGame("%s was skipped this round for being idle.", event.Nickname)
}

func eventGameWhiteShuffle(client *Client, event Event) {
	client.sendBotMessageToGame("The discarded white cards have been re-shuffled into a new deck.")
}

func eventGameBlackShuffle(client *Client, event Event) {
	client.sendBotMessageToGame("The discarded black cards have been re-shuffled into a new deck.")
}
