package webrtc

import (
	"testing"
	"time"

	"github.com/pion/interceptor"
	pion "github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
)

func TestNestRTXAttributeDetection(t *testing.T) {
	require.False(t, nestWasRTX(nil))

	attrs := make(interceptor.Attributes)
	require.False(t, nestWasRTX(attrs))

	attrs.Set(pion.AttributeRtxSsrc, uint32(7778))
	require.True(t, nestWasRTX(attrs))
}

func TestNestRTXGapHeadFilled(t *testing.T) {
	var reorder nestRTPReorder
	var diag nestRTXDiagnostics
	now := time.Unix(1, 0)

	diag.push(&reorder, nestTestPacket(100), now, false)
	diag.push(&reorder, nestTestPacket(102),
		now.Add(10*time.Millisecond), false)

	result := diag.push(&reorder, nestTestPacket(101),
		now.Add(80*time.Millisecond), true)

	require.Equal(t, []uint16{101, 102},
		nestPacketSeqs(result.packets))
	require.Equal(t, uint64(1), diag.rtxDelivered)
	require.Equal(t, uint64(1), diag.rtxGapHeadFilled)
	require.Zero(t, diag.rtpGapEvents)
}

func TestNestRTXAfterDeadline(t *testing.T) {
	var reorder nestRTPReorder
	var diag nestRTXDiagnostics
	now := time.Unix(1, 0)

	diag.push(&reorder, nestTestPacket(100), now, false)
	diag.push(&reorder, nestTestPacket(102),
		now.Add(10*time.Millisecond), false)

	result := diag.push(&reorder, nestTestPacket(101),
		now.Add(320*time.Millisecond), true)

	require.True(t, result.gapExpired)
	require.Equal(t, uint64(1), diag.rtxDelivered)
	require.Zero(t, diag.rtxGapHeadFilled)
	require.Equal(t, uint64(1), diag.rtxLateOrOld)
	require.Equal(t, uint64(1), diag.rtpGapEvents)
	require.Equal(t, uint64(1), diag.rtpMissingTotal)
}

func TestNestRTXDuplicatePending(t *testing.T) {
	var reorder nestRTPReorder
	var diag nestRTXDiagnostics
	now := time.Unix(1, 0)

	diag.push(&reorder, nestTestPacket(100), now, false)
	diag.push(&reorder, nestTestPacket(102),
		now.Add(10*time.Millisecond), false)

	result := diag.push(&reorder, nestTestPacket(102),
		now.Add(20*time.Millisecond), true)

	require.Empty(t, result.packets)
	require.Equal(t, uint64(1), diag.rtxDuplicatePending)
	require.Zero(t, diag.rtxGapHeadFilled)
}

func TestNestRTXWraparound(t *testing.T) {
	var reorder nestRTPReorder
	var diag nestRTXDiagnostics
	now := time.Unix(1, 0)

	diag.push(&reorder, nestTestPacket(65534), now, false)
	diag.push(&reorder, nestTestPacket(0),
		now.Add(10*time.Millisecond), false)

	result := diag.push(&reorder, nestTestPacket(65535),
		now.Add(20*time.Millisecond), true)

	require.Equal(t, []uint16{65535, 0},
		nestPacketSeqs(result.packets))
	require.Equal(t, uint64(1), diag.rtxGapHeadFilled)
}

func TestNestRTXInOrderNotCountedAsGapRecovery(t *testing.T) {
	var reorder nestRTPReorder
	var diag nestRTXDiagnostics
	now := time.Unix(1, 0)

	diag.push(&reorder, nestTestPacket(100), now, false)
	diag.push(&reorder, nestTestPacket(101),
		now.Add(time.Millisecond), true)

	require.Zero(t, diag.rtxGapHeadFilled)
	require.Equal(t, uint64(1), diag.rtxOther)
}
