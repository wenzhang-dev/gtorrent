package tracker

import (
	"fmt"
	"net/url"

    "github.com/wenzhang-dev/gtorrent/tracker/http"
    "github.com/wenzhang-dev/gtorrent/tracker/udp"

    "github.com/wenzhang-dev/gtorrent/metainfo"
)


type TrackerClient interface {
    Connect() error
    Announce() (metainfo.PeerInfos, error)
}

func FindPeers(announce string, infoHash, peerId []byte) (metainfo.PeerInfos, error) {
    parsed_url, err := url.Parse(announce)
    if err != nil {
        return nil, err
    }

    var cli TrackerClient
    switch parsed_url.Scheme {
    case "udp":
        cli = udptracker.NewClient(parsed_url, infoHash, peerId)
    case "http", "https":
        cli = httptracker.NewClient(parsed_url, infoHash, peerId)
    default:
        return nil, fmt.Errorf("Unsupported announce scheme")
    }

    if err := cli.Connect(); err != nil {
        return nil, err
    }

    return cli.Announce()
}
