package main

import (
    "fmt"
    "bytes"
    "io"
    "time"
    "net"
)

type HandshakeMsg struct {
    Proto string
    InfoHash []byte
    PeerId []byte
}

func NewHandshakeMsg(info_hash, peer_id []byte) *HandshakeMsg {
    return &HandshakeMsg {
        Proto: "BitTorrent protocol",
        InfoHash: info_hash,
        PeerId: peer_id,
    }
}

// Note, we cannot receive any response if handshake message format is error
func encodeHandshakeMsg(msg *HandshakeMsg, w io.Writer) error {
    var buf bytes.Buffer

    buf.Write([]byte{byte(len(msg.Proto))})  // length of protocol string
    buf.Write([]byte(msg.Proto))  // protocol string
    buf.Write(make([]byte, 8))  // reserved 8 bytes
    buf.Write(msg.InfoHash)  // 20 bytes sha1
    buf.Write(msg.PeerId)  // 20 bytes peer id

    _, err := w.Write(buf.Bytes())
    
    return err
}

// Note, maybe we can use bufio to decode the handshake message
func decodeHandshakeMsg(r io.Reader) (*HandshakeMsg, error) {
    lbuf := make([]byte, 1)  // length of protocol string
    if _, err := io.ReadFull(r, lbuf); err != nil {
        return nil, err
    }

    pbuf := make([]byte, lbuf[0])  // protocol string
    if _, err := io.ReadFull(r, pbuf); err != nil {
        return nil, err
    }

    rbuf := make([]byte, 8)  // reserved bytes
    if _, err := io.ReadFull(r, rbuf); err != nil {
        return nil, err
    }

    infoHash := make([]byte, 20)  // sha1 hash
    if _, err := io.ReadFull(r, infoHash); err != nil {
        return nil, err
    }

    peerId := make([]byte, 20)  // peer id
    if _, err := io.ReadFull(r, peerId); err != nil {
        return nil, err
    }

    return &HandshakeMsg {
        Proto: string(pbuf),
        InfoHash: infoHash,
        PeerId: peerId,
    }, nil
}

func Handshake(conn net.Conn, info_hash, peer_id []byte) (*HandshakeMsg, error) {
    conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetDeadline(time.Time{})

    req := NewHandshakeMsg(info_hash, peer_id)
    if err := encodeHandshakeMsg(req, conn); err != nil {
        return nil, err
    }

    msg, err := decodeHandshakeMsg(conn)
    if err != nil {
        return nil, err
    }

    if !bytes.Equal(msg.InfoHash, info_hash) {
        return nil, fmt.Errorf("Handshake failed: %x", msg.InfoHash)
    }

    return msg, nil
}
