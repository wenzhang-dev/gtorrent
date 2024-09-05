package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

type PeerInfo struct {
    Ip net.IP
    Port uint16
}

func NewPeerInfo(endpoint string) (*PeerInfo, error) {
    addr, err := net.ResolveTCPAddr("tcp", endpoint)
    if err != nil {
        return nil, err
    }

    return &PeerInfo {
        Ip: addr.IP,
        Port: uint16(addr.Port),
    }, nil
}

type TrackerResponse struct {
    Interval int `bencode:"interval"`
    Peers string `bencode:"peers"`
}

const (
	PeerPort int = 6666
	IpLen    int = 4
	PortLen  int = 2
	PeerLen  int = IpLen + PortLen
)

func buildUrl(t *Torrent, peerId []byte) (string, error) {
    parsed_url, err := url.Parse(t.Announce)
	if err != nil {
		return "", err
	}

	params := url.Values{
		"info_hash":  []string{string(t.InfoSHA1[:])},
		"peer_id":    []string{string(peerId[:])},
		"port":       []string{strconv.Itoa(PeerPort)},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.FileLen)},
	}

	parsed_url.RawQuery = params.Encode()
	return parsed_url.String(), nil
}

func buildPeerInfo(peers []byte) ([]PeerInfo, error) {
	num := len(peers) / PeerLen
	if len(peers) % PeerLen != 0 {
        return nil, fmt.Errorf("Received malformed peers")
	}

	infos := make([]PeerInfo, num)
	for i := 0; i < num; i++ {
		offset := i * PeerLen
		infos[i].Ip = net.IP(peers[offset : offset+IpLen])
		infos[i].Port = binary.BigEndian.Uint16(peers[offset+IpLen : offset+PeerLen])
	}

	return infos, nil
}

func FindPeers(t *Torrent, peerId []byte) ([]PeerInfo, error) {
	url_str, err := buildUrl(t, peerId)
	if err != nil {
		return nil, err
	}

	cli := &http.Client{}
	resp, err := cli.Get(url_str)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	trackResp := new(TrackerResponse)
    // TODO: maybe implement the io.reader in bencode parser
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    
	if err = Unmarshal(string(body), trackResp); err != nil {
        return nil, err
    }

	return buildPeerInfo([]byte(trackResp.Peers))
}

