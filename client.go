package mpv

import (
	"encoding/json"
	"errors"
	"fmt"
	//log "github.com/golang/glog"
	"strconv"
)

// Client is a more comfortable higher level interface
// to LLClient. It can use any LLClient implementation.
type Client struct {
	LLClient
}

// NewClient creates a new highlevel client based on a lowlevel client.
func NewClient(llclient LLClient) *Client {
	return &Client{
		llclient,
	}
}

// Mode options for Loadfile
const (
	LoadFileModeReplace    = "replace"
	LoadFileModeAppend     = "append"
	LoadFileModeAppendPlay = "append-play" // Starts if nothing is playing
)

// Loadfile loads a file, it either replaces the currently playing file (LoadFileModeReplace),
// appends to the current playlist (LoadFileModeAppend) or appends to playlist and plays if
// nothing is playing right now (LoadFileModeAppendPlay)
func (c *Client) Loadfile(path string, mode string) error {
	_, err := c.Exec("loadfile", path, mode)
	return err
}

type SubFlag string

const (
	// Select the subtitle immediately.
	Select SubFlag = "select"
	// Don't select the subtitle.
	// (Or in some special situations,
	// let the default stream selection mechanism decide.)
	Auto SubFlag = "auto"
	// Select the subtitle. If a subtitle with the same filename was
	// already added, that one is selected, instead of loading a
	// duplicate entry. (In this case, title/language are ignored,
	// and if the was changed since it was loaded, these
	// changes won't be reflected.)
	Cached SubFlag = "cached"
	// The title argument sets the track title in the UI.
	Title SubFlag = "title"
	// The lang argument sets the track language, and
	// can also influence stream selection with flags set
	// to auto.
	Lang SubFlag = "lang"
)

// Load the given subtitle file. It is selected as current subtitle after loading.
// The flags args is one of the following values:
func (c *Client) SubAdd(file string, flags ...SubFlag) error {
	var argv []interface{}
	argv = append(argv, "sub-add")
	argv = append(argv, file)
	for _, v := range flags {
		argv = append(argv, string(v))
	}
	res, err := c.Exec(argv...)
	if res == nil {
		return err
	}
	return err
}

// Remove the given subtitle track. If the id argument is missing,
// remove the current track. (Works on external subtitle files only.)
func (c *Client) SubRemove(id string) error {
	res, err := c.Exec("sub-remove", id)
	if res == nil {
		return err
	}
	return err
}

// Display OSD menu
func (c *Client) SetOSD(ok bool) error {
	var err error
	var res *Response
	if ok {
		res, err = c.Exec("osc")
	} else {
		res, err = c.Exec("no-osc")
	}
	if res == nil {
		return err
	}
	return err
}

type VideoTrack struct {
	DemuxW   int     `json:"demux-w"`
	DemuxH   int     `json:"demux-h"`
	DemuxFPS float64 `json:"demux-fps"`
}

type AudioTrack struct {
	Channels      int    `json:"audio-channels"`
	ChannelCount  int    `json:"demux-channel-count"`
	DemuxChannels string `json:"demux-channels"`
	SampleRate    int    `json:"demux-samplerate"`
}

type Track struct {
	ID          int    `json:"id"`   // unique wihtin Type
	Type        string `json:"type"` // e.g. audio , video, sub
	SrcID       int    `json:"src-id"`
	Title       string `json:"title"`
	Lang        string `json:"lang"`
	Albumart    bool   `json:"albumart"`
	Default     bool   `json:"default"`
	Forced      bool   `json:"forced"`
	External    bool   `json:"external"`
	Selected    bool   `json:"selected"`
	FFIndex     int    `json:"ff-index"`
	DecoderDesc string `json:"decoder-desc"`
	Codec       string `json:"codec"`
	Filename    string `json:"external-filename"`

	AudioTrack
	VideoTrack
}

// List of audio/video/sub tracks, current entry marked.
// Currently, the raw property value is useless.
// This has a number of sub-properties.
func (c *Client) TrackList() ([]Track, error) {
	res, err := c.Exec("get_property", "track-list")
	if res == nil {
		return nil, err
	}
	//log.Errorf("Data %s", string(res.Data))
	var ta []Track
	if err = json.Unmarshal([]byte(res.Data), &ta); err != nil {
		return nil, fmt.Errorf("data %s, err %v", res.Data, err)
	}
	return ta, nil
}

// While playback is active, you can set existing tracks only.
// (The option allows setting any track ID,
// and which tracks to enable is chosen at loading time.)
func (c *Client) SetAudioTrack(ID int) error {
	return c.SetProperty("aid", ID)
}

func (c *Client) SetVideoTrack(ID int) error {
	return c.SetProperty("vid", ID)
}

// subtitle track
func (c *Client) SetTextTrack(ID int) error {
	//vid, aid, sid
	return c.SetProperty("sid", ID)
}

// Mode options for Seek
const (
	SeekModeRelative = "relative"
	SeekModeAbsolute = "absolute"
)

// Seek seeks to a position in the current file.
// Use mode to seek relative to current position (SeekModeRelative) or absolute (SeekModeAbsolute).
func (c *Client) Seek(n int, mode string) error {
	_, err := c.Exec("seek", strconv.Itoa(n), mode)
	return err
}

