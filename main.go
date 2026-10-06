package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"music-player/internal"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: music-player <file.mp3|file.wav|file.flac|file.ogg>")
	}

	p := internal.New()
	if err := p.PlayFile(os.Args[1]); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Playing:", os.Args[1])

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	done := p.Done()
	for {
		select {
		case <-done:
			return
		case t := <-tick.C:
			_ = t
			fmt.Println(p.Position().Round(time.Second), "/", p.Len().Round(time.Second))
		}
	}
}
