package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

var _ = json.Marshal

func decode() error {
    bencodedValue := os.Args[2]
    parser := NewBencodeParser(bencodedValue)

    decoded, err := parser.Parse()
    if err != nil {
        return err
    }

    jsonOutput, _ := json.Marshal(decoded)
    fmt.Println(string(jsonOutput))

    return nil
}

func info() error {
    filename := os.Args[2]

    content, err := os.ReadFile(filename)
    if err != nil {
        return err
    }

    torrent, err := ParseTorrent(string(content))
    if err != nil {
        return err
    }

    fmt.Printf("Tracker URL: %s\n", torrent.Announce)
    fmt.Printf("Length: %d\n", torrent.FileLen)
    fmt.Printf("Info Hash: %x\n", torrent.InfoSHA1)
    fmt.Printf("Piece Length: %d\n", torrent.PieceLen)
    fmt.Println("Piece Hashes:")
    for i:=0; i<len(torrent.PieceSHA1); i++ {
        fmt.Printf("%x\n", torrent.PieceSHA1[i])
    }

    return nil
}

func peers() error {
    filename := os.Args[2]

    content, err := os.ReadFile(filename)
    if err != nil {
        return err
    }

    torrent, err := ParseTorrent(string(content))
    if err != nil {
        return err
    }

    peers, err := FindPeers(torrent, []byte("01020304050607080900"))
    if err != nil {
        return err
    }

    for _, peer := range(peers) {
        fmt.Printf("%s:%d\n", peer.Ip.String(), peer.Port)
    }

    return nil
}

func handShake() error {
    filename := os.Args[2]

    content, err := os.ReadFile(filename)
    if err != nil {
        return err
    }

    torrent, err := ParseTorrent(string(content))
    if err != nil {
        return err
    }

    endpoint := os.Args[3]
    peerInfo, err := NewPeerInfo(endpoint)
    if err != nil {
        return err
    }

    peer, err := NewPeerConn(peerInfo, torrent.InfoSHA1, []byte("01020304050607080900"))
    if err != nil {
        return err
    }

    fmt.Printf("Peer ID: %x\n", peer.peerId)

    return nil
}

func downloadPiece() error {
    if len(os.Args) < 6 {
        return fmt.Errorf("Usage: ./your_bittorrent.sh download_piece -o /tmp/test-piece-0 sample.torrent 0")
    }

    dstPath := os.Args[3]
    torrentPath := os.Args[4]
    index, err := strconv.Atoi(os.Args[5])
    if err != nil {
        return err
    }

    content, err := os.ReadFile(torrentPath)
    if err != nil {
        return err
    }

    torrent, err := ParseTorrent(string(content))
    if err != nil {
        return err
    }

    downloader, err := NewDownloader([]byte("01020304050607080900"), torrent)
    if err != nil {
        return err
    }

    if err := downloader.DownloadPiece(int64(index), dstPath); err != nil {
        return err
    }

    fmt.Printf("Piece %d downloaded to %s.\n", index, dstPath)

    return nil
}

func downloadAllPieces() error {
    if len(os.Args) < 5 {
        return fmt.Errorf("Usage: ./your_bittorrent.sh download -o /tmp/test.txt sample.torrent")
    }

    dstPath := os.Args[3]
    torrentPath := os.Args[4]

    content, err := os.ReadFile(torrentPath)
    if err != nil {
        return err
    }

    torrent, err := ParseTorrent(string(content))
    if err != nil {
        return err
    }

    downloader, err := NewDownloader([]byte("01020304050607080900"), torrent)
    if err != nil {
        return err
    }

    if err := downloader.Download(dstPath); err != nil {
        return err
    }

    fmt.Printf("Downloaded %s to %s.\n", torrentPath, dstPath)

    return nil
}

func main() {
    var err error
    command := os.Args[1]
    switch command {
    case "decode":
        err = decode()
    case "info":
        err = info()
    case "peers":
        err = peers()
    case "handshake":
        err = handShake()
    case "download_piece":
        err = downloadPiece()
    case "download":
        err = downloadAllPieces()
    default:
        fmt.Println("Unknown command: " + command)
        os.Exit(1)
    }

    if err != nil {
        fmt.Println(err)
    }
}
