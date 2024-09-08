package main

import "log/slog"

const (
    DefaultDownloaderWorkers = 3
    MaxDownloaderWorkers = 10
)

// TODO: dynamic add the PeerInfo
type WorkerManager struct {
    Options *WorkerManagerOptions

    Downloader *Downloader

    PeerInfos []PeerInfo
    PeerInfoPos int

    Workers []*Worker

    monitorCh chan int
    closeCh chan struct{}
}

type WorkerManagerOptions struct {
    Workers int
}

func NewWorkerManager(downlaoder *Downloader, opts *WorkerManagerOptions) *WorkerManager {
    if opts.Workers <= 0 {
        opts.Workers = DefaultDownloaderWorkers
    }
    if opts.Workers > MaxDownloaderWorkers {
        opts.Workers = MaxDownloaderWorkers
    }

    return &WorkerManager {
        Options: opts,
        Downloader: downlaoder,
        PeerInfos: downlaoder.PeerInfos,
        PeerInfoPos: 0,
        Workers: make([]*Worker, 0, opts.Workers),
        monitorCh: make(chan int),
        closeCh: make(chan struct{}),
    }
}

func (mgr *WorkerManager) Close() {
    slog.Info("[WorkerManager] WorkerManager will exit")

    close(mgr.closeCh)

    for _, worker := range mgr.Workers {
        if !worker.IsClosed() {
            worker.Close()
        }
    }
}

func (mgr *WorkerManager) Start() {
    numWorkers := mgr.Options.Workers
    if numWorkers > len(mgr.PeerInfos) {
        numWorkers = len(mgr.PeerInfos)
    }

    slog.Info("[WorkerManager] launch the keepalive")
    go mgr.keepalive()

    slog.Info("[WorkerManager] init workers", "numWorkers", numWorkers)
    for ; mgr.PeerInfoPos<len(mgr.PeerInfos); mgr.PeerInfoPos++ {
        worker, err := NewWorker(
            &mgr.PeerInfos[mgr.PeerInfoPos],
            mgr.Downloader.Torrent.InfoSHA1,
            mgr.Downloader.PeerId,
            mgr.Downloader.TaskCh,
            mgr.Downloader.PieceCh,
            mgr.Downloader,
        )
        if err != nil {
            slog.Info(
                "[WorkerManager] skip peer",
                "peer", mgr.PeerInfos[mgr.PeerInfoPos],
                "err", err,
            )
            continue
        }

        mgr.Workers = append(mgr.Workers, worker)

        slog.Info(
            "[WorkerManager] monitor the worker",
            "peer", worker.Conn.peer.String(),
        )
        mgr.monitorWorker(len(mgr.Workers) - 1)

        slog.Info(
            "[WorkerManager] start a worker",
            "peer", worker.Conn.peer.String(),
            "index", len(mgr.Workers) - 1,
        )
        go worker.Run()

        if len(mgr.Workers) >= numWorkers {
            mgr.PeerInfoPos++
            break
        }
    }
}

func (mgr *WorkerManager) keepalive() {
    for {
        select {
        case <-mgr.closeCh:
            slog.Info("[WorkerManager] keepalive closed")
            return
        case idx := <-mgr.monitorCh:
            slog.Warn(
                "[WorkerManager] keepalive got the exited worker",
                "err", mgr.Workers[idx].Err.Error(),
            )
retry:
            if mgr.PeerInfoPos >= len(mgr.PeerInfos) {
                continue
            }

            worker, err := NewWorker(
                &mgr.PeerInfos[mgr.PeerInfoPos],
                mgr.Downloader.Torrent.InfoSHA1,
                mgr.Downloader.PeerId,
                mgr.Downloader.TaskCh,
                mgr.Downloader.PieceCh,
                mgr.Downloader,
            )
            if err != nil {
                mgr.PeerInfoPos++
                goto retry
            }

            slog.Info(
                "[WorkerManager] keepalive launch a new worker",
                "peer", mgr.PeerInfos[mgr.PeerInfoPos].String(),
            )

            go worker.Run()
            mgr.Workers[idx] = worker
            mgr.PeerInfoPos++
        }
    }
}

func (mgr *WorkerManager) monitorWorker(idx int) {
    go func(idx int) {
        select {
        case <-mgr.closeCh:
            slog.Info("[WorkerManager] monitor closed")
            return
        case <-mgr.Workers[idx].CloseCh:
            mgr.monitorCh <- idx
            slog.Info(
                "[WorkerManager] monitor got the exited worker",
                "peer", mgr.Workers[idx].Conn.peer.String(),
            )
        }
    }(idx)
}
