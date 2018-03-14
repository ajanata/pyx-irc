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

package pyx

import (
	"encoding/json"
	"fmt"
	"gopkg.in/resty.v1"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FIXME config
const PyxBaseUrl = "http://loopback.pretendyoure.xyz:8080/"

var globalChatEnabledRegex = regexp.MustCompile("cah.GLOBAL_CHAT_ENABLED = (true|false);")

type Client struct {
	GlobalChatEnabled bool
	IncomingEvents    chan *LongPollResponse
	User              *User
	stop              chan bool
	pollWg            sync.WaitGroup
	http              *resty.Client
	sessionId         string
	serial            int
}

func NewClient(nick string, idcode string) (*Client, error) {
	client := &Client{
		IncomingEvents: make(chan *LongPollResponse),
		stop:           make(chan bool, 1),
		http:           resty.New(),
	}

	client.http.
		SetDebug(true).
		SetHeader("User-Agent", "PYX-IRC").
		SetHostURL(PyxBaseUrl).
		SetRetryCount(3).
		SetTimeout(time.Duration(1 * time.Minute))

	err := client.prepare()
	if err != nil {
		return client, err
	}
	return client, client.login(nick, idcode)
}

// long poll goroutine
func (client *Client) receive() {
	log.Debugf("Starting long poll routine for session %s", client.sessionId)
	client.pollWg.Add(1)
	for {
		select {
		case <-client.stop:
			log.Infof("Stopping long poll for client %s", client.sessionId)
			client.pollWg.Done()
			return
		default:
			resp, err := client.http.NewRequest().
				Post("/LongPollServlet")

			if err != nil {
				log.Errorf("Long poll for session %s received error: %+v", client.sessionId, err)
				// order matters here!
				client.pollWg.Done()
				client.Close()
				return
			}

			var res interface{}
			// this is dumb but I can't figure out another way to do it
			if strings.HasPrefix(resp.String(), "[") {
				// array of LongPollResponse
				var t []*LongPollResponse
				err = json.Unmarshal(resp.Body(), &t)
				res = t
			} else {
				var t *LongPollResponse
				err = json.Unmarshal(resp.Body(), &t)
				res = t
			}

			log.Debugf("Result: %+v", res)
			switch v := res.(type) {
			case *LongPollResponse:
				// bare object, likely an error or no-op
				singleResult := v
				err = checkPollForError(singleResult, err)
				if err != nil {
					log.Errorf("Long poll for session %s received error: %+v", client.sessionId,
						err)
					// order matters here!
					client.pollWg.Done()
					client.Close()
					return
				}
				client.dispatchSinglePyxEvent(singleResult)
			case []*LongPollResponse:
				// array of objects, so can't be an error
				for _, event := range v {
					client.dispatchSinglePyxEvent(event)
				}
			default:
				log.Errorf("No idea what the type of this is: %+v", res)
			}
		}
	}
}

func (client *Client) dispatchSinglePyxEvent(event *LongPollResponse) {
	log.Debugf("Received long poll for session %s: %+v", client.sessionId, event)
	if event.Event == LongPollEvent_NOOP {
		return
	}
	client.IncomingEvents <- event
}

// Make initial contact with PYX and obtain a session. Obtain server configuration information.
// Does not log in. Logging in should be done within half a minute of this call so that the session
// does not expire.
func (client *Client) prepare() error {
	resp, err := client.http.NewRequest().Get("/game.jsp")
	if err != nil {
		return err
	}
	for _, c := range resp.Cookies() {
		if "JSESSIONID" == c.Name {
			client.sessionId = c.Value
			break
		}
	}
	client.http.SetCookies(resp.Cookies())

	resp, err = client.http.NewRequest().Get("/js/cah.config.js")
	if err != nil {
		return err
	}
	matches := globalChatEnabledRegex.FindStringSubmatch(resp.String())
	if len(matches) > 1 {
		client.GlobalChatEnabled, _ = strconv.ParseBool(matches[1])
	}

	flResp, err := client.Send(map[string]string{
		AjaxRequest_OP: AjaxOperation_FIRST_LOAD,
	})
	if err != nil {
		return err
	}
	// TODO save the card sets somewhere
	if flResp.InProgress {
		return fmt.Errorf("Session %s already in progress, not yet implemented (next=%s)",
			client.sessionId, flResp.Next)
	}
	log.Debugf("Cards: %+v", flResp.CardSets)

	return nil
}

// Log in to the server and start the long poll goroutine
func (client *Client) login(nick string, idcode string) error {
	// TODO persistent ID?
	req := map[string]string{
		AjaxRequest_OP:       AjaxOperation_REGISTER,
		AjaxRequest_NICKNAME: nick,
	}
	if len(idcode) > 0 {
		req[AjaxRequest_ID_CODE] = idcode
	}
	resp, err := client.Send(req)
	err = checkForError(resp, err)
	if err != nil {
		return err
	}

	client.User = newUser(resp.Nickname, resp.Sigil, resp.IdCode)

	go client.receive()

	return nil
}

// Make the request on the server, and check for PYX application errors.
func (client *Client) Send(request map[string]string) (*AjaxResponse, error) {
	resp, err := client.sendNoErrorCheck(request)
	return resp, checkForError(resp, err)
}

// Check for an error condition in a server response. If the passed in reqError is not nil, that is
// returned directly. Otherwise, if the ERROR field in response is true, an error containing the
// ErrorCodeMsg for the ERROR_CODE is returned. If neither of these are true, then nil is returned.
func checkForError(response *AjaxResponse, reqError error) error {
	if reqError != nil {
		return reqError
	}
	if response.Error {
		return fmt.Errorf("PYX error: %s", ErrorCodeMsgs[response.ErrorCode])
	}
	return nil
}

// Same as checkForError but for long polls instead of requests.
func checkPollForError(response *LongPollResponse, reqError error) error {
	if reqError != nil {
		return reqError
	}
	if response.Error {
		return fmt.Errorf("PYX error: %s", ErrorCodeMsgs[response.ErrorCode])
	}
	return nil
}

func (client *Client) sendNoErrorCheck(request map[string]string) (*AjaxResponse, error) {
	// make a copy of the input
	reqCopy := make(map[string]string)
	for k, v := range request {
		reqCopy[k] = v
	}
	reqCopy[AjaxRequest_SERIAL] = strconv.Itoa(client.serial)
	client.serial++

	resp, err := client.http.NewRequest().
		SetResult(AjaxResponse{}).
		SetFormData(reqCopy).Post("/AjaxServlet")
	if err != nil {
		log.Errorf("Request %+v failed: %+v", request, err)
		// TODO do we have to return here or will the Result call always do something sane enough?
	}

	return resp.Result().(*AjaxResponse), err
}

func (client *Client) Close() {
	log.Infof("Stopping client for session %s", client.sessionId)
	client.stop <- true
	close(client.stop)
	client.pollWg.Wait()
	close(client.IncomingEvents)
	log.Infof("Client for session %s stopped", client.sessionId)
}
