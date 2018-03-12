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
	"testing"
)

type testpair struct {
	input string
	cmd   string
	args  []string
}

var tests = []testpair{
	{"", "", []string{}},
	{"nick test", "NICK", []string{"test"}},
	{"nick    test", "NICK", []string{"test"}},
	{"   nick    test   ", "NICK", []string{"test"}},
	{"user test 0 0 :test user", "USER", []string{"test", "0", "0", "test user"}},
	{"privmsg #test :testing 1 2 3", "PRIVMSG", []string{"#test", "testing 1 2 3"}},
	{"privmsg   #test    :testing 1 2 3   ", "PRIVMSG", []string{"#test", "testing 1 2 3"}},
	{"privmsg   #test    :", "PRIVMSG", []string{"#test", ""}},
}

func TestNewMessage(t *testing.T) {
	for _, test := range tests {
		m := NewMessage(test.input)
		if m.cmd != test.cmd {
			t.Error("For", test.input,
				"expected cmd", test.cmd,
				"got", m.cmd,
			)
		}
		if len(test.args) != len(m.args) {
			t.Error("For", test.input,
				"expected arg length", len(test.args),
				"got", len(m.args),
			)
		} else {
			for i, _ := range test.args {
				if test.args[i] != m.args[i] {
					t.Error("For", test.input,
						"expected arg ", i,
						"to be", test.args[i],
						"got", m.args[i],
					)
				}
			}
		}
	}
}
