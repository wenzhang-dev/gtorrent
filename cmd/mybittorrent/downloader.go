package main

import (
    "fmt"
    "os"
    "context"
)

type Downloader struct {
    Torrent *Torrent

    PeerId []byte
    PeerInfos []PeerInfo

    Workers []*Worker
    TaskCh chan *Task
    PieceCh chan *Piece
}

func NewDownloader(peerId []byte, torrent *Torrent) (*Downloader, error) {
    peers, err := FindPeers(torrent, peerId)
    if err != nil {
        return nil, err
    }

    return &Downloader{
        Torrent: torrent,
        PeerId: peerId,
        PeerInfos: peers,
    }, nil
}

func (d *Downloader) resolveRange(idx int64) (begin, end int64){
    begin = idx * d.Torrent.PieceLen
    end = begin + d.Torrent.PieceLen
    if end > d.Torrent.FileLen {
        end = d.Torrent.FileLen
    }
    return
}

// Download all pieces
func (d *Downloader) Download(filePath string) error {
    return d.DownloadPieces(nil, filePath)
}

// Download the specific piece
func (d *Downloader) DownloadPiece(index int64, filePath string) error {
    return d.DownloadPieces([]int64{index}, filePath)
}

// Download the specific pieces
func (d *Downloader) DownloadPieces(indexes []int64, filePath string) error {
    ctx := context.Background()
    allPieces := indexes == nil || len(indexes) == len(d.Torrent.InfoSHA1)
    singleFile := allPieces || len(indexes) == 1
    return d.DownloadPiecesWithCallback(ctx, indexes, func(ctx context.Context, piece *Piece) error {
        if piece == nil {
            return fmt.Errorf("Empty piece#%d", piece.index)
        }

        filename := filePath
        if !singleFile {
            filename = fmt.Sprintf("%s-%d", filename, piece.index)
        }

        offset := int64(0)
        if allPieces {
            offset = piece.index * d.Torrent.PieceLen
        }

        f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
        if err != nil {
            return err
        }

        defer f.Close()

        _, err = f.WriteAt(piece.data, offset)
        return err
    })
}


func (d *Downloader) DownloadPiecesWithCallback(
    ctx context.Context,
    indexes []int64,
    writer func(ctx context.Context, piece *Piece) error,
) error {
    if len(indexes) == 0 {
        // If indexes if empty, download all pieces
        indexes = make([]int64, len(d.Torrent.PieceSHA1))
        for i := range indexes { indexes[i] = int64(i) }
    }

    d.PieceCh = make(chan *Piece)
    d.TaskCh = make(chan *Task, len(indexes))

    defer close(d.TaskCh)
    defer close(d.PieceCh)

    for _, idx := range(indexes) {
        if idx > int64(len(d.Torrent.PieceSHA1)) {
            return fmt.Errorf("Piece index out of range: %d", idx)
        }

        begin, end := d.resolveRange(idx)
        d.TaskCh <- &Task{
            index: idx,
            length: end - begin,
            sha1: d.Torrent.PieceSHA1[idx],
        }
    }

    for _, peer := range(d.PeerInfos) {
        worker, err := NewWorker(&peer, d.Torrent.InfoSHA1, d.PeerId, d.TaskCh, d.PieceCh, d)
        if err != nil {
            continue
        }

        d.Workers = append(d.Workers, worker)

        go worker.Run()
    }

    count := 0
    for count < len(indexes) {
        piece := <-d.PieceCh
        if err := writer(ctx, piece); err != nil {
            return err
        }

        count++
    }

    return nil
}
