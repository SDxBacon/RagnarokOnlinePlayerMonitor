package network

import (
	"fmt"
	"log"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/context"
)

type PacketCaptureService struct {
	ip                     string // IP address to filter packets
	port                   int    // Port number to filter packets
	ctx                    context.Context
	cancel                 context.CancelFunc
	connections            map[ConnectionKey]*Connection
	connCloseNotifyChannel chan *Connection
}

func NewPacketCaptureService(ip string, port int) *PacketCaptureService {
	ctx, cancel := context.WithCancel(context.Background())

	return &PacketCaptureService{
		ip:                     ip,
		port:                   port,
		ctx:                    ctx,
		cancel:                 cancel,
		connections:            make(map[ConnectionKey]*Connection),
		connCloseNotifyChannel: make(chan *Connection),
	}
}

func (pcs *PacketCaptureService) GetConnectionCloseNotifyChannel() chan *Connection {
	return pcs.connCloseNotifyChannel
}

func (pcs *PacketCaptureService) GetContext() context.Context {
	return pcs.ctx
}

// StopCapture terminates the packet capture service by canceling
// its associated context. This stops all ongoing packet capturing
// and monitoring operations.
func (pcs *PacketCaptureService) StopCapture() {
	pcs.cancel()
}

// StartCaptureAllInterfaces initiates packet capture on all available network interfaces except loopback.
// It performs the following steps for each non-loopback interface:
// 1. Opens the interface for live packet capture
// 2. Applies the configured BPF filter
// 3. Starts a goroutine to continuously capture packets
//
// The captured packets are sent to the packetReceivedChannel for processing.
// The capture can be stopped by canceling the context provided to the PacketCaptureService.
//
// This method runs asynchronously and does not block. Each interface capture runs in its own goroutine.
// If there are errors opening devices or setting filters, they will be logged as fatal errors.
func (pcs *PacketCaptureService) StartCaptureAllInterfaces() {
	// first, find all network interfaces with pcap library
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return
	}

	// build filter for packet capture
	filter := fmt.Sprintf("tcp and net %s and port %d", pcs.ip, pcs.port)
	fmt.Printf("[Network.StartCaptureAllInterfaces] build filter success: %s", filter)

	// then, iterate through all interfaces and capture packets
	for _, device := range devices {
		// fmt.Printf("Device %d: %s\n", index, device.Name)

		// if the interface is not valid, skip it
		if !IsValidInterface(device) {
			continue
		}

		go func() {
			// open the device for live capture
			handle, err := pcap.OpenLive(device.Name, 1600, true, pcap.BlockForever)
			if err != nil {
				log.Fatal("[Network.StartCaptureAllInterfaces] Unable to open network device:", err)
				return
			}
			defer handle.Close()

			// set the BPF filter
			err = handle.SetBPFFilter(filter)
			if err != nil {
				log.Fatal("[Network.StartCaptureAllInterfaces] Unable to set filter:", err)
				return
			}

			fmt.Printf("[Network.StartCaptureAllInterfaces] Start sniffing on interface: %s\n", device.Name)
			// start capturing packets
			packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
			packetSource.NoCopy = true

			for {
				select {
				case <-pcs.ctx.Done(): // listen for cancellation
					return
				case packet := <-packetSource.Packets():
					pcs.handlePacket(packet)
				}
			}
		}()
	}
}

func (pcs *PacketCaptureService) handlePacket(packet gopacket.Packet) {
	// extract IP layer
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return
	}
	ip, _ := ipLayer.(*layers.IPv4)

	// extract TCP layer
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return
	}
	tcp, _ := tcpLayer.(*layers.TCP)

	// if the direction of the packet is NOT incoming, ignoring
	if ip.SrcIP.String() != pcs.ip || tcp.SrcPort != layers.TCPPort(pcs.port) {
		return
	}

	// create Connection instance
	conn := &Connection{
		SrcIP:   ip.SrcIP,
		DstIP:   ip.DstIP,
		SrcPort: uint16(tcp.SrcPort),
		DstPort: uint16(tcp.DstPort),
	}
	key := conn.Key()

	// check if the connection is already in the map
	var existingConn *Connection

	if existing, exists := pcs.connections[key]; exists {
		existingConn = existing
	} else {
		// new connection
		conn.StartTime = time.Now()
		conn.LastSeen = time.Now()
		pcs.connections[key] = conn
		existingConn = conn

		fmt.Printf("[NEW CONNECTION] %s:%d -> %s:%d\n",
			conn.SrcIP, conn.SrcPort, conn.DstIP, conn.DstPort)
	}

	// update the last seen value of the existing connection
	existingConn.LastSeen = time.Now()

	// if the payload is not empty, recording it
	payload := tcp.Payload
	if len(payload) > 0 {
		// copy payload and append to the IncomingPackets slice
		data := make([]byte, len(payload))
		copy(data, payload)
		existingConn.IncomingData = append(existingConn.IncomingData, data)
	}

	// if the packet is a FIN or RST meaning the connection is about to close
	if tcp.FIN || tcp.RST {
		// TODO:
	}
}
