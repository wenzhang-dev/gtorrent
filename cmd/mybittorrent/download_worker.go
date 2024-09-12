package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

    "github.com/wenzhang-dev/gtorrent/metainfo"
)

const (
    MaxBacklog int64 = 5
    BlockSize int64 = 16 * 1024
)

// In different goroutines, we use atomic variables to keep corrent
// data access. For example, In handleMsg goroutine, it maybe decrease
// backlog number, but at the same time, downloadPiece goroutine maybe
// increase the backlog number
type TaskState struct {
    backlog *atomic.Int64
    downloaded *atomic.Int64
    requested *atomic.Int64
    data []byte
}

func NewTaskState(length int64) TaskState {
    return TaskState{
        backlog: new(atomic.Int64),
        downloaded: new(atomic.Int64),
        requested: new(atomic.Int64),
        data: make([]byte, length),
    }
}

type Task struct {
    index int64
    sha1 []byte
    length int64
    
    stat TaskState
}

type Piece struct {
    index int64
    data []byte
}

type Worker struct {
    Conn *PeerConn

    Bitfield Bitfield
    Choke bool

    TaskCh chan *Task
    CurrentTask *Task
    PieceCh chan *Piece

    Downloader *Downloader

    Ntfr chan struct{}

    CloseCh chan struct{}
    Closed bool
}

func NewWorker(
    peer *metainfo.PeerInfo,
    infoHash []byte,
    peerId []byte,
    taskCh chan *Task,
    pieceCh chan *Piece,
    downloader *Downloader,
) (*Worker, error) {
    conn, err := NewPeerConn(peer, infoHash, peerId)
    if err != nil {
        return nil, err
    }

    worker := &Worker{
        Conn: conn,
        Bitfield: nil,
        Choke: true,
        TaskCh: taskCh,
        PieceCh: pieceCh,
        Downloader: downloader,
        Ntfr: make(chan struct{}),
        CloseCh: make(chan struct{}),
        Closed: false,
    }

    if err := worker.initBitfield(); err != nil {
        return nil, err
    }

    return worker, nil
}

func (w *Worker) Close() {
    slog.Info("[Worker] worker will exit", "peer", w.Conn.peer.String())

    // when the peer connection has been closed, the handleMsg goroutine will
    // exit immediately. And then the Run routine will exit
    w.Conn.Close()

    // this channel is used to notify other stuffs
    close(w.CloseCh)

    w.Closed = true
}

func (w *Worker) IsClosed() bool {
    return w.Closed
}

func (w *Worker) initBitfield() error {
    cancel := w.setTimeout(10 * time.Second)
    msg, err := w.Conn.ReadMsg()
    cancel()

    if err != nil {
        return err
    }

    if msg.Id != MsgBitfield {
        return fmt.Errorf("Expect the bitfield message")
    }

    w.handleBitfield(msg.Payload)

    return nil
}

func (w *Worker) downloadPiece(task *Task) (*Piece, error) {
    task.stat = NewTaskState(task.length)
    handler := func() error {
        if w.Choke || task.stat.backlog.Load() >= MaxBacklog {
            return nil
        }

        for task.stat.requested.Load() < task.length {
            length := BlockSize
            if task.length - task.stat.requested.Load() < length {
                length = task.length - task.stat.requested.Load()
            }

            msg := NewRequestMsg(task.index, task.stat.requested.Load(), length)
            if _, err := w.Conn.WriteMsg(msg); err != nil {
                return err
            }

            task.stat.backlog.Add(1)
            task.stat.requested.Add(length)
        }

        return nil
    }

    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for task.stat.downloaded.Load() < task.length {
        select {
        case _, ok := <-w.Ntfr:
            if !ok {
                return nil, fmt.Errorf("Receive the closed message")
            }

            if err := handler(); err != nil {
                return nil, err
            }
        case <-ticker.C:
            return nil, fmt.Errorf("Ticker timeout")
        }

        ticker.Reset(10 * time.Second)
    }

    return &Piece{
        index: task.index,
        data: task.stat.data,
    }, nil
}

func (w *Worker) Run() {
    defer w.Close()
    go w.handleMsg()

    var err error
    var piece *Piece
    for task := range w.TaskCh {
        if !w.Bitfield.HasPiece(int(task.index)) {
            w.TaskCh <- task
            continue
        }

        w.CurrentTask = task

        if piece, err = w.downloadPiece(task); err != nil {
            w.TaskCh <- task
            slog.Info("[Worker] main handler exited", "err", err)
            return
        }

        if err = w.checkPiece(piece); err != nil {
            w.TaskCh <- task
            slog.Info("[Worker] mismatch piece hash", "err", err)
            continue
        }

        w.PieceCh <- piece
    }
}

func (w *Worker) notify() {
    w.Ntfr <- struct{}{}
}

func (w *Worker) handleMsg() {
    var err error
    var msg *PeerMsg

    defer close(w.Ntfr)
    defer func(){
        if err != nil {
            slog.Info("[Worker] message handler exited", "err", err)
        }
    }()

    for err == nil {
        msg, err = w.Conn.ReadMsg()
        if err != nil {
            return
        }

        // keep-alive message
        if msg == nil {
            continue
        }

        switch msg.Id {
        case MsgBitfield:
            w.handleBitfield(msg.Payload)
        case MsgChoke:
            w.Choke = true
        case MsgUnchoke:
            w.Choke = false
        case MsgPiece:
            err = w.handlePieceMsg(msg.Payload)
        // TOOD: handle other messages
        }

        w.notify()
    }
}

func (w *Worker) handleBitfield(payload []byte) error {
    w.Bitfield = payload

    msg := NewInterestMsg()
    _, err := w.Conn.WriteMsg(msg)
    return err
}

func (w *Worker) handlePieceMsg(payload []byte) error {
    if len(payload) < 8 {
        return fmt.Errorf("Invalid block data: payload size: %d", len(payload))
    }
    index := int64(binary.BigEndian.Uint32(payload[:4]))
    begin := int64(binary.BigEndian.Uint32(payload[4:8]))
    block := payload[8:]

    task := w.CurrentTask
    if task == nil || task.stat.backlog.Load() <= 0 {
        return fmt.Errorf("Invalid block data: no launched request")
    }

    if index != task.index {
        return fmt.Errorf("Mismatch piece#%d, but got %d", task.index, index)
    }

    blockSize := int64(len(block))
    if blockSize > BlockSize || begin + blockSize > task.length {
        return fmt.Errorf("Invalid block data: begin: %d, block size: %d", begin, len(block))
    }

    copy(task.stat.data[begin:], block)

    task.stat.backlog.Add(-1)
    task.stat.downloaded.Add(blockSize)

    return nil
}

func (w *Worker) setTimeout(timeout time.Duration) func() {
    w.Conn.SetDeadline(time.Now().Add(timeout))
    return func() {
        w.Conn.SetDeadline(time.Time{})
    }
}

func (w *Worker) checkPiece(piece *Piece) error {
    sha := sha1.Sum(piece.data)
    originSha :=  w.Downloader.Torrent.PieceSHA1[piece.index]
    if !bytes.Equal(sha[:], originSha) {
        return fmt.Errorf("Mismatch SHA1 of piece#%d", piece.index)
    }

    return nil
}
