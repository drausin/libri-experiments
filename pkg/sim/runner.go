package sim

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"go.uber.org/zap"
	"github.com/drausin/libri/libri/common/logging"
	"go.uber.org/zap/zapcore"
)

const (
	toUploadSlack      = 16
	toDownloadSlack    = 16
	contentMediaType   = "application/x-gzip" // so we don't try to compress
	veryLightScryptN   = 2
	veryLightScryptP   = 1
	authorKeychainAuth = "sim author passphrase"
)

type Parameters struct {
	Duration             time.Duration
	NAuthors             uint
	DocsPerDay           uint
	ContentSizeGammaMean uint
	ContentSizeGammaVar  uint
	SharesPerUpload      uint
	ShareWaitMin         time.Duration
	ShareWaitMax         time.Duration
	NUploaders           uint
	NDownloaders         uint
	LogLevel             string
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
	privDir         directory
	nextUploadWaits durationSampler
	upDocs          uploadEventSampler
	toUpload        chan *uploadEvent
	toDownload      chan *downloadEvent
	done            chan struct{}
	mu              sync.Mutex
	logger          *zap.Logger
}

func NewRunner(params *Parameters, librarianAddrs []*net.TCPAddr) *runner {
	rng := rand.New(rand.NewSource(0))

	nextUploadWaits := &uniformDurationSampler{
		min: params.ShareWaitMin,
		max: params.ShareWaitMax,
		rng: rng,
	}
	upDocs := &uploadEventSamplerImpl{
		nSharesPerUpload: params.SharesPerUpload,
		content:          newGammaContentSampler(rng, params.ContentSizeGammaMean, params.ContentSizeGammaVar),
	}
	privDir := newDirectory(rng, librarianAddrs, params.NAuthors, params.LogLevel)
	return &runner{
		params:          params,
		privDir:         privDir,
		nextUploadWaits: nextUploadWaits,
		upDocs:          upDocs,
		toUpload:        make(chan *uploadEvent, toUploadSlack),
		toDownload:      make(chan *downloadEvent, toDownloadSlack),
		done:            make(chan struct{}),
		logger:          newDevLogger(getLogLevel(params.LogLevel)),
	}
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
	go r.generateUploads()

	// execute upload events & generate download events
	upWG := new(sync.WaitGroup)
	for c := uint(0); c < r.params.NUploaders; c++ {
		upWG.Add(1)
		go r.doUploads(upWG)
	}

	// execute download events
	downWG := new(sync.WaitGroup)
	for c := uint(0); c < r.params.NUploaders; c++ {
		downWG.Add(1)
		go r.doDownloads(downWG)
	}

	// exit cleanly
	<-r.done
	upWG.Wait()
	close(r.toDownload)
	downWG.Wait()
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

func (r *runner) generateUploads() {
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
}

func (r *runner) doUploads(wg *sync.WaitGroup) {
	defer wg.Done()
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
				to:     r.privDir.get(withPub),
				envKey: envKey,
			}
		}
	}
}

func (r *runner) doDownloads(wg *sync.WaitGroup) {
	defer wg.Done()
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

func pubKeyHex(pubKey *ecdsa.PublicKey) string {
	return fmt.Sprintf("%0130x", ecid.ToPublicKeyBytes(pubKey))
}

func getLogLevel(logLevelStr string) zapcore.Level {
	var logLevel zapcore.Level
	err := logLevel.Set(logLevelStr)
	maybePanic(err)
	return logLevel
}

// NewDevLogger creates a new logger with a given log level for use in development (i.e., not
// production).
func newDevLogger(logLevel zapcore.Level) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.DisableCaller = true
	config.Level.SetLevel(logLevel)

	logger, err := config.Build()
	maybePanic(err)
	return logger
}

func maybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

func newAuthorConfigs(dataDir string, librarianAddrs []*net.TCPAddr, nAuthors uint, logLevelStr string) []*author.Config {
	logLevel := server.GetLogLevel(logLevelStr)
	authorConfigs := make([]*author.Config, nAuthors)
	for c := uint(0); c < nAuthors; c++ {
		authorDataDir := filepath.Join(dataDir, fmt.Sprintf("author-%d", c))

		authorConfigs[c] = author.NewDefaultConfig().
			WithLibrarianAddrs(librarianAddrs).
			WithDataDir(authorDataDir).
			WithDefaultDBDir().
			WithDefaultKeychainDir().
			WithLogLevel(logLevel)
	}
	return authorConfigs
}
