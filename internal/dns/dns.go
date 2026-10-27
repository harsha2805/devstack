// Package dns is a tiny DNS responder for local site names: every name under
// the stack's TLD (*.test) resolves to loopback, and anything else is
// refused. Windows sends it only that TLD's lookups, through a Name
// Resolution Policy Table rule (see rule_windows.go), so it never sees or
// affects real domains. Entries in the hosts file still win, as always.
package dns

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Addr is where the responder listens and what the NRPT rule points at.
const Addr = "127.0.0.1:53"

const (
	typeA    = 1
	typeAAAA = 28
	classIN  = 1

	rcodeOK      = 0
	rcodeFormErr = 1
	rcodeNotImp  = 4
	rcodeRefused = 5

	ttl = 30 // seconds; short, since the names only work while devstack runs
)

type zone struct {
	tld  string // "test"
	ipv6 bool   // answer AAAA with ::1 (nginx listens there too)
}

// Server answers on UDP and TCP.
type Server struct {
	zone atomic.Pointer[zone]
	udp  net.PacketConn
	tcp  net.Listener
	wg   sync.WaitGroup
}

// Listen starts answering for *.tld on addr.
func Listen(addr, tld string, ipv6 bool) (*Server, error) {
	s := &Server{}
	s.Set(tld, ipv6)
	var err error
	if s.udp, err = net.ListenPacket("udp", addr); err != nil {
		return nil, err
	}
	if s.tcp, err = net.Listen("tcp", addr); err != nil {
		s.udp.Close()
		return nil, err
	}
	s.wg.Add(2)
	go s.serveUDP()
	go s.serveTCP()
	return s, nil
}

// Set changes the zone without restarting (a reload with a new tld).
func (s *Server) Set(tld string, ipv6 bool) {
	s.zone.Store(&zone{tld: strings.ToLower(strings.Trim(tld, ".")), ipv6: ipv6})
}

// Close stops listening and waits for the loops to end.
func (s *Server) Close() error {
	err := errors.Join(s.udp.Close(), s.tcp.Close())
	s.wg.Wait()
	return err
}

func (s *Server) serveUDP() {
	defer s.wg.Done()
	buf := make([]byte, 1500)
	for {
		n, from, err := s.udp.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue // e.g. WSAECONNRESET after an ICMP port-unreachable
		}
		if resp := Answer(buf[:n], s.zone.Load()); resp != nil {
			s.udp.WriteTo(resp, from)
		}
	}
}

func (s *Server) serveTCP() {
	defer s.wg.Done()
	for {
		c, err := s.tcp.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go s.handleTCP(c)
	}
}

func (s *Server) handleTCP(c net.Conn) {
	defer c.Close()
	for {
		c.SetDeadline(time.Now().Add(10 * time.Second))
		var n uint16
		if err := binary.Read(c, binary.BigEndian, &n); err != nil {
			return
		}
		msg := make([]byte, n)
		if _, err := io.ReadFull(c, msg); err != nil {
			return
		}
		resp := Answer(msg, s.zone.Load())
		if resp == nil {
			return
		}
		out := binary.BigEndian.AppendUint16(nil, uint16(len(resp)))
		if _, err := c.Write(append(out, resp...)); err != nil {
			return
		}
	}
}

// Answer builds the response to one query, or nil to drop it.
func Answer(q []byte, z *zone) []byte {
	if len(q) < 12 || q[2]&0x80 != 0 { // too short, or a response
		return nil
	}
	if opcode := (q[2] >> 3) & 0x0f; opcode != 0 {
		return reply(q, 12, rcodeNotImp, nil)
	}
	if binary.BigEndian.Uint16(q[4:]) != 1 {
		return reply(q, 12, rcodeFormErr, nil)
	}
	name, end, ok := readName(q, 12)
	if !ok || end+4 > len(q) {
		return reply(q, 12, rcodeFormErr, nil)
	}
	qtype := binary.BigEndian.Uint16(q[end:])
	qclass := binary.BigEndian.Uint16(q[end+2:])
	qend := end + 4

	if name != z.tld && !strings.HasSuffix(name, "."+z.tld) {
		return reply(q, qend, rcodeRefused, nil)
	}
	var rdata []byte
	switch {
	case qclass != classIN:
	case qtype == typeA:
		rdata = net.IPv4(127, 0, 0, 1).To4()
	case qtype == typeAAAA && z.ipv6:
		rdata = net.IPv6loopback
	}
	return reply(q, qend, rcodeOK, rdata) // no rdata: NOERROR with no answer (NODATA)
}

// readName reads an uncompressed question name, lowercased and without the
// trailing dot.
func readName(q []byte, off int) (string, int, bool) {
	var labels []string
	total := 0
	for {
		if off >= len(q) {
			return "", 0, false
		}
		l := int(q[off])
		off++
		if l == 0 {
			break
		}
		if l > 63 || off+l > len(q) { // 0xC0 pointers don't belong in a question
			return "", 0, false
		}
		total += l + 1
		if total > 255 {
			return "", 0, false
		}
		labels = append(labels, strings.ToLower(string(q[off:off+l])))
		off += l
	}
	return strings.Join(labels, "."), off, true
}

// reply copies the header and question (q[12:qend]) and adds one answer when
// rdata is set.
func reply(q []byte, qend, rcode int, rdata []byte) []byte {
	r := make([]byte, 0, qend+16+len(rdata))
	r = append(r, q[0], q[1])        // ID
	r = append(r, 0x84|q[2]&0x01, 0) // QR, AA, RD copied from the query
	r[3] = byte(rcode)               // RA off: we don't recurse
	qd := uint16(0)
	if qend > 12 {
		qd = 1
	}
	an := uint16(0)
	if rdata != nil {
		an = 1
	}
	r = binary.BigEndian.AppendUint16(r, qd)
	r = binary.BigEndian.AppendUint16(r, an)
	r = append(r, 0, 0, 0, 0) // NS, AR
	r = append(r, q[12:qend]...)
	if rdata != nil {
		r = append(r, 0xc0, 0x0c) // name: pointer to the question
		r = append(r, q[qend-4:qend]...)
		r = binary.BigEndian.AppendUint32(r, ttl)
		r = binary.BigEndian.AppendUint16(r, uint16(len(rdata)))
		r = append(r, rdata...)
	}
	return r
}
