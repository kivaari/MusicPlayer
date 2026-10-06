package internal

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/gopxl/beep/wav"
)

const speakerRate = beep.SampleRate(44100)

var speakerOnce sync.Once

type Player struct {
	mu       sync.Mutex
	ctrl     *beep.Ctrl
	vol      *effects.Volume
	streamer beep.StreamSeekCloser
	file     *os.File
	volume   int
	done     chan struct{}
}

func New() *Player {
	speakerOnce.Do(func() {
		speaker.Init(speakerRate, speakerRate.N(time.Second/10))
	})
	return &Player{volume: 80}
}

func decode(path string, f *os.File) (beep.StreamSeekCloser, beep.Format, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "mp3":
		return mp3.Decode(f)
	case "wav":
		return wav.Decode(f)
	case "flac":
		return flac.Decode(f)
	case "ogg":
		return vorbis.Decode(f)
	default:
		return nil, beep.Format{}, fmt.Errorf("unsupported format: %s (mp3/wav/flac/ogg)", path)
	}
}

func gainToVolume(v int) float64 {
	if v <= 0 {
		return 0
	}
	if v >= 100 {
		return 0
	}
	return math.Log2(float64(v) / 100.0)
}

func (p *Player) PlayFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	s, format, err := decode(path, f)
	if err != nil {
		f.Close()
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer != nil {
		p.streamer.Close()
	}
	if p.file != nil {
		p.file.Close()
	}

	var streamer beep.Streamer = s
	if format.SampleRate != speakerRate {
		streamer = beep.Resample(3, format.SampleRate, speakerRate, s)
	}

	p.streamer = s
	p.file = f
	p.ctrl = &beep.Ctrl{Streamer: streamer}
	p.vol = &effects.Volume{
		Streamer: p.ctrl,
		Base:     2,
		Volume:   gainToVolume(p.volume),
		Silent:   p.volume == 0,
	}

	done := make(chan struct{})
	p.done = done

	speaker.Clear()
	speaker.Play(beep.Seq(p.vol, beep.Callback(func() {
		close(done)
	})))
	return nil
}

func (p *Player) Toggle() {
	speaker.Lock()
	defer speaker.Unlock()
	if p.ctrl == nil {
		return
	}
	p.ctrl.Paused = !p.ctrl.Paused
}

func (p *Player) IsPaused() bool {
	speaker.Lock()
	defer speaker.Unlock()
	if p.ctrl == nil {
		return false
	}
	return p.ctrl.Paused
}

func (p *Player) Stop() {
	speaker.Clear()
}

func (p *Player) Done() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return p.done
}

func (p *Player) Wait() {
	<-p.Done()
}

func (p *Player) SetVolume(v int) {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	p.mu.Lock()
	p.volume = v
	vol := p.vol
	p.mu.Unlock()

	if vol == nil {
		return
	}
	speaker.Lock()
	vol.Volume = gainToVolume(v)
	vol.Silent = v == 0
	speaker.Unlock()
}

func (p *Player) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

func (p *Player) Position() time.Duration {
	speaker.Lock()
	defer speaker.Unlock()
	if p.streamer == nil {
		return 0
	}
	return speakerRate.D(p.streamer.Position())
}

func (p *Player) Len() time.Duration {
	speaker.Lock()
	defer speaker.Unlock()
	if p.streamer == nil {
		return 0
	}
	if l := p.streamer.Len(); l >= 0 {
		return speakerRate.D(l)
	}
	return 0
}
