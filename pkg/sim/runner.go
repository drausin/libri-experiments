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
	"github.com/drausin/libri/libri/common/id"
	"go.uber.org/zap"
	"github.com/drausin/libri/libri/common/logging"
	"go.uber.org/zap/zapcore"
)

const (
	toUploadSlack    = 16
	toDownloadSlack  = 16
	contentMediaType = "application/x-gzip" // so we don't try to compress

	defaultDuration   = 1 * time.Hour
	defaultNAuthors   = uint(1000)
	defaultDocsPerDay = uint(1)

	// units are in KB, so this distribution has a mean of ~256 KB and a 95% CI of [~18, ~794] KB
	defaultContentSizeKBGammaShape = float64(1.5)
	defaultContentSizeKBGammaRate  = 1.0 / float64(170) // 1 / scale

	defaultSharesPerUpload = uint(2)
	defaultDownloadWaitMin = 2 * time.Second
	defaultDownloadWaitMax = 10 * time.Second
	defaultNUploaders      = 3
	defaultNDownloaders    = defaultNUploaders * defaultSharesPerUpload
	defaultLogLevel        = "INFO"
)

type Parameters struct {
	Duration                time.Duration
	NAuthors                uint
	DocsPerDay              uint
	ContentSizeKBGammaShape float64
	ContentSizeKBGammaRate  float64
	SharesPerUpload         uint
	DownloadWaitMin         time.Duration
	DownloadWaitMax         time.Duration
	NUploaders              uint
	NDownloaders            uint
	LogLevel                string
}

func NewDefaultParameters() *Parameters {
	return &Parameters{
		Duration:                defaultDuration,
		NAuthors:                defaultNAuthors,
		DocsPerDay:              defaultDocsPerDay,
		ContentSizeKBGammaShape: defaultContentSizeKBGammaShape,
		ContentSizeKBGammaRate:  defaultContentSizeKBGammaRate,
		SharesPerUpload:         defaultSharesPerUpload,
		DownloadWaitMin:         defaultDownloadWaitMin,
		DownloadWaitMax:         defaultDownloadWaitMax,
		NUploaders:              defaultNUploaders,
		NDownloaders:            defaultNDownloaders,
		LogLevel:                defaultLogLevel,
	}
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
	params         *Parameters
	authors        directory
	nextUploadWait durationSampler
	downloadWait   durationSampler
	upDocs         uploadEventSampler
	querier        querier
	toUpload       chan *uploadEvent
	toDownload     chan *downloadEvent
	done           chan struct{}
	mu             sync.Mutex
	logger         *zap.Logger
}

func NewRunner(params *Parameters, dataDir string, librarianAddrs []*net.TCPAddr) *runner {
	rng := rand.New(rand.NewSource(0))
	downloadWait := &uniformDurationSampler{
		min: params.DownloadWaitMin,
		max: params.DownloadWaitMax,
		rng: rng,
	}
	authors := newDirectory(rng, dataDir, librarianAddrs, params.NAuthors, params.LogLevel)
	upDocs := &uploadEventSamplerImpl{
		authors:          authors,
		nSharesPerUpload: params.SharesPerUpload,
		content:          newGammaContentSampler(rng, params.ContentSizeKBGammaShape, params.ContentSizeKBGammaRate),
	}
	uploadsPerSecond := float64(params.NAuthors) * float64(params.DocsPerDay) / (24 * 3600)
	uploadWaitMS := 1000 / uploadsPerSecond

	return &runner{
		params:         params,
		authors:        authors,
		nextUploadWait: newExponentialDurationSampler(rng, uploadWaitMS),
		downloadWait:   downloadWait,
		upDocs:         upDocs,
		querier:        &querierImpl{},
		toUpload:       make(chan *uploadEvent, toUploadSlack),
		toDownload:     make(chan *downloadEvent, toDownloadSlack),
		done:           make(chan struct{}),
		logger:         newDevLogger(getLogLevel(params.LogLevel)),
	}
}

func (r *runner) Run() {
	stopSignals := make(chan os.Signal, 3)
	signal.Notify(stopSignals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	go func() {
		<-stopSignals
		r.logger.Info("received external stop signal")
		r.stop()
	}()
	go func() {
		time.Sleep(r.params.Duration)
		r.logger.Info("finished experiment duration")
		r.stop()
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

func (r *runner) stop() {
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
			wait := r.nextUploadWait.sample()
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
		select {
		case <-r.done:
			return
		default:
			// TODO (drausin) add and log timer, speed
			envKey, err := r.querier.upload(uploadEvent.from, uploadEvent.content)
			if err != nil {
				r.logger.Info("upload errored", zap.Error(err))
				continue
			}
			r.logger.Info("upload succeeded")
			for _, withPub := range uploadEvent.shareWith {
				shareEnvKey, err := r.querier.share(uploadEvent.from, envKey, withPub)
				if err != nil {
					r.logger.Info("share errored", zap.Error(err))
					continue
				}
				r.toDownload <- &downloadEvent{
					to:     r.authors.get(withPub),
					envKey: shareEnvKey,
				}
			}
		}
	}
}

func (r *runner) doDownloads(wg *sync.WaitGroup) {
	defer wg.Done()
	for downloadEvent := range r.toDownload {
		select {
		case <-r.done:
			return
		default:
			wait := r.downloadWait.sample()
			r.logger.Debug("waiting to download", zap.Duration("wait_time", wait))
			time.Sleep(wait)
			downloaded := new(bytes.Buffer)
			// TODO (drausin) add and log timer, speed
			if err := r.querier.download(downloadEvent.to, downloaded, downloadEvent.envKey); err != nil {
				r.logger.Info("download errored", zap.Error(err))
				continue
			}
			r.logger.Info("download succeeded")
		}
	}
}

// thin wrapper around author functions so they're easy to mock
type querier interface {
	upload(author *author.Author, content io.Reader) (id.ID, error)
	download(author *author.Author, content io.Writer, envKey id.ID) error
	share(author *author.Author, envKey id.ID, readerPub *ecdsa.PublicKey) (id.ID, error)
}

type querierImpl struct{}

func (q *querierImpl) upload(author *author.Author, content io.Reader) (id.ID, error) {
	_, envKey, err := author.Upload(content, contentMediaType)
	return envKey, err
}

func (q *querierImpl) download(author *author.Author, content io.Writer, envKey id.ID) error {
	return author.Download(content, envKey)
}

func (q *querierImpl) share(author *author.Author, envKey id.ID, readerPub *ecdsa.PublicKey) (id.ID, error) {
	_, shareEnvKey, err := author.Share(envKey, readerPub)
	return shareEnvKey, err
}

func pubKeyHex(pubKey *ecdsa.PublicKey) string {
	return id.Hex(pubKey.X.Bytes())
}

func getLogLevel(logLevelStr string) zapcore.Level {
	var logLevel zapcore.Level
	err := logLevel.Set(logLevelStr)
	maybePanic(err)
	return logLevel
}

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
