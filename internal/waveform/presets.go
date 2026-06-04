package waveform

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed presets.json
var presetsJSON []byte

// presetRaw は presets.json の 1 要素。DG-LAB-OPENSOURCE の
// DG_WAVES_V2_V3_simple.js と同一フォーマット。
type presetRaw struct {
	Name       string   `json:"name"`
	Raw        string   `json:"raw"`
	ExpectedV2 []string `json:"expectedV2"`
	ExpectedV3 []string `json:"expectedV3"`
}

// Preset は名前付きの波形プリセット。
type Preset struct {
	Name     string
	Waveform Waveform
}

var (
	presetsOnce sync.Once
	presets     []Preset
	presetByName map[string]Preset
	presetErr   error
)

func loadPresets() {
	var raws []presetRaw
	if err := json.Unmarshal(presetsJSON, &raws); err != nil {
		presetErr = fmt.Errorf("waveform: parse presets.json: %w", err)
		return
	}
	presetByName = make(map[string]Preset, len(raws))
	for _, r := range raws {
		w, err := HexFramesToWaveform(r.Name, r.ExpectedV3)
		if err != nil {
			presetErr = fmt.Errorf("waveform: preset %q: %w", r.Name, err)
			return
		}
		p := Preset{Name: r.Name, Waveform: w}
		presets = append(presets, p)
		presetByName[r.Name] = p
	}
}

// Presets は埋め込まれた全プリセットを定義順で返す。
func Presets() ([]Preset, error) {
	presetsOnce.Do(loadPresets)
	if presetErr != nil {
		return nil, presetErr
	}
	return presets, nil
}

// PresetNames は全プリセット名を定義順で返す。
func PresetNames() ([]string, error) {
	ps, err := Presets()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	return names, nil
}

// PresetByName は名前でプリセットを引く。見つからなければ ok=false。
func PresetByName(name string) (Preset, bool) {
	if _, err := Presets(); err != nil {
		return Preset{}, false
	}
	p, ok := presetByName[name]
	return p, ok
}
