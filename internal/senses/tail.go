package senses

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// TailSensor is the file-content sense: it follows the files it is
// given and emits each appended line as an observation on the reading
// channel, prefixed with the file's base name so different files keep
// distinct shapes. It reads from the end, because the past of a log is
// old news; a truncated file (rotation) is picked up again from the
// top. The lines are already symbolic, so no transducer stands between
// the file and the mind.
const (
	tailChannel  = "reading"
	tailInterval = 2 * time.Second
	tailLineCap  = 240
)

func TailSensor(paths []string, out chan Observation, stop <-chan struct{}, dropped *atomic.Int64) {
	for _, path := range paths {
		go tailOne(path, out, stop, dropped)
	}
}

func tailOne(path string, out chan Observation, stop <-chan struct{}, dropped *atomic.Int64) {
	base := filepath.Base(path)
	var pos int64 = -1 // -1 means start from the current end
	tick := time.NewTicker(tailInterval)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			pos = tailRead(path, base, pos, out, dropped)
		}
	}
}

// tailRead emits the lines appended since the last position and returns
// the new position. A file that cannot be opened keeps the sensor
// waiting rather than dead, and a shrink (rotation or truncation) reads
// again from the top.
func tailRead(path, base string, pos int64, out chan Observation, dropped *atomic.Int64) int64 {
	f, err := os.Open(path)
	if err != nil {
		return pos
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return pos
	}
	size := st.Size()
	if pos < 0 || size < pos {
		if pos < 0 {
			return size // first sight: the existing content is old news
		}
		pos = 0 // truncated or rotated: start over
	}
	if size == pos {
		return pos
	}
	if _, err := f.Seek(pos, io.SeekStart); err != nil {
		return pos
	}
	r := bufio.NewReader(f)
	read := pos
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			// A partial last line waits for its newline.
			return read
		}
		read += int64(len(line))
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > tailLineCap {
			line = line[:tailLineCap]
		}
		emit(out, dropped, Observation{Channel: tailChannel, Content: base + ": " + line, At: time.Now()})
	}
}
