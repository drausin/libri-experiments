package sim

import (
	"github.com/drausin/libri/libri/author"
	"crypto/ecdsa"
	"github.com/drausin/libri/libri/common/id"
	"go.uber.org/zap"
	"time"
	"io"
	"bytes"
	"sync"
	"os"
	"os/signal"
	"syscall"
)

const (
	toUploadSlack    = 16
	toDownloadSlack  = 16
	contentMediaType = "application/x-gzip" // so we don't try to compress
)

type Parameters struct {
	Duration             time.Duration
	NAuthors             uint
	DocsPerDay           uint
	SharesPerUpload      uint
	ContentSizeGammaMean uint
	ContentSizeGammaVar  uint
	NUploaders           uint
	NDownloaders         uint
}

type uploadEvent struct {
	content   io.Reader
	from      *author.Author
	shareWith []*ecdsa.PublicKey
}

type downloadEvent struct {
	to     *author.Author
	envKey id.ID
}

type runner struct {
	params          *Parameters
	pubDir          publicDirectory
	privDir         privateDirectory
	nextUploadWaits durationSampler
	upDocs          uploadDocSampler
	toUpload        chan *uploadEvent
	toDownload      chan *downloadEvent
	done            chan struct{}
	mu              sync.Mutex
	logger          *zap.Logger
}

func (r *runner) Run() {
	stopSignals := make(chan os.Signal, 3)
	signal.Notify(stopSignals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	go func() {
		<-stopSignals
		r.Stop()
	}()
	go func() {
		time.Sleep(r.params.Duration)
		r.Stop()
	}()

	// generate upload events
	go func() {
		done := false
		for !done {
			select {
			case <-r.done:
				done = true
			default:
				wait := r.nextUploadWaits.sample()
				r.logger.Debug("waiting to upload", zap.Duration("wait_time", wait))
				time.Sleep(wait)
				r.toUpload <- r.upDocs.sample()
			}
		}
		close(r.toUpload)
	}()

	// execute upload events & generate download events
	upWG1 := new(sync.WaitGroup)
	for c := uint(0); c < r.params.NUploaders; c++ {
		upWG1.Add(1)
		go func(upWG2 *sync.WaitGroup) {
			defer upWG2.Done()
			r.doUploads()
		}(upWG1)
	}

	// execute download events
	downWG1 := new(sync.WaitGroup)
	for c := uint(0); c < r.params.NUploaders; c++ {
		downWG1.Add(1)
		go func(downWG2 *sync.WaitGroup) {
			defer downWG2.Done()
			r.doDownloads()
		}(downWG1)
	}

	// exit cleanly
	<-r.done
	upWG1.Wait()
	close(r.toDownload)
	downWG1.Wait()
}

func (r *runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.done: // already closed
	default:
		close(r.done)
	}
}

func (r *runner) doUploads() {
	for uploadEvent := range r.toUpload {
		// TODO (drausin) add and log timer
		_, envKey, err := uploadEvent.from.Upload(uploadEvent.content, contentMediaType)
		if err != nil {
			r.logger.Info("upload errored", zap.Error(err))
			continue
		}
		r.logger.Info("upload succeeded")
		for _, withPub := range uploadEvent.shareWith {
			r.toDownload <- &downloadEvent{
				to:     r.privDir.getAuthor(withPub),
				envKey: envKey,
			}
		}
	}
}

func (r *runner) doDownloads() {
	for downloadEvent := range r.toDownload {
		downloaded := new(bytes.Buffer)
		// TODO (drausin) add and log timer
		if err := downloadEvent.to.Download(downloaded, downloadEvent.envKey); err != nil {
			r.logger.Info("download errored", zap.Error(err))
			continue
		}
		r.logger.Info("download succeeded")
	}
}

type durationSampler interface {
	sample() time.Duration
}

type contentSampler interface {
	sample() []byte
}

type uploadDocSampler interface {
	sample() *uploadEvent
}

type uploader interface {
	upload(doc *uploadEvent)
}

type downloader interface {
	download(envKey *id.ID)
}

type publicDirectory interface {
	getKey(authorID *id.ID) *ecdsa.PublicKey
}

type privateDirectory interface {
	getAuthor(key *ecdsa.PublicKey) *author.Author
}
