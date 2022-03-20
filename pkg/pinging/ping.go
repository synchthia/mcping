package pinging

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

const (
	protocolVersion = 0x2F
)

// PingResponse - Response object from Minecraft server
type PingResponse struct {
	Version     Version     `json:"version"`
	Players     Players     `json:"players"`
	Description interface{} `json:"description"`
	Favicon     string      `json:"favicon"`
}

// Version - Server version
type Version struct {
	Name     string
	Protocol int
}

// Players - Information of server players
type Players struct {
	Max    int
	Online int
	Sample []map[string]string
}

// Ping - Send ping to Minecraft server
func Ping(address string, port int) (*PingResponse, error) {
	host := fmt.Sprintf("%s:%d", address, port)
	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	if err := sendHandShake(conn, host); err != nil {
		return nil, err
	}

	if err := sendStatusRequest(conn); err != nil {
		return nil, err
	}

	pong, err := readPong(conn)
	if err != nil {
		return nil, err
	}

	return pong, nil
}

func sendHandShake(conn net.Conn, host string) error {
	pl := &bytes.Buffer{}

	// packet id
	pl.WriteByte(0x00)

	// protocol version
	pl.WriteByte(protocolVersion)

	// server address
	host, port, err := net.SplitHostPort(host)
	if err != nil {
		panic(err)
	}

	pl.Write(encodeVarint(uint64(len(host))))
	pl.WriteString(host)

	// server port
	iPort, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}
	binary.Write(pl, binary.BigEndian, int16(iPort))

	// next state (status)
	pl.WriteByte(0x01)

	if _, err := makePacket(pl).WriteTo(conn); err != nil {
		return errors.New("cannot write handshake")
	}

	return nil
}

func sendStatusRequest(conn net.Conn) error {
	pl := &bytes.Buffer{}

	// send request zero
	pl.WriteByte(0x00)

	if _, err := makePacket(pl).WriteTo(conn); err != nil {
		return errors.New("cannot write send status request")
	}

	return nil
}

func readPong(rd io.Reader) (*PingResponse, error) {
	r := bufio.NewReader(rd)
	nl, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, errors.New("could not read length")
	}

	pl := make([]byte, nl)
	_, err = io.ReadFull(r, pl)
	if err != nil {
		return nil, errors.New("could not read length given by length header")
	}

	// packet id
	_, n := binary.Uvarint(pl)
	if n <= 0 {
		return nil, errors.New("could not read packet id")
	}

	// string varint
	_, n2 := binary.Uvarint(pl[n:])
	if n2 <= 0 {
		return nil, errors.New("could not read string varint")
	}

	res := PingResponse{}
	if err := json.Unmarshal(pl[n+n2:], &res); err != nil {
		return nil, errors.New("could not read pong json")
	}

	return &res, nil
}

// https://code.google.com/p/goprotobuf/source/browse/proto/encode.go#83
func encodeVarint(x uint64) []byte {
	var buf [10]byte
	var n int
	for n = 0; x > 127; n++ {
		buf[n] = 0x80 | uint8(x&0x7F)
		x >>= 7
	}
	buf[n] = uint8(x)
	n++
	return buf[0:n]
}

func makePacket(pl *bytes.Buffer) *bytes.Buffer {
	var buf bytes.Buffer
	// get payload length
	buf.Write(encodeVarint(uint64(len(pl.Bytes()))))

	// write payload
	buf.Write(pl.Bytes())

	return &buf
}
