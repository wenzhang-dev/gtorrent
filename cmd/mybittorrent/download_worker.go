package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"time"
    "encoding/binary"
)

const (
    MaxBacklog = 5
    BlockSize = 16 * 1024
)

type TaskState struct {
    backlog int
    downloaded int
    requested int
    data []byte
}

func NewTaskState(length int) TaskState {
    return TaskState{
        backlog: 0,
        downloaded: 0,
        requested: 0,
        data: make([]byte, length),
    }
}

type Task struct {
    index int
    sha1 []byte
    length int
    
    stat TaskState
}

type Piece struct {
    index int
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

    Err error
    CloseCh chan struct{}
}

func NewWorker(
    peer *PeerInfo,
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

    return &Worker{
        Conn: conn,
        Bitfield: nil,
        Choke: true,
        TaskCh: taskCh,
        PieceCh: pieceCh,
        Downloader: downloader,
        Err: nil,
        CloseCh: make(chan struct{}),
    }, nil
}

func (w *Worker) downloadPiece(task *Task) (*Piece, error) {
    task.stat = NewTaskState(task.length)
    for task.stat.downloaded < task.length {
        if w.Err != nil {
            return nil, w.Err
        }

        if w.Choke || task.stat.backlog >= MaxBacklog {
            time.Sleep(10 * time.Millisecond)
            continue
        }
        
        for task.stat.requested < task.length {
            length := BlockSize
            if task.length - task.stat.requested < length {
                length = task.length - task.stat.requested
            }

            msg := NewRequestMsg(task.index, task.stat.requested, length)
            if _, err := w.Conn.WriteMsg(msg); err != nil {
                return nil, err
            }

            task.stat.backlog++
            task.stat.requested += length
        }
    }

    return &Piece{
        index: task.index,
        data: task.stat.data,
    }, nil
}

func (w *Worker) checkPiece(piece *Piece) error {
    sha := sha1.Sum(piece.data)
    originSha :=  w.Downloader.Torrent.PieceSHA1[piece.index]
    if !bytes.Equal(sha[:], originSha) {
        return fmt.Errorf("Mismatch SHA1 of piece#%d", piece.index)
    }

    return nil
}

func (w *Worker) Run() {
    defer w.Conn.Close()
    defer close(w.CloseCh)

    go w.handleMsg()

    for task := range w.TaskCh {
        if !w.Bitfield.HasPiece(task.index) {
            w.TaskCh <- task
            continue
        }

        w.CurrentTask = task

        piece, err := w.downloadPiece(task)
        if err != nil {
            fmt.Println(err)
            w.TaskCh <- task
            return
        }

        if err := w.checkPiece(piece); err != nil {
            w.TaskCh <- task
            continue
        }

        w.PieceCh <- piece
    }
}

func (w *Worker) setTimeout(timeout time.Duration) func() {
    w.Conn.SetDeadline(time.Now().Add(timeout))
    return func() {
        w.Conn.SetDeadline(time.Time{})
    }
}

func (w *Worker) handleMsg() {
    var err error
    var msg *PeerMsg
    defer func() {
        w.Err = err
    }()

    for err == nil {
        cancel := w.setTimeout(10 * time.Second)
        msg, err = w.Conn.ReadMsg()
        if err != nil {
            return
        }
        cancel()

        // keep-alive message
        if msg.Payload == nil {
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
        }
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
    index := binary.BigEndian.Uint32(payload[:4])
    begin := binary.BigEndian.Uint32(payload[4:8])
    block := payload[8:]

    task := w.CurrentTask
    if task == nil || task.stat.backlog <= 0 {
        return fmt.Errorf("Invalid block data: no launched request")
    }

    if index != uint32(task.index) {
        return fmt.Errorf("Mismatch piece#%d, but got %d", task.index, index)
    }

    if len(block) > BlockSize || int(begin) + len(block) > task.length {
        return fmt.Errorf("Invalid block data: begin: %d, block size: %d", begin, len(block))
    }

    copy(task.stat.data[begin:], block)

    task.stat.backlog--
    task.stat.downloaded += len(block)

    return nil
}
