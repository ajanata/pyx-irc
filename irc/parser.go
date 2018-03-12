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
	"regexp"
	"strings"
)

var whitespaceRegex = regexp.MustCompile("\\s+")

type Message struct {
	cmd  string
	args []string
	orig string
}

func NewMessage(input string) Message {
	msg := Message{orig: input}

	input = strings.TrimSpace(input)
	// easy case if we don't have any trail
	if !strings.Contains(input, ":") {
		parts := whitespaceRegex.Split(input, -1)
		msg.cmd = parts[0]
		msg.args = parts[1:]
	} else {
		// have to do this a bit more complicated
		bigparts := strings.SplitN(input, ":", 2)
		parts := whitespaceRegex.Split(strings.TrimSpace(bigparts[0]), -1)
		msg.cmd = parts[0]
		msg.args = parts[1:]
		msg.args = append(msg.args, bigparts[1])
	}

	msg.cmd = strings.ToUpper(msg.cmd)

	log.Debugf("Parsed message, cmd: %s args: %s", msg.cmd, msg.args)
	return msg
}
