package http2

// [SIGHTGLASS PATCH 9] this file is patch 9's fork-local guard.

// A CONNECTION-LEVEL PROTOCOL ERROR MUST PUT ITS GOAWAY ON THE WIRE.
//
// readLoop answers a ConnectionError by writing GOAWAY and then returning, at which point the
// deferred cleanup closes the connection. `Framer.endWrite` writes into `cc.bw`, a *bufio.Writer,
// and does NOT flush — every other write path in transport.go calls `cc.bw.Flush()` explicitly, and
// this one did not. So the GOAWAY sat in the buffer and the close discarded it.
//
// The peer saw an ABRUPT CLOSE where it should have seen a diagnosed one: the error code it needs to
// understand what it did wrong is lost, and a browser is distinguishable from this client in a
// single frame, because Chrome answers a connection-level protocol error with GOAWAY and then
// closes — which is what this code was already trying to do.
//
// Inherited from upstream golang.org/x/net/http2, which has the same omission.
//
//	go test ./http2/ -run TestGoAwayIsFlushed -v

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"golang.org/x/net/http2/hpack"
)

// goawayWriter lets hpack.Encoder append into a byte slice.
type goawayWriter struct{ b *[]byte }

func (w *goawayWriter) Write(p []byte) (int, error) { *w.b = append(*w.b, p...); return len(p), nil }

// TestGoAwayIsFlushedOnAConnectionError drives a real ClientConn into a connection-level protocol
// error and reads the client's own bytes off a real socket.
//
// A real TCP pair, not net.Pipe: NewClientConn writes the preface and SETTINGS synchronously, and an
// unbuffered pipe deadlocks before a reader can start.
//
// The trigger is an unsolicited PUSH_PROMISE on a Transport with no PushHandler, which
// processPushPromise answers with ConnectionError(ErrCodeProtocol) — its own comment there reads
// "should not be receiving PUSH_PROMISE if ENABLE_PUSH is disabled".
func TestGoAwayIsFlushedOnAConnectionError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	sawGoAway := make(chan bool, 1)
	go func() {
		srv, aerr := ln.Accept()
		if aerr != nil {
			sawGoAway <- false
			return
		}
		defer func() { _ = srv.Close() }()
		_ = srv.SetDeadline(time.Now().Add(10 * time.Second))

		// CONSUME THE CLIENT PREFACE FIRST. It is 24 bytes that are not a frame, and reading it as
		// one makes "PRI * HTT" a frame header with a 5 MB length — the reader then blocks forever
		// and the test reports "no GOAWAY" for a reason that has nothing to do with the GOAWAY.
		if _, rerr := io.ReadFull(srv, make([]byte, len(clientPreface))); rerr != nil {
			sawGoAway <- false
			return
		}

		// Server preface, then an unsolicited PUSH_PROMISE on stream 1 with a WELL-FORMED header
		// block. The block has to be valid: readMetaFrame runs checkPseudos() before the frame ever
		// reaches processPushPromise, and a malformed block is a STREAM error, which produces no
		// GOAWAY and would make this test pass or fail for the wrong reason.
		if _, werr := srv.Write([]byte{0, 0, 0, 0x04, 0, 0, 0, 0, 0}); werr != nil {
			sawGoAway <- false
			return
		}
		var hb []byte
		enc := hpack.NewEncoder(&goawayWriter{&hb})
		for _, f := range []hpack.HeaderField{
			{Name: ":method", Value: "GET"},
			{Name: ":authority", Value: "example.test"},
			{Name: ":scheme", Value: "https"},
			{Name: ":path", Value: "/pushed"},
		} {
			_ = enc.WriteField(f)
		}
		body := make([]byte, 4, 4+len(hb))
		binary.BigEndian.PutUint32(body, 2) // promised stream id
		body = append(body, hb...)
		frame := []byte{byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body)), 0x05, 0x04, 0, 0, 0, 1}
		frame = append(frame, body...)
		if _, werr := srv.Write(frame); werr != nil {
			sawGoAway <- false
			return
		}
		// Read frames until GOAWAY or EOF.
		hdr := make([]byte, 9)
		for {
			if _, rerr := io.ReadFull(srv, hdr); rerr != nil {
				sawGoAway <- false
				return
			}
			length := int(hdr[0])<<16 | int(hdr[1])<<8 | int(hdr[2])
			typ := hdr[3]
			stream := binary.BigEndian.Uint32(hdr[5:9]) & 0x7fffffff
			if length > 0 {
				if _, rerr := io.ReadFull(srv, make([]byte, length)); rerr != nil {
					sawGoAway <- false
					return
				}
			}
			if typ == 0x07 && stream == 0 {
				sawGoAway <- true
				return
			}
		}
	}()

	cli, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = cli.Close() }()

	tr := &Transport{} // PushHandler nil: a push is a connection error
	cc, err := tr.NewClientConn(cli)
	if err != nil {
		t.Fatalf("NewClientConn: %v", err)
	}
	defer func() { _ = cc.Close() }()

	select {
	case ok := <-sawGoAway:
		if !ok {
			t.Fatal("the client hit a connection-level PROTOCOL_ERROR and put NO GOAWAY on the " +
				"wire. readLoop writes one into cc.bw and the deferred cleanup closes the " +
				"connection before it is flushed, so the peer sees an abrupt close instead of a " +
				"diagnosed one — losing the error code, and diverging from a browser in one frame.")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("neither a GOAWAY nor a close within 15s")
	}
	t.Log("the client wrote a GOAWAY before closing")
}