// PlaylistNext plays the next playlistitem or NOP if no item is available.
func (c *Client) PlaylistNext() error {
	_, err := c.Exec("playlist-next", "weak")
	return err
}

// PlaylistPrevious plays the previous playlistitem or NOP if no item is available.
func (c *Client) PlaylistPrevious() error {
	_, err := c.Exec("playlist-prev", "weak")
	return err
}

// Mode options for LoadList
const (
	LoadListModeReplace = "replace"
	LoadListModeAppend  = "append"
)

// LoadList loads a playlist from path. It can either replace the current playlist (LoadListModeReplace)
// or append to the current playlist (LoadListModeAppend).
func (c *Client) LoadList(path string, mode string) error {
	_, err := c.Exec("loadlist", path, mode)
	return err
}

// GetProperty reads a property by name and returns the data as a string.
func (c *Client) GetProperty(name string) (string, error) {
	res, err := c.Exec("get_property", name)
	if res == nil {
		return "", err
	}
	return fmt.Sprintf("%#v", res.Data), err
}

// SetProperty sets the value of a property.
func (c *Client) SetProperty(name string, value interface{}) error {
	_, err := c.Exec("set_property", name, value)
	return err
}

// ErrInvalidType is returned if the response data does not match the methods return type.
// Use GetProperty or find matching type in mpv docs.
var ErrInvalidType = errors.New("Invalid type")

// GetFloatProperty reads a float property and returns the data as a float64.
func (c *Client) GetFloatProperty(name string) (float64, error) {
	res, err := c.Exec("get_property", name)
	if res == nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(string(res.Data), 64)
	if err == nil {
		return v, nil
	}
	return 0, ErrInvalidType
}

// GetBoolProperty reads a bool property and returns the data as a boolean.
func (c *Client) GetBoolProperty(name string) (bool, error) {
	res, err := c.Exec("get_property", name)
	if res == nil {
		return false, err
	}
	v, err := strconv.ParseBool(string(res.Data))
	if err == nil {
		return v, nil
	}
	return false, ErrInvalidType
}

// Filename returns the currently playing filename
func (c *Client) Filename() (string, error) {
	return c.GetProperty("filename")
}

// Path returns the currently playing path
func (c *Client) Path() (string, error) {
	return c.GetProperty("path")
}

// Pause returns true if the player is paused
func (c *Client) Pause() (bool, error) {
	return c.GetBoolProperty("pause")
}

// property - e.g. pause
func (c *Client) Cycle(property string) error {
	res, err := c.Exec("cycle", property)
	if res == nil {
		return err
	}
	return err
}

// SetPause pauses or unpauses the player
func (c *Client) SetPause(pause bool) error {
	return c.SetProperty("pause", pause)
}

// Idle returns true if the player is idle
func (c *Client) Idle() (bool, error) {
	return c.GetBoolProperty("idle")
}

func (c *Client) PlaybackTime() (float64, error) {
	return c.GetFloatProperty("playback-time")
}

// Mute returns true if the player is muted.
func (c *Client) Mute() (bool, error) {
	return c.GetBoolProperty("mute")
}

// SetMute mutes or unmutes the player.
func (c *Client) SetMute(mute bool) error {
	return c.SetProperty("mute", mute)
}

// Fullscreen returns true if the player is in fullscreen mode.
func (c *Client) Fullscreen() (bool, error) {
	return c.GetBoolProperty("fullscreen")
}

// SetFullscreen activates/deactivates the fullscreen mode.
func (c *Client) SetFullscreen(v bool) error {
	return c.SetProperty("fullscreen", v)
}

// Volume returns the current volume level.
func (c *Client) Volume() (float64, error) {
	return c.GetFloatProperty("volume")
}

// 1 increases the volume by 1 , -1 decreases the volume by 1
func (c *Client) SetVolumeGain(i int) error {
	v, err := c.GetFloatProperty("volume")
	if err != nil {
		return err
	}
	v = v + float64(i)
	return c.SetProperty("volume", v)
}

func (c *Client) SetVolume(i int) error {
	return c.SetProperty("volume", i)
}

// Speed returns the current playback speed.
func (c *Client) Speed() (float64, error) {
	return c.GetFloatProperty("speed")
}

// Duration returns the duration of the currently playing file.
func (c *Client) Duration() (float64, error) {
	return c.GetFloatProperty("duration")
}

// Position returns the current playback position in seconds.
func (c *Client) Position() (float64, error) {
	s, err := c.GetFloatProperty("time-pos")
	if err != nil {
		return 0, err
	}
	return s, err
}

// PercentPosition returns the current playback position in percent.
func (c *Client) PercentPosition() (float64, error) {
	return c.GetFloatProperty("percent-pos")
}

// Stop playback and clear playlist.
// With default settings, this is essentially like quit.
// Useful for the client API: playback can be stopped without
// terminating the player.
func (c *Client) Stop() error {
	res, err := c.Exec("stop")
	if res == nil {
		return err
	}
	return err
}

// Exit the player. If an argument is given,
// it's used as process exit code.
func (c *Client) Quit(code int) error {
	res, err := c.Exec("quit", code)
	if res == nil {
		return err
	}
	return err
}
