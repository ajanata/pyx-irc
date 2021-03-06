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

const NoGameIdSentinel = -1

var globalChatEnabledRegex = regexp.MustCompile("cah.GLOBAL_CHAT_ENABLED = (true|false);")
var broadcastingUsersRegex = regexp.MustCompile("cah.BROADCASTING_USERS = (true|false);")

type Client struct {
	BroadcastingUsers bool
	GlobalChatEnabled bool
	IncomingEvents    chan *LongPollResponse
	ServerStarted     int64
	User              *User
	stop              chan bool
	stopped           bool
	pollWg            sync.WaitGroup
	http              *resty.Client
	sessionId         string
	serial            int
	config            *Config
}

func NewClient(nick string, idcode string, config *Config) (*Client, error) {
	client := &Client{
		IncomingEvents: make(chan *LongPollResponse),
		stop:           make(chan bool, 1),
		http:           resty.New(),
		config:         config,
	}

	client.http.
		SetHeader("User-Agent", "PYX-IRC").
		SetHostURL(config.BaseAddress).
		SetRetryCount(3).
		SetTimeout(time.Duration(1 * time.Minute))
	if config.HttpDebug {
		client.http.SetDebug(true)
	}

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
			if !strings.HasPrefix(resp.Header().Get("Content-Type"), "application/json") {
				// probably an error of some description
				log.Errorf("Didn't get JSON response for long poll for session %s, body: %s",
					client.sessionId, resp.String())
				// order matters here!
				client.pollWg.Done()
				client.Close()
				return
			}
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
	matches = broadcastingUsersRegex.FindStringSubmatch(resp.String())
	if len(matches) > 1 {
		client.BroadcastingUsers, _ = strconv.ParseBool(matches[1])
	}

	flResp, err := client.send(map[string]string{
		AjaxRequest_OP: AjaxOperation_FIRST_LOAD,
	})
	if err != nil {
		return err
	}
	if flResp.InProgress {
		return fmt.Errorf("Session %s already in progress, not yet implemented (next=%s)",
			client.sessionId, flResp.Next)
	}
	client.ServerStarted = flResp.ServerStarted
	// TODO save the card sets somewhere
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
	resp, err := client.send(req)
	err = checkForError(resp, err)
	if err != nil {
		return err
	}

	client.User = newUser(resp.Nickname, resp.Sigil, resp.IdCode)

	go client.receive()

	return nil
}

func (client *Client) Names() ([]string, error) {
	resp, err := client.send(map[string]string{
		AjaxRequest_OP: AjaxOperation_NAMES,
	})
	if err != nil {
		return []string{}, err
	}
	return resp.Names, nil
}

func (client *Client) SendGlobalChat(msg string, emote bool) error {
	return client.sendChat(msg, emote, false, NoGameIdSentinel)
}

func (client *Client) SendGameChat(gameId int, msg string, emote bool) error {
	return client.sendChat(msg, emote, false, gameId)
}

func (client *Client) sendChat(msg string, emote bool, wall bool, gameId int) error {
	req := map[string]string{
		AjaxRequest_OP:      AjaxOperation_CHAT,
		AjaxRequest_MESSAGE: msg,
	}
	if emote {
		req[AjaxRequest_EMOTE] = "true"
	}
	if wall {
		req[AjaxRequest_WALL] = "true"
	}
	if gameId >= 0 {
		req[AjaxRequest_OP] = AjaxOperation_GAME_CHAT
		req[AjaxRequest_GAME_ID] = strconv.Itoa(gameId)
	}

	_, err := client.send(req)
	return err
}

func (client *Client) Whois(nick string) (*AjaxResponse, error) {
	return client.send(map[string]string{
		AjaxRequest_OP:       AjaxOperation_WHOIS,
		AjaxRequest_NICKNAME: nick,
	})
}

func (client *Client) GameList() (*AjaxResponse, error) {
	return client.send(map[string]string{
		AjaxRequest_OP: AjaxOperation_GAME_LIST,
	})
}

func (client *Client) GameInfo(gameId int) (*AjaxResponse, error) {
	return client.send(map[string]string{
		AjaxRequest_OP:      AjaxOperation_GET_GAME_INFO,
		AjaxRequest_GAME_ID: strconv.Itoa(gameId),
	})
}

func (client *Client) LogOut() {
	// disregard result since we're throwing the user away anyway
	client.send(map[string]string{
		AjaxRequest_OP: AjaxOperation_LOG_OUT,
	})
	client.Close()
}

func (client *Client) LeaveGame(gameId int) (*AjaxResponse, error) {
	return client.send(map[string]string{
		AjaxRequest_OP:      AjaxOperation_LEAVE_GAME,
		AjaxRequest_GAME_ID: strconv.Itoa(gameId),
	})
}

func (client *Client) SpectateGame(gameId int, password string) (*AjaxResponse, error) {
	return client.send(map[string]string{
		AjaxRequest_OP:       AjaxOperation_SPECTATE_GAME,
		AjaxRequest_GAME_ID:  strconv.Itoa(gameId),
		AjaxRequest_PASSWORD: password,
	})
}

func (client *Client) JoinGame(gameId int, password string) (*AjaxResponse, error) {
	return client.send(map[string]string{
		AjaxRequest_OP:       AjaxOperation_JOIN_GAME,
		AjaxRequest_GAME_ID:  strconv.Itoa(gameId),
		AjaxRequest_PASSWORD: password,
	})
}

// Make the request on the server, and check for PYX application errors.
func (client *Client) send(request map[string]string) (*AjaxResponse, error) {
	resp, err := client.sendNoErrorCheck(request)
	return resp, checkForError(resp, err)
}

// Check for an error condition in a server response. If the passed in reqError is not nil, that is
// returned directly. Otherwise, if the ERROR field in response is true, an error containing the
// ErrorCodeMsg for the ERROR_CODE is returned. If neither of these are true, then nil is returned.
func checkForError(response *AjaxResponse, reqError error) error {
	if reqError != nil {
		log.Errorf("Request error: %s", reqError)
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
		log.Errorf("Request error: %s", reqError)
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
	// make sure we only do this once, got a panic in an edge case before
	if client.stopped {
		return
	}
	client.stopped = true
	log.Infof("Stopping client for session %s", client.sessionId)
	client.stop <- true
	close(client.stop)
	client.pollWg.Wait()
	close(client.IncomingEvents)
	log.Infof("Client for session %s stopped", client.sessionId)
}
