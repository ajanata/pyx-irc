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
	"errors"
	"fmt"
	"github.com/ajanata/pyx-irc/pyx"
	"strconv"
	"strings"
)

const CtcpMagic byte = 1

// Assemble the values in pieces into one or more space-separated strings, with no more than
// charsPerLine characters per line.
func joinIntoLines(charsPerLine int, pieces []string, joiner string) []string {
	var ret []string
	var curLine string
	trimmedJoiner := strings.TrimSpace(joiner)
	for _, val := range pieces {
		if len(trimmedJoiner)+len(val) > charsPerLine {
			panic(fmt.Sprintf("Impossibly long piece %s longer than %d", val, charsPerLine))
		} else if len(curLine) == 0 {
			curLine = val
		} else if len(curLine)+len(joiner)+len(val) > charsPerLine {
			ret = append(ret, curLine+trimmedJoiner)
			curLine = val
		} else {
			curLine = curLine + joiner + val
		}
	}
	return append(ret, curLine)
}

func (client *Client) botNickUserAtHost() string {
	return fmt.Sprintf("%s!%s@%s", client.config.BotNick, client.config.BotUsername,
		client.config.BotHostname)
}

func (client *Client) getNickUserAtHost(nick string) string {
	return fmt.Sprintf("%s!%s@%s", nick, getUser(nick), client.getHost(nick))
}

func getUser(nick string) string {
	user := nick
	if len(user) > 10 {
		user = user[:10]
	}
	return strings.ToLower(user)
}

func (client *Client) getHost(nick string) string {
	// TODO unique hosts per user? idk.
	return "users." + client.config.AdvertisedName
}

func isEmote(msg string) (bool, string) {
	if msg[0] == CtcpMagic && msg[len(msg)-1] == CtcpMagic && len(msg) > len("ACTION")+2 &&
		msg[1:len("ACTION")+1] == "ACTION" {
		return true, msg[len("ACTION")+2 : len(msg)-1]
	}
	return false, msg
}

func makeEmote(msg string) string {
	log.Debugf("Converting to emote: %s", msg)
	return fmt.Sprintf("%cACTION %s%c", CtcpMagic, msg, CtcpMagic)
}

func totalUserCount(game *pyx.GameInfo) int {
	return len(game.Players) + len(game.Spectators)
}

func makeGameTopic(game *pyx.GameInfo) string {
	// TODO include information about card sets, but cardcast stuff isn't included in this data set
	// at all...
	passwdLabel := ""
	if game.HasPassword {
		passwdLabel = "(Has password.) "
	}
	return fmt.Sprintf("%s's game (%s). %s%d score goal. %d/%d players, %d/%d spectators.",
		game.Host, pyx.GameStateMsgs[game.State], passwdLabel, game.GameOptions.ScoreLimit,
		len(game.Players), game.GameOptions.PlayerLimit, len(game.Spectators),
		game.GameOptions.SpectatorLimit)
}

func (client *Client) getGameFromChannel(channel string) (int, bool, error) {
	if strings.HasPrefix(channel, client.config.GameChannelPrefix) {
		id, err := strconv.Atoi(channel[len(client.config.GameChannelPrefix):])
		if err != nil {
			goto badChannel
		}
		return id, false, nil
	} else if strings.HasPrefix(channel, client.config.SpectateGameChannelPrefix) {
		id, err := strconv.Atoi(channel[len(client.config.SpectateGameChannelPrefix):])
		if err != nil {
			goto badChannel
		}
		return id, true, nil
	}
badChannel:
	return -1, false, errors.New("Channel name does not match game channel name format.")
}

func (client *Client) getGameChannel() string {
	if client.gameId == nil {
		return ""
	}
	if client.gameIsSpectate {
		return client.config.SpectateGameChannelPrefix + strconv.Itoa(*client.gameId)
	} else {
		return client.config.GameChannelPrefix + strconv.Itoa(*client.gameId)
	}
}

func blackCardText(card pyx.BlackCardData) string {
	return fmt.Sprintf("(Pick %d, source %s) %s", card.Pick, card.Watermark, card.Text)
}

func whiteCardText(card pyx.WhiteCardData) string {
	return fmt.Sprintf("%s (source %s)", card.Text, card.Watermark)
}

func getJudge(playerInfo *[]pyx.GamePlayerInfo) string {
	for _, player := range *playerInfo {
		if player.Status == pyx.GamePlayerStatus_JUDGE ||
			player.Status == pyx.GamePlayerStatus_JUDGING {
			return player.Name
			break
		}
	}
	// This should be impossible
	log.Error("getJudge called without a judge in the player info?!")
	return ""
}
