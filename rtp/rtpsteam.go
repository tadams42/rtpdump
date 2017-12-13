package rtp

import (
    "fmt"
    "time"
    "github.com/tadams42/rtpdump/util"
)

type RtpStream struct {

    // Public
    Ssrc uint32
    PayloadType int
    SrcIP, DstIP string
    SrcPort, DstPort uint
    StartTime, EndTime time.Time

    // Internal - improve
    FirstTimestamp uint32
    FirstSeq uint16
    Cycle uint
    CurSeq uint16

    // Calculated
    TotalExpectedPackets uint
    LostPackets uint
    MeanJitter float32
    MeanBandwidth float32
	OverflownPackets uint

    RtpPackets []*RtpPacket
}

func (r RtpStream) String() string {
  return fmt.Sprintf("%s - %s   0x%08X   %3d   %5d   %s:%d -> %s:%d",
    util.TimeToStr(r.StartTime),
    util.TimeToStr(r.EndTime),
    r.Ssrc,
    r.PayloadType,
    len(r.RtpPackets),
    r.SrcIP,
    r.SrcPort,
    r.DstIP,
    r.DstPort,
  )
}


//maybe check RTP timestamps as well???
func (r *RtpStream) AddPacket(rtp *RtpPacket) {
	
    if rtp.SequenceNumber > r.CurSeq {

//		fmt.Printf("%v", rtp.SequenceNumber)
//		fmt.Printf(" ")
//		fmt.Println(r.CurSeq)

		r.EndTime = rtp.ReceivedAt
		r.CurSeq = rtp.SequenceNumber
		r.TotalExpectedPackets = uint(r.CurSeq - r.FirstSeq) + r.OverflownPackets
		r.LostPackets = r.TotalExpectedPackets - uint(len(r.RtpPackets))

		r.RtpPackets = append(r.RtpPackets, rtp)
		
    } else if rtp.SequenceNumber < r.CurSeq {
		//fmt.Printf("%v", rtp.SequenceNumber)
		//fmt.Printf(" ")
		//fmt.Println(r.CurSeq)	
		if rtp.SequenceNumber < 200 && r.CurSeq > 65300 {
				//fmt.Printf("ulazim")
				r.EndTime = rtp.ReceivedAt
				r.TotalExpectedPackets = uint(r.CurSeq - r.FirstSeq)
				var until64k uint = uint(65535 - r.CurSeq)
				var fromZero uint= uint(rtp.SequenceNumber)
				r.FirstSeq = rtp.SequenceNumber
				r.CurSeq = rtp.SequenceNumber
				
				
				r.OverflownPackets = r.OverflownPackets + r.TotalExpectedPackets + until64k + fromZero
				r.TotalExpectedPackets = r.OverflownPackets 
				r.LostPackets = r.TotalExpectedPackets - uint(len(r.RtpPackets))
				
				r.RtpPackets = append(r.RtpPackets, rtp)
				
		}
	} else if rtp.SequenceNumber == r.CurSeq {
		return 
	}
	
	
}



