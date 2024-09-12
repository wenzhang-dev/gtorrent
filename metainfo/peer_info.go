package metainfo

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type PeerInfo struct {
    Host string
    Port uint16
}

func (p PeerInfo) String() string {
    return fmt.Sprintf("%s:%d", p.Host, p.Port)
}

type PeerInfos []PeerInfo

func (p PeerInfos) String() string {
    infos := make([]string, len(p))
    for idx, peer := range p {
        infos[idx] = fmt.Sprintf("%s:%d", peer.Host, peer.Port)
    }
    return strings.Join(infos, ",")
}

func NewPeerInfo(endpoint string) (*PeerInfo, error) {
    host, port, err := net.SplitHostPort(endpoint)
    if err != nil {
        return nil, err
    }

    portNum, err := strconv.ParseInt(port, 10, 16)
    if err != nil {
        return nil, err
    }

    return &PeerInfo{
        Host: host,
        Port: uint16(portNum),
    }, nil
}


