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
	"github.com/ajanata/pyx-irc/pyx"
)

type Config struct {
	BindAddress               string `toml:"bind_address"`
	Port                      int
	AdvertisedName            string `toml:"advertised_name"`
	NetworkName               string `toml:"network_name"`
	BotNick                   string `toml:"bot_nick"`
	BotUsername               string `toml:"bot_username"`
	BotHostname               string `toml:"bot_hostname"`
	UserHostname              string `toml:"user_hostname"`
	GlobalChannel             string `toml:"global_channel"`
	GameChannelPrefix         string `toml:"game_channel_prefix"`
	SpectateGameChannelPrefix string `toml:"spectate_game_channel_prefix"`
	Pyx                       pyx.Config
}

func (config *Config) EnsureDefaults() {
	if config.BindAddress == "" {
		config.BindAddress = "0.0.0.0"
	}
	if config.Port == 0 {
		config.Port = 6667
	}
	if config.AdvertisedName == "" {
		config.AdvertisedName = "localhost"
	}
	if config.NetworkName == "" {
		config.NetworkName = "PYX"
	}
	if config.BotNick == "" {
		config.BotNick = "Xyzzy"
	}
	if config.BotUsername == "" {
		config.BotUsername = "xyzzy"
	}
	if config.BotHostname == "" {
		config.BotHostname = "localhost"
	}
	if config.UserHostname == "" {
		config.UserHostname = "users.localhost"
	}
	if config.GlobalChannel == "" {
		config.GlobalChannel = "#global"
	}
	if config.GameChannelPrefix == "" {
		config.GameChannelPrefix = "#game-"
	}
	if config.SpectateGameChannelPrefix == "" {
		config.SpectateGameChannelPrefix = "#watch-"
	}
	config.Pyx.EnsureDefaults()
}
