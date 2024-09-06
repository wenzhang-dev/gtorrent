package main

import (
    "crypto/sha1"
)

type MetaInfo struct {
    Name string `bencode:"name"`
    Pieces string `bencode:"pieces"`
    Length int `bencode:"length"`
    PieceLength int `bencode:"piece length"`
}

type Meta struct {
    Announce string `bencode:"announce"`
    Info MetaInfo `bencode:"info"`
}

type Torrent struct {
	Announce string
	InfoSHA1  []byte
	FileName string
	FileLen  int64
	PieceLen int64
	PieceSHA1 [][]byte
}

const SHA1_LEN = 20

func ParseTorrent(content string) (torrent *Torrent, err error) {
    var meta Meta
	torrent = new(Torrent)
	if err = Unmarshal(content, &meta); err != nil {
        return
    }

	torrent.Announce = meta.Announce
	torrent.FileName = meta.Info.Name
	torrent.FileLen = int64(meta.Info.Length)
	torrent.PieceLen = int64(meta.Info.PieceLength)

    hasher := sha1.New()
    if err = Marshal(hasher, meta.Info); err != nil {
        return
    }
    torrent.InfoSHA1 = hasher.Sum(nil)

    torrent.PieceSHA1 = make([][]byte, 0, len(meta.Info.Pieces) / SHA1_LEN)
    for i:=0; i<len(meta.Info.Pieces); i+=SHA1_LEN {
        torrent.PieceSHA1 = append(torrent.PieceSHA1, []byte(meta.Info.Pieces[i:i+SHA1_LEN]))
    }

	return
}
