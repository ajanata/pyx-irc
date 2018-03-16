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
const RplWelcome = "001"
const RplYourHost = "002"
const RplMyInfo = "004"
const RplISupport = "005"

const RplUModeIs = "221"
const RplLUserClient = "251"
const RplLUserOp = "252"
const RplLUserChannels = "254"
const RplLUserMe = "255"
const RplLocalUsers = "265"
const RplGlobalUsers = "266"

const RplEndOfWho = "315"
const RplChannelModeIs = "324"
const RplCreationTime = "329"
const RplTopic = "332"
const RplTopicWhoTime = "333"
const RplWho = "352"
const RplNames = "353"
const RplEndNames = "366"
const RplBanList = "367"
const RplEndOfBanList = "368"

// errors
const ErrNoSuchNick = "401"
const ErrNoSuchChannel = "403"
const ErrCannotSendToChan = "404"
const ErrTooManyChannels = "405"
const ErrTooManyTargets = "407"
const ErrNoRecipient = "411"
const ErrNoTextToSend = "412"
const ErrUnknownCommand = "421"
const ErrNoMotd = "422"
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
const ErrChanOpPrivsNeeded = "482"

func formatSimpleReply(numeric string, target string, msg string) string {
	return formatFmt(numeric, target, ":%s", msg)
}

func formatFmt(numeric string, target string, format string, args ...interface{}) string {
	return fmt.Sprintf(":%s %s %s %s", MyServerName, numeric, target, fmt.Sprintf(format, args...))
}
