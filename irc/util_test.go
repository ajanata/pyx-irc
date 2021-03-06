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

type joinLineTestPair struct {
	input   []string
	perLine int
	joiner  string
	output  []string
}

var joinLineTests = []joinLineTestPair{
	{[]string{"a", "b", "c"}, 5, " ", []string{"a b c"}},
	{[]string{"a", "b", "c"}, 3, " ", []string{"a b", "c"}},
	{[]string{"a", "b", "c"}, 2, " ", []string{"a", "b", "c"}},
	{[]string{"a", "b", "c"}, 1, " ", []string{"a", "b", "c"}},
	{[]string{"a", "b", "c"}, 2, ", ", []string{"a,", "b,", "c"}},
	{[]string{"a", "b", "c"}, 3, ", ", []string{"a,", "b,", "c"}},
	{[]string{"a", "b", "c"}, 4, ", ", []string{"a, b,", "c"}},
}

func TestJoinIntoLines(t *testing.T) {
	for _, test := range joinLineTests {
		out := joinIntoLines(test.perLine, test.input, test.joiner)
		if len(test.output) != len(out) {
			t.Logf("In: %v, out: %v", test.input, out)
			t.Error("For", test.input,
				"max len", test.perLine,
				"expected output length", len(test.output),
				"got", len(out),
			)
		} else {
			for i, _ := range test.output {
				if test.output[i] != out[i] {
					t.Error("For", test.input,
						"max len", test.perLine,
						"expected output ", i,
						"to be", test.output[i],
						"got", out[i],
					)
				}
			}
		}
	}
}
