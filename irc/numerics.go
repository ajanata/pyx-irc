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
)

// replies

// errors
const ErrNoSuchNick = "401"
const ErrNoSuchChannel = "403"
const ErrCannotSendToChan = "404"
const ErrTooManyChannels = "405"
const ErrTooManyTargets = "407"
const ErrNoRecipient = "411"
const ErrNoTextToSend = "412"
const ErrUnknownCommand = "421"
const ErrNoNicknameGiven = "431"
const ErrErroneousNickname = "432"
const ErrNicknameInUse = "433"
const ErrNickCollision = "436"
const ErrNoNickChange = "447"
const ErrForbiddenChannel = "448"
const ErrNotRegistered = "451"
const ErrNeedMoreParams = "461"
const ErrAlreadyRegistered = "462"
const ErrKeySet = "467"
const ErrChannelIsFull = "471"
const ErrBadChannelKey = "475"

func formatSimpleError(server string, numeric string, verb string, msg string) string {
	return fmt.Sprintf(":%s %s %s :%s", server, numeric, verb, msg)
}
