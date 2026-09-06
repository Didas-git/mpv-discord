package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/Didas-git/mpv-discord/discordrpc"
	"github.com/Didas-git/mpv-discord/mpvrpc"
)

var (
	client   *mpvrpc.Client
	presence *discordrpc.Presence
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Lmsgprefix)

	client = mpvrpc.NewClient()
	presence = discordrpc.NewPresence(os.Args[2])
}

var currTime int64 = time.Now().UnixMilli()

func refreshCurrTime() {
	currTime = time.Now().UnixMilli()
}

func getActivity() (activity discordrpc.Activity, err error) {
	getProperty := func(key string) (prop interface{}) {
		prop, err = client.GetProperty(key)
		return
	}
	getPropertyString := func(key string) (prop string) {
		prop, err = client.GetPropertyString(key)
		return
	}
	getPropertyBool := func(key string) (prop bool) {
		prop, err = client.GetPropertyBool(key)
		return
	}

	activity.LargeImageKey = "mpv"
	activity.LargeImageText = "MPV Media Player"
	if version := getPropertyString("mpv-version"); version != "" {
		activity.LargeImageText = version[5:]
	}

	details := getPropertyString("media-title")

	if details == "" {
		return discordrpc.Activity{
			State:          "(Idle)",
			Details:        "Nothing Playing...",
			Type:           3,
			LargeImageKey:  "mpv",
			LargeImageText: activity.LargeImageText,
			SmallImageKey:  "stop",
			SmallImageText: "Idle",
			Timestamps: &discordrpc.ActivityTimestamps{
				Start: currTime,
				End:   0,
			},
		}, nil
	}

	metadata_title := getPropertyString("metadata/by-key/Title")
	metadata_artist := getPropertyString("metadata/by-key/Artist")
	metadata_album := getPropertyString("metadata/by-key/Album")

	if metadata_title != "" {
		details = metadata_title
	}
	if metadata_artist != "" {
		details += "\nby " + metadata_artist
	}
	if metadata_album != "" {
		details += "\non " + metadata_album
	}

	idle := getPropertyBool("idle-active")
	core_idle := getPropertyBool("core-idle")
	buffering := getPropertyBool("paused-for-cache")
	pause := getPropertyBool("pause")

	state := ""

	if idle {
		state = "(Idle)"
		activity.SmallImageKey = "stop"
		activity.SmallImageText = "Idle"
	} else if buffering {
		activity.SmallImageKey = "buffer"
		activity.SmallImageText = "Buffering"
	} else if pause {
		activity.SmallImageKey = "pause"
		activity.SmallImageText = "Paused"
	} else if !core_idle {
		activity.SmallImageKey = "play"
		activity.SmallImageText = "Playing"
	}

	if !idle {
		playlist := fmt.Sprintf(" - Playlist: [%s/%s]", getPropertyString("playlist-pos-1"), getPropertyString("playlist-count"))

		loop := ""

		loop_file := getPropertyBool("loop-file")
		loop_playlist := getPropertyBool("loop-playlist")

		if loop_file {
			if loop_playlist {
				loop = "File, Playlist"
			} else {
				loop = "File"
			}
		} else if loop_playlist {
			loop = "Playlist"
		} else {
			loop = "disabled"
		}

		loop = fmt.Sprintf(" - Loop: %s", loop)

		state += getPropertyString("options/term-status-msg")
		activity.SmallImageText = fmt.Sprintf("%s%s%s", activity.SmallImageText, playlist, loop)
	}

	activity.State = state
	activity.Details = details
	activity.Type = 3

	// Timestamps
	_duration := getProperty("duration")
	if _duration == nil {
		_duration = 0.0
	}
	durationMillis := int64(_duration.(float64))

	_timePos := getProperty("time-pos")
	if _timePos == nil {
		_timePos = 0.0
	}
	timePosMills := int64(_timePos.(float64))

	refreshCurrTime()
	startTimePos := currTime - (timePosMills * 1000)
	duration := startTimePos + (durationMillis * 1000)

	if !pause {
		activity.Timestamps = &discordrpc.ActivityTimestamps{
			Start: startTimePos,
			End:   duration,
		}
	}
	return
}

func openClient() {
	if err := client.Open(os.Args[1]); err != nil {
		log.Fatalln(err)
	}
	log.Println("(mpv-ipc): connected")
}

func openPresence() {
	// try until success
	for range time.Tick(500 * time.Millisecond) {
		if client.IsClosed() {
			return // stop trying when mpv shuts down
		}
		if err := presence.Open(); err == nil {
			break // break when successfully opened
		}
	}
	log.Println("(discord-ipc): connected")
}

func main() {
	defer func() {
		if !client.IsClosed() {
			if err := client.Close(); err != nil {
				log.Fatalln(err)
			}
			log.Println("(mpv-ipc): disconnected")
		}
		if !presence.IsClosed() {
			if err := presence.Close(); err != nil {
				log.Fatalln(err)
			}
			log.Println("(discord-ipc): disconnected")
		}
	}()

	openClient()
	go openPresence()

	for range time.Tick(time.Second) {
		activity, err := getActivity()
		if err != nil {
			if errors.Is(err, syscall.EPIPE) {
				break
			} else if !errors.Is(err, io.EOF) {
				log.Println(err)
				continue
			}
		}
		if !presence.IsClosed() {
			go func() {
				if err = presence.Update(activity); err != nil {
					if errors.Is(err, syscall.EPIPE) {
						// close it before retrying
						if err = presence.Close(); err != nil {
							log.Fatalln(err)
						}
						log.Println("(discord-ipc): reconnecting...")
						go openPresence()
					} else if !errors.Is(err, io.EOF) {
						log.Println(err)
					}
				}
			}()
		}
	}
}
