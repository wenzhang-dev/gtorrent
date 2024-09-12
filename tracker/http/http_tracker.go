package httptracker

import (
    "io"
    "time"
    "net/url"
    "net/http"
    "strconv"

    "github.com/wenzhang-dev/gtorrent/bencode"
    "github.com/wenzhang-dev/gtorrent/metainfo"
    "github.com/wenzhang-dev/gtorrent/tracker/utils"
)

type Response struct {
    Interval int `bencode:"interval"`
    Peers string `bencode:"peers"`
}

type Client struct {
    Url *url.URL
    InfoHash []byte
    PeerId []byte

    Cli *http.Client
}

func NewClient(announce *url.URL, infoHash, peerId []byte) *Client {
    return &Client{
        Url: announce,
        InfoHash: infoHash,
        PeerId: peerId,
        Cli: &http.Client{Timeout: 5 * time.Second},
    }
}

func (c *Client) Connect() error {
    return nil
}

func (c *Client) buildUrl() string {
	params := url.Values{
		"info_hash":  []string{string(c.InfoHash[:])},
		"peer_id":    []string{string(c.PeerId[:])},
		"port":       []string{strconv.Itoa(6666)}, // fake port
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{"1024"}, // fake number
	}

	c.Url.RawQuery = params.Encode()
	return c.Url.String()
}

func (c *Client) Announce() (metainfo.PeerInfos, error) {
    url_str := c.buildUrl()
    resp, err := c.Cli.Get(url_str)
    if err != nil {
        return nil, err
    }
    
	defer resp.Body.Close()

	trackResp := new(Response)
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    
	if err = bencode.Unmarshal(string(body), trackResp); err != nil {
        return nil, err
    }

	return utils.ParsePeerInfo([]byte(trackResp.Peers))
}
