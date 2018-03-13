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
	"sync"
	"time"
)

// FIXME config
const PyxBaseUrl = "http://loopback.pretendyoure.xyz:8080/"

var globalChatEnabledRegex = regexp.MustCompile("cah.GLOBAL_CHAT_ENABLED = (true|false);")

type Client struct {
	GlobalChatEnabled bool
	IncomingEvents    chan map[string]interface{}
	stop              chan bool
	pollWg            sync.WaitGroup
	http              *resty.Client
	sessionId         string
	serial            int
}

func NewClient() *Client {
	client := &Client{
		IncomingEvents: make(chan map[string]interface{}),
		stop:           make(chan bool),
		http:           resty.New(),
	}

	client.http.
		SetDebug(true).
		SetHeader("User-Agent", "PYX-IRC").
		SetHostURL(PyxBaseUrl).
		SetRetryCount(3).
		SetTimeout(time.Duration(1 * time.Minute))

	return client
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
				log.Errorf("Long poll for session %s received error: %v", client.sessionId, err)
				// order matters here!
				client.pollWg.Done()
				client.Close()
				return
			}

			var res interface{}
			err = json.Unmarshal(resp.Body(), &res)

			log.Debugf("Result: %v", res)
			switch v := res.(type) {
			case map[string]interface{}:
				// bare object, likely an error or no-op
				singleResult := v
				err = checkForError(singleResult, err)
				if err != nil {
					log.Errorf("Long poll for session %s received error: %v", client.sessionId, err)
					// order matters here!
					client.pollWg.Done()
					client.Close()
					return
				}
				client.dispatchSinglePyxEvent(singleResult)
			case []interface{}:
				// array of objects, so can't be an error
				for _, event := range v {
					client.dispatchSinglePyxEvent(event.(map[string]interface{}))
				}
			default:
				log.Errorf("No idea what the type of this is: %v", res)
			}
		}
	}
}

func (client *Client) dispatchSinglePyxEvent(event map[string]interface{}) {
	// TODO handle the NOOP event here?
	log.Debugf("Received long poll for session %s: %v", client.sessionId, event)
	client.IncomingEvents <- event
}

// Make initial contact with PYX and obtain a session. Obtain server configuration information.
// Does not log in. Logging in should be done within half a minute of this call so that the session
// does not expire.
func (client *Client) Prepare() error {
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
	inProg, ok := flResp[AjaxResponse_IN_PROGRESS]
	if ok && inProg.(bool) {
		return fmt.Errorf("Session %s already in progress, not yet implemented (next=%s)",
			client.sessionId, flResp[AjaxResponse_NEXT])
	}

	return nil
}

// Log in to the server and start the long poll goroutine
func (client *Client) Login(nick string, idcode string) error {
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

	go client.receive()

	return nil
}

// Make the request on the server, and check for PYX application errors.
func (client *Client) Send(request map[string]string) (map[string]interface{}, error) {
	resp, err := client.sendNoErrorCheck(request)
	return resp, checkForError(resp, err)
}

// Check for an error condition in a server response. If the passed in reqError is not nil, that is
// returned directly. Otherwise, if the ERROR field in response is true, an error containing the
// ErrorCodeMsg for the ERROR_CODE is returned. If neither of these are true, then nil is returned.
func checkForError(response map[string]interface{}, reqError error) error {
	if reqError != nil {
		return reqError
	}
	errVal, ok := response[AjaxResponse_ERROR]
	if ok && errVal.(bool) {
		return fmt.Errorf("PYX error: %s",
			ErrorCodeMsgs[response[AjaxResponse_ERROR_CODE].(string)])
	}
	return nil
}

func (client *Client) sendNoErrorCheck(request map[string]string) (map[string]interface{}, error) {
	// make a copy of the input
	reqCopy := make(map[string]string)
	for k, v := range request {
		reqCopy[k] = v
	}
	reqCopy[AjaxRequest_SERIAL] = strconv.Itoa(client.serial)
	client.serial++

	resp, err := client.http.NewRequest().
		SetResult(map[string]interface{}{}).
		SetFormData(reqCopy).Post("/AjaxServlet")
	if err != nil {
		log.Errorf("Request %v failed: %v", request, err)
		// TODO do we have to return here or will the Result call always do something sane enough?
	}

	return *(resp.Result().(*map[string]interface{})), err
}

func (client *Client) Close() {
	log.Infof("Stopping client for session %s", client.sessionId)
	client.stop <- true
	close(client.stop)
	client.pollWg.Wait()
	close(client.IncomingEvents)
}
