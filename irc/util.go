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
	"fmt"
	"strings"
)

const CtcpMagic byte = 1

// Assemble the values in pieces into one or more space-separated strings, with no more than
// charsPerLine characters per line.
func joinIntoLines(charsPerLine int, pieces []string) []string {
	var ret []string
	var curLine string
	for _, val := range pieces {
		if len(val) > charsPerLine {
			panic(fmt.Sprintf("Impossibly long piece %s longer than %d", val, charsPerLine))
		} else if len(curLine) == 0 {
			curLine = val
		} else if len(curLine)+1+len(val) > charsPerLine {
			ret = append(ret, curLine)
			curLine = val
		} else {
			curLine = curLine + " " + val
		}
	}
	return append(ret, curLine)
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
