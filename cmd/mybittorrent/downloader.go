package main

import (
    "fmt"
    "os"
    "context"
    "log/slog"

    "github.com/wenzhang-dev/gtorrent/tracker"
    "github.com/wenzhang-dev/gtorrent/metainfo"
)

type Downloader struct {
    Torrent *Torrent

    PeerId []byte
    PeerInfos metainfo.PeerInfos

    WorkerMgr *WorkerManager

    TaskCh chan *Task
    PieceCh chan *Piece
}

func NewDownloader(peerId []byte, torrent *Torrent) (*Downloader, error) {
    peers, err := tracker.FindPeers(torrent.Announce, torrent.InfoSHA1, peerId)
    if err != nil {
        return nil, err
    }

    slog.Info("[Downloader] found the peers", "peers", peers.String())

    downloader := &Downloader{
        Torrent: torrent,
        PeerId: peerId,
        PeerInfos: peers,
    }

    downloader.WorkerMgr = NewWorkerManager(downloader, &WorkerManagerOptions{Workers: 5})

    return downloader, nil
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
    defer d.WorkerMgr.Close()

    slog.Info("[Downloader] start download", "numPieces", len(indexes))
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

    d.WorkerMgr.Start()

    count := 0
    for count < len(indexes) {
        piece := <-d.PieceCh

        slog.Info("[Downloader] got a piece", "piece", piece.index)

        if err := writer(ctx, piece); err != nil {
            return err
        }

        count++
    }

    slog.Info("[Downloader] download all requested pieces", "numPieces", count)

    return nil
}
