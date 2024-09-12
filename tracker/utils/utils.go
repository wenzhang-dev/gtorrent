package utils

import (
    "net"
	"encoding/binary"
	"errors"

	"github.com/wenzhang-dev/gtorrent/metainfo"
)

const (
	IpLen    int = 4
	PortLen  int = 2
	PeerLen  int = IpLen + PortLen
)

func ParsePeerInfo(peers []byte) (metainfo.PeerInfos, error) {
	num := len(peers) / PeerLen
	if len(peers) % PeerLen != 0 {
        return nil, errors.New("Invalid peer infos")
	}

	infos := make(metainfo.PeerInfos, num)
	for i := 0; i < num; i++ {
		offset := i * PeerLen

        infos[i].Host = net.IP(peers[offset:offset+IpLen]).String()
        infos[i].Port = binary.BigEndian.Uint16(peers[offset+IpLen:offset+PeerLen])
	}

	return infos, nil
}
