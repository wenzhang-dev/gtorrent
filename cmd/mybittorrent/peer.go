package main

import (
    "io"
    "net"
    "time"
    "encoding/binary"

    "github.com/wenzhang-dev/gtorrent/metainfo"
)

type MsgId uint8

const (
	MsgChoke       MsgId = 0
	MsgUnchoke     MsgId = 1
	MsgInterested  MsgId = 2
	MsgNotInterest MsgId = 3
	MsgHave        MsgId = 4
	MsgBitfield    MsgId = 5
	MsgRequest     MsgId = 6
	MsgPiece       MsgId = 7
	MsgCancel      MsgId = 8

    MsgLength = 4
)

type PeerMsg struct {
	Id      MsgId
	Payload []byte
}

type PeerConn struct {
    net.Conn

    peer metainfo.PeerInfo
	selfId []byte
    peerId []byte
	infoSHA1 []byte
}

func (c *PeerConn) ReadMsg() (*PeerMsg, error) {
	lbuf := make([]byte, MsgLength)
	_, err := io.ReadFull(c, lbuf)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lbuf)
	if length == 0 {
		return nil, nil
	}

	mbuf := make([]byte, length)
	_, err = io.ReadFull(c, mbuf)
	if err != nil {
		return nil, err
	}
	return &PeerMsg{
		Id:      MsgId(mbuf[0]),
		Payload: mbuf[1:],
	}, nil
}

func (c *PeerConn) WriteMsg(m *PeerMsg) (int, error) {
	var buf []byte
	if m == nil {
		buf = make([]byte, MsgLength)
	}

	length := uint32(len(m.Payload) + 1) // payload + id
	buf = make([]byte, length + MsgLength)

	binary.BigEndian.PutUint32(buf[0:MsgLength], length)
	buf[MsgLength] = byte(m.Id)
	copy(buf[MsgLength+1:], m.Payload)

	return c.Write(buf)
}

func NewRequestMsg(index, offset, length int64) *PeerMsg {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(offset))
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))

	return &PeerMsg{MsgRequest, payload}
}

func NewInterestMsg() *PeerMsg {
    return &PeerMsg{
        Id: MsgInterested,
        Payload: nil,
    }
}

func NewPeerConn(peerInfo *metainfo.PeerInfo, infoHash, selfId []byte) (*PeerConn, error) {
    conn, err := net.DialTimeout("tcp", peerInfo.String(), 3 * time.Second)
    if err != nil {
        return nil, err
    }

    // torrent p2p handshake
    var peerId []byte
    if msg, err := Handshake(conn, infoHash, selfId); err != nil {
        conn.Close()
        return nil, err
    } else {
        peerId = msg.PeerId
    }

    return &PeerConn {
        Conn: conn,
        peer: *peerInfo,
        selfId: selfId,
        peerId: peerId,
        infoSHA1: infoHash,
    }, nil
}
