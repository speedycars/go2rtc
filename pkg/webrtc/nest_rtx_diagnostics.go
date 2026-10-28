package webrtc

import (
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	pion "github.com/pion/webrtc/v4"
	zlog "github.com/rs/zerolog/log"
)

// Session-local counters, updated only by the Nest video read goroutine.
type nestRTXDiagnostics struct {
	rtxDelivered        uint64
	rtxGapHeadFilled    uint64
	rtxLateOrOld        uint64
	rtxDuplicatePending uint64
	rtxOther            uint64

	rtpGapEvents    uint64
	rtpMissingTotal uint64
}

func nestWasRTX(attrs interceptor.Attributes) bool {
	return attrs != nil &&
		attrs.Get(pion.AttributeRtxSsrc) != nil
}

func (d *nestRTXDiagnostics) push(
	reorder *nestRTPReorder,
	packet *rtp.Packet,
	now time.Time,
	isRTX bool,
) nestRTPReorderResult {

	// Inspect the state BEFORE push(), without changing the reorderer.
	openGap := reorder.started &&
		!reorder.gapSince.IsZero() &&
		len(reorder.pending) > 0

	atHead := openGap &&
		packet.SequenceNumber == reorder.next

	withinWindow := openGap &&
		now.Sub(reorder.gapSince) < nestRTPReorderMaxWait

	behindBefore := reorder.started &&
		int16(packet.SequenceNumber-reorder.next) < 0

	_, duplicatePending := reorder.pending[packet.SequenceNumber]

	result := reorder.push(packet, now)

	if isRTX {
		d.rtxDelivered++

		switch {
		case atHead && withinWindow &&
			!result.gapExpired &&
			len(result.packets) > 0 &&
			result.packets[0].SequenceNumber == packet.SequenceNumber:

			// RTX closed the missing playout head before its deadline.
			d.rtxGapHeadFilled++

		case (atHead && !withinWindow && result.gapExpired) ||
			(behindBefore && !result.reanchored):

			// Late RTX or retransmission of an already-consumed packet.
			d.rtxLateOrOld++

		case duplicatePending:
			d.rtxDuplicatePending++

		default:
			// Includes RTX accepted ahead of the current head.
			// These packets may still be useful later.
			d.rtxOther++
		}
	}

	if result.gapExpired {
		d.rtpGapEvents++
		d.rtpMissingTotal += uint64(result.missing)
	}

	return result
}

func (d *nestRTXDiagnostics) log(
	connID int64,
	elapsed time.Duration,
	final bool,
) {
	zlog.Info().
		Int64("conn", connID).
		Dur("elapsed", elapsed).
		Bool("final", final).
		Uint64("rtx_delivered", d.rtxDelivered).
		Uint64("rtx_gap_head_filled", d.rtxGapHeadFilled).
		Uint64("rtx_late_or_old", d.rtxLateOrOld).
		Uint64("rtx_duplicate_pending", d.rtxDuplicatePending).
		Uint64("rtx_other", d.rtxOther).
		Uint64("rtp_gap_events", d.rtpGapEvents).
		Uint64("rtp_missing_total", d.rtpMissingTotal).
		Msg("nest: RTX diagnostics")
}
