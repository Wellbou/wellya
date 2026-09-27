package api

import (
	"encoding/json"
	"testing"
)

func TestFlexBool(t *testing.T) {
	cases := map[string]bool{
		`true`: true, `false`: false, `1`: true, `0`: false,
		`"true"`: true, `"false"`: false, `"1"`: true, `"0"`: false,
		`null`: false, `""`: false, `2`: true, `0.0`: false,
	}
	for in, want := range cases {
		var v struct {
			B FlexBool `json:"b"`
		}
		if err := json.Unmarshal([]byte(`{"b":`+in+`}`), &v); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if bool(v.B) != want {
			t.Errorf("%s: got %v want %v", in, v.B, want)
		}
	}
}

func TestTrackDecodesNumericBools(t *testing.T) {
	var tr Track
	raw := `{"id":123,"available":1,"lyricsAvailable":0,"lyricsInfo":{"hasAvailableSyncLyrics":"1"}}`
	if err := json.Unmarshal([]byte(raw), &tr); err != nil {
		t.Fatal(err)
	}
	if !tr.Available || tr.LyricsAvailable || !tr.LyricsInfo.HasAvailableSyncLyrics {
		t.Fatalf("unexpected decode: %+v", tr)
	}
}
