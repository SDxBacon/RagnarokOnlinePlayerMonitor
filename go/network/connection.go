package network

import (
	"net"
	"time"
)

type Payload = []byte

type ConnectionKey struct {
	SrcIP   string
	DstIP   string
	SrcPort uint16
	DstPort uint16
}

type Connection struct {
	SrcIP        net.IP
	DstIP        net.IP
	SrcPort      uint16
	DstPort      uint16
	StartTime    time.Time
	LastSeen     time.Time
	IncomingData []Payload // array of incoming payload
	IsFinished   bool      // flag to indicate if the connection is finished
}

func (c *Connection) Key() ConnectionKey {
	return ConnectionKey{
		SrcIP:   c.SrcIP.String(),
		DstIP:   c.DstIP.String(),
		SrcPort: c.SrcPort,
		DstPort: c.DstPort,
	}
}

func (c *Connection) GetIncomingDataSortedByLength() []Payload {
	if len(c.IncomingData) == 0 {
		return nil
	}

	// Create a copy of the IncomingData slice
	sortedPackets := make([]Payload, len(c.IncomingData))
	copy(sortedPackets, c.IncomingData)

	// Sort the packets by their length in descending order
	for i := 0; i < len(sortedPackets)-1; i++ {
		for j := i + 1; j < len(sortedPackets); j++ {
			if len(sortedPackets[i]) < len(sortedPackets[j]) {
				sortedPackets[i], sortedPackets[j] = sortedPackets[j], sortedPackets[i]
			}
		}
	}

	return sortedPackets
}
