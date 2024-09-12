package udptracker

import (
    "net/url"
    "net"
	"math/rand"
	"bytes"
    "errors"
    "time"
    "encoding/binary"

    "github.com/wenzhang-dev/gtorrent/metainfo"
    "github.com/wenzhang-dev/gtorrent/tracker/utils"
)

const (
    ActionConnect   = 0
    ActionAnnounce  = 1
    ActionScrap     = 2
    ActionError     = 3

    ProtoID uint64 = 0x41727101980
    MinPacketLen int = 8
)

var (
    InvalidConnectResponse = errors.New("Invalid connect response")
    InvalidAnnounceResponse = errors.New("Invalid announce response")
)

type Client struct {
    Url *url.URL
    InfoHash []byte
    PeerId []byte

    Conn net.Conn
    TID uint32
    CID uint64
}

func NewClient(announce *url.URL, infoHash, peerId []byte) *Client {
    return &Client{
        Url: announce,
        InfoHash: infoHash,
        PeerId: peerId,
        TID: uint32(rand.Int31()),
        CID: 0,
        Conn: nil,
    }
}

type ConnectRequest struct {
	PID uint64
	Action uint32
	TID uint32
}

type ConnectResponse struct {
	Action uint32
	TID uint32
	CID uint64
}

type AnnounceRequest struct {
	CID uint64
	Action uint32
	TID uint32
	InfoHash [20]byte
	PeerID [20]byte
	Downloaded uint64
	Left uint64
	Uploaded uint64
	Event uint32 
	IP uint32
	Key uint32
	NumWant uint32
	Port uint16
    Extensions uint16
}

type AnnounceResponse struct {
	Action uint32
	TID uint32
	Interval uint32
	Leechers uint32
	Seeders uint32
	Peers []byte
}

func (c *Client) parseConnectResponse(buf []byte) (*ConnectResponse, error) {
    var resp ConnectResponse
    if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &resp); err != nil {
        return nil, err
    }
    return &resp, nil
}

func (c *Client) parseAnnounceResponse(buf []byte) (*AnnounceResponse, error) {
    var resp AnnounceResponse

    // binary.Read can handle fixed-size struct. however, the number of Peers is variable
    resp.Action = binary.BigEndian.Uint32(buf[0:])
    resp.TID = binary.BigEndian.Uint32(buf[4:])
    resp.Interval = binary.BigEndian.Uint32(buf[8:])
    resp.Leechers = binary.BigEndian.Uint32(buf[12:])
    resp.Seeders = binary.BigEndian.Uint32(buf[16:])
    resp.Peers = buf[20:]

    return &resp, nil
}

func (c *Client) newConnectRequest() []byte {
    var buf bytes.Buffer
    
    req := ConnectRequest {
        PID: ProtoID,
        Action: ActionConnect,
        TID: c.TID,
    }

    binary.Write(&buf, binary.BigEndian, req)

    return buf.Bytes()
}

func (c *Client) newAnnounceRequest() []byte {
    var buf bytes.Buffer

    numWant := int32(-1)
    var infoHash, peerId [20]byte

    copy(peerId[:], c.PeerId)
    copy(infoHash[:], c.InfoHash)

    req := AnnounceRequest {
        CID: c.CID,
        Action: ActionAnnounce,
        TID: c.TID,
        InfoHash: infoHash,
        PeerID: peerId,
        Downloaded: 0,
        Left: 100, // left, fake number
        Uploaded: 0,
        Event: 2, // event, started
        IP: 0,
        Key: 1,
        NumWant: uint32(numWant),
        Port: uint16(6666), // fake port
        Extensions: 0,
    }

    binary.Write(&buf, binary.BigEndian, req)

    return buf.Bytes()
}

func (c *Client) sendAndReply(packet []byte) (buf []byte, err error) {
    const RetryTimes = 3
    const BufferSize = 512

    var readn = 0
    buf = make([]byte, BufferSize)
    for i:=0; i<RetryTimes; i++ {
        if _, err := c.Conn.Write(packet); err != nil {
            return nil, err
        }

        c.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
        readn, err = c.Conn.Read(buf)
        c.Conn.SetReadDeadline(time.Time{})

        if err, ok := err.(net.Error); ok && err.Timeout() {
            if i + 1 >= RetryTimes {
                return nil, err
            }
        } else if err != nil {
            return nil, err
        } else {
            break
        }
    }

    return buf[:readn], nil
}

func (c *Client) parseAction(buf []byte) uint32 {
    return binary.BigEndian.Uint32(buf[0:])
}

func (c *Client) Connect() error {
    var err error
    if c.Conn, err = net.DialTimeout("udp", c.Url.Host, 5 * time.Second); err != nil {
        return err
    }

    buf, err := c.sendAndReply(c.newConnectRequest())
    if err != nil {
        return err
    }

    if len(buf) < MinPacketLen || c.parseAction(buf) != ActionConnect {
        return InvalidConnectResponse
    }

    resp, err := c.parseConnectResponse(buf)
    if err != nil {
        return err
    }
    
    if resp.TID != c.TID {
        return InvalidConnectResponse
    }

    c.CID = resp.CID

    return nil
}

func (c *Client) Announce() (metainfo.PeerInfos, error) {
    buf, err := c.sendAndReply(c.newAnnounceRequest())
    if err != nil {
        return nil, err
    }

    if len(buf) < MinPacketLen || c.parseAction(buf) != ActionAnnounce {
        return nil, InvalidAnnounceResponse
    }

    resp, err := c.parseAnnounceResponse(buf)
    if err != nil {
        return nil, err
    }

	return utils.ParsePeerInfo(resp.Peers)
}
