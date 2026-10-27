package dns

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

func resolver(s *Server) *net.Resolver {
	return &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		addr := s.udp.LocalAddr().String()
		if strings.HasPrefix(network, "tcp") {
			addr = s.tcp.Addr().String()
		}
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}}
}

func TestResolve(t *testing.T) {
	s, err := Listen("127.0.0.1:0", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := resolver(s)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, name := range []string{"hello.test", "api.my-app.TEST", "test"} {
		ips, err := r.LookupIP(ctx, "ip", name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var got []string
		for _, ip := range ips {
			got = append(got, ip.String())
		}
		if s := strings.Join(got, ","); !strings.Contains(s, "127.0.0.1") || !strings.Contains(s, "::1") {
			t.Errorf("%s -> %s", name, s)
		}
	}
	if _, err := r.LookupIP(ctx, "ip4", "example.com"); err == nil {
		t.Error("answered a name outside the zone")
	}

	// A reload that turns IPv6 off stops AAAA answers.
	s.Set("test", false)
	ips, err := r.LookupIP(ctx, "ip", "hello.test")
	if err != nil || len(ips) != 1 || !ips[0].Equal(net.IPv4(127, 0, 0, 1)) {
		t.Fatalf("ipv4 only: %v %v", ips, err)
	}
}

func TestTCP(t *testing.T) {
	s, err := Listen("127.0.0.1:0", "test", false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := net.Dial("tcp", s.tcp.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	q := query(0x1234, "site.test", typeA)
	c.Write(binary.BigEndian.AppendUint16(nil, uint16(len(q))))
	c.Write(q)
	c.SetDeadline(time.Now().Add(3 * time.Second))
	var n uint16
	if err := binary.Read(c, binary.BigEndian, &n); err != nil {
		t.Fatal(err)
	}
	resp := make([]byte, n)
	if _, err := c.Read(resp); err != nil {
		t.Fatal(err)
	}
	if resp[0] != 0x12 || resp[1] != 0x34 || binary.BigEndian.Uint16(resp[6:]) != 1 {
		t.Fatalf("bad response % x", resp)
	}
	if ip := resp[len(resp)-4:]; !net.IP(ip).Equal(net.IPv4(127, 0, 0, 1)) {
		t.Fatalf("answer %v", net.IP(ip))
	}
}

func TestAnswerEdgeCases(t *testing.T) {
	z := &zone{tld: "test"}
	rcode := func(b []byte) int { return int(b[3] & 0x0f) }

	if Answer([]byte{1, 2, 3}, z) != nil {
		t.Error("answered a runt")
	}
	resp := query(1, "a.test", typeA)
	resp[2] |= 0x80
	if Answer(resp, z) != nil {
		t.Error("answered a response")
	}
	if r := Answer(query(1, "example.com", typeA), z); rcode(r) != rcodeRefused {
		t.Errorf("outside zone: rcode %d", rcode(r))
	}
	if r := Answer(query(1, "nottest", typeA), z); rcode(r) != rcodeRefused {
		t.Errorf("suffix without a dot: rcode %d", rcode(r))
	}
	r := Answer(query(1, "a.test", 15 /* MX */), z)
	if rcode(r) != rcodeOK || binary.BigEndian.Uint16(r[6:]) != 0 {
		t.Error("MX should be NOERROR with no answer")
	}
	bad := query(1, "a.test", typeA)
	bad[12] = 0xc0 // a compression pointer in the question
	if r := Answer(bad, z); rcode(r) != rcodeFormErr {
		t.Errorf("pointer: rcode %d", rcode(r))
	}
	truncated := query(1, "a.test", typeA)
	if r := Answer(truncated[:len(truncated)-3], z); rcode(r) != rcodeFormErr {
		t.Errorf("truncated: rcode %d", rcode(r))
	}
}

func query(id uint16, name string, qtype uint16) []byte {
	q := binary.BigEndian.AppendUint16(nil, id)
	q = append(q, 0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 0) // RD, QDCOUNT 1
	for _, l := range strings.Split(name, ".") {
		q = append(q, byte(len(l)))
		q = append(q, l...)
	}
	q = append(q, 0)
	q = binary.BigEndian.AppendUint16(q, qtype)
	return binary.BigEndian.AppendUint16(q, classIN)
}
