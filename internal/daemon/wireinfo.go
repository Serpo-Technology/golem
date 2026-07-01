package daemon

import (
	"golem/internal/limbic"
	"golem/internal/memory"
	"golem/wire"
)

// The wire types are the daemon's public protocol and carry no dependency on the
// store or the limbic system, so a separate client can import them. The
// conversion from those internal shapes into the wire form lives here, the one
// place the daemon translates its memory and its chemistry for the socket.

// fragmentInfo renders a stored fragment into its wire form, the display shape
// the socket carries.
func fragmentInfo(f *memory.Fragment) *wire.FragmentInfo {
	info := &wire.FragmentInfo{
		ID: f.ID, Channel: f.Channel, Content: f.Content, Template: f.Template,
		Surprise: f.Surprise, Limbic: chemWire(f.Limbic), AtUnix: f.Time.Unix(), Weight: f.Weight,
		Lang: f.Lang, Category: f.Category, Edited: f.Edited,
	}
	if f.Count > 1 {
		info.Count = f.Count
	}
	if f.Kind != "" && f.Kind != memory.KindEpisodic {
		info.Kind = f.Kind
	}
	return info
}

// attachMedia fills in the raw percept behind each info that kept one, so a recalled
// percept can show what was actually seen and not only how it was described. Most
// fragments kept nothing, so the lookup is skipped for all but the handful a reply
// carries.
func (d *Daemon) attachMedia(infos []*wire.FragmentInfo) {
	for _, info := range infos {
		if hash, mime, ok := d.store.Media(info.ID); ok {
			info.Media, info.Mime = hash, mime
		}
	}
}

// chemWire mirrors the limbic state into its wire form, the levels a client
// reads.
func chemWire(s limbic.State) wire.Chem {
	return wire.Chem{
		Dopamine:      s.Dopamine,
		Noradrenaline: s.Noradrenaline,
		Serotonin:     s.Serotonin,
		Oxytocin:      s.Oxytocin,
		Adenosine:     s.Adenosine,
	}
}
