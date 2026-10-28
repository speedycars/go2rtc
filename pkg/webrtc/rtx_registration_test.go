package webrtc

import (
	"strings"
	"testing"

	pion "github.com/pion/webrtc/v4"
)

func TestRTXCodecsInVideoOffer(t *testing.T) {
	api, err := NewAPI()
	if err != nil {
		t.Fatal(err)
	}

	pc, err := api.NewPeerConnection(pion.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	_, err = pc.AddTransceiverFromKind(
		pion.RTPCodecTypeVideo,
		pion.RTPTransceiverInit{
			Direction: pion.RTPTransceiverDirectionRecvonly,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"a=rtpmap:96 H264/90000",
		"a=rtpmap:97 H264/90000",
		"a=rtpmap:98 H264/90000",

		"a=rtpmap:99 rtx/90000",
		"a=fmtp:99 apt=96",

		"a=rtpmap:102 rtx/90000",
		"a=fmtp:102 apt=97",

		"a=rtpmap:103 rtx/90000",
		"a=fmtp:103 apt=98",
	}

	sdp := strings.ToLower(offer.SDP)

	for _, line := range expected {
		if !strings.Contains(sdp, strings.ToLower(line)) {
			t.Errorf("Missing SDP line: %s", line)
		}
	}
}
