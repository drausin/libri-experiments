package sim

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof" // pprof doc calls for blank import
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/api"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	toUploadSlack    = 16
	toDownloadSlack  = 16
	contentMediaType = "application/x-gzip" // so we don't try to compress

	// DefaultDuration is the default time for the experiment to run
	DefaultDuration = 1 * time.Hour

	// DefaultNAuthors is the default number of authors to use in the experiment.
	DefaultNAuthors = uint(1000)

	// DefaultDocsPerDay is the default number of documents to assume each author uploads per day.
	DefaultDocsPerDay = uint(1)

	// DefaultContentSizeKBGammaShape is the gamma distribution shape parameter for the content
	// size (in KB); this shape and rate imply a mean of ~256 KB and a 95% CI of [~18, ~794] KB.
	DefaultContentSizeKBGammaShape = float64(1.5)

	// DefaultContentSizeKBGammaRate is the gamma distribution rate parameter.
	DefaultContentSizeKBGammaRate = 1.0 / float64(170) // 1 / scale

	// DefaultSharesPerUpload is the default number of times each uploaded document is shared.
	DefaultSharesPerUpload = uint(2)

	// DefaultDownloadWaitMin is the default lower bound on the uniform distribution on time to
	// wait before downloading document.
	DefaultDownloadWaitMin = 2 * time.Second

	// DefaultDownloadWaitMax is the default upper bound on the uniform distribution on time to
	// wait before downloading document.
	DefaultDownloadWaitMax = 10 * time.Second

	// DefaultNUploaders is the default number of uploader workers to use.
	DefaultNUploaders = 3

	// DefaultNDownloaders is the default number of downloader workers to use.
	DefaultNDownloaders = DefaultNUploaders * DefaultSharesPerUpload

	// DefaultProfile is the default setting for whether to enable the profiling endpoint.
	DefaultProfile = false

	// DefaultLogLevel is the default log level.
	DefaultLogLevel = "INFO"

	localProfilerPort = 20300
)

// Parameters contains the parameters that define the experiment.
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
	Profile                 bool
	LogLevel                string
}

type uploadEvent struct {
	content   *bytes.Buffer
	from      *author.Author
	shareWith []*ecdsa.PublicKey
}

type downloadEvent struct {
	to     *author.Author
	envKey id.ID
}

// Runner runs experiments.
type Runner struct {
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

// NewRunner creates a new experiment Runner.
func NewRunner(params *Parameters, dataDir string, librarianAddrs []*net.TCPAddr) *Runner {
	downloadWait := &uniformDurationSampler{
		min: params.DownloadWaitMin,
		max: params.DownloadWaitMax,
		rng: rand.New(rand.NewSource(0)),
	}
	authors := newDirectory(rand.New(rand.NewSource(0)), dataDir, librarianAddrs, params.NAuthors,
		params.LogLevel)
	docSizeSampler := newGammaContentSampler(
		rand.New(rand.NewSource(0)),
		params.ContentSizeKBGammaShape,
		params.ContentSizeKBGammaRate,
	)
	upDocs := &uploadEventSamplerImpl{
		authors:          authors,
		nSharesPerUpload: params.SharesPerUpload,
		content:          docSizeSampler,
	}
	uploadsPerSecond := float64(params.NAuthors) * float64(params.DocsPerDay) / (24 * 3600)
	uploadWaitMS := 1000 / uploadsPerSecond

	return &Runner{
		params:         params,
		authors:        authors,
		nextUploadWait: newExponentialDurationSampler(rand.New(rand.NewSource(0)), uploadWaitMS),
		downloadWait:   downloadWait,
		upDocs:         upDocs,
		querier:        &querierImpl{},
		toUpload:       make(chan *uploadEvent, toUploadSlack),
		toDownload:     make(chan *downloadEvent, toDownloadSlack),
		done:           make(chan struct{}),
		logger:         newDevLogger(getLogLevel(params.LogLevel)),
	}
}

// Run begins the experiment.
func (r *Runner) Run() {
	stopSignals := make(chan os.Signal, 3)
	signal.Notify(stopSignals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	if r.params.Profile {
		go func() {
			profilerAddr := fmt.Sprintf(":%d", localProfilerPort)
			if err := http.ListenAndServe(profilerAddr, nil); err != nil {
				r.logger.Error("error serving profiler", zap.Error(err))
				r.stop()
			}
		}()
	}

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

func (r *Runner) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.done: // already closed
	default:
		close(r.done)
	}
}

func (r *Runner) generateUploads() {
	done := false
	for !done {
		select {
		case <-r.done:
			done = true
		default:
			wait := r.nextUploadWait.sample()
			r.logger.Debug("waiting for next upload", zap.Duration("wait_time", wait))
			time.Sleep(wait)
			r.toUpload <- r.upDocs.sample()
		}
	}
	close(r.toUpload)
}

func (r *Runner) doUploads(wg *sync.WaitGroup) {
	defer wg.Done()
	for uploadEvent := range r.toUpload {
		env, err := r.querier.upload(uploadEvent.from, uploadEvent.content)
		if err != nil {
			r.logger.Info("upload errored", zap.Error(err))
			continue
		}
		for _, withPub := range uploadEvent.shareWith {
			shareEnvKey, err := r.querier.share(uploadEvent.from, env, withPub)
			if err != nil {
				r.logger.Info("share errored", zap.Error(err))
				continue
			}
			r.toDownload <- &downloadEvent{
				to:     r.authors.get(withPub),
				envKey: shareEnvKey,
			}
		}
		select {
		case <-r.done:
			return
		default:
		}
	}
}

func (r *Runner) doDownloads(wg *sync.WaitGroup) {
	defer wg.Done()
	for downEvent := range r.toDownload {
		wait := r.downloadWait.sample()
		r.logger.Debug("waiting to download", zap.Duration("wait_time", wait))
		time.Sleep(wait)
		downloaded := new(bytes.Buffer)
		r.logger.Debug("downloading",
			zap.String("author_id", downEvent.to.ClientID.ID().String()),
		)
		if err := r.querier.download(downEvent.to, downloaded, downEvent.envKey); err != nil {
			r.logger.Info("download errored", zap.Error(err))
			continue
		}
		select {
		case <-r.done:
			return
		default:
		}
	}
}

// thin wrapper around author functions so they're easy to mock
type querier interface {
	upload(author *author.Author, content io.Reader) (*api.Envelope, error)
	download(author *author.Author, content io.Writer, envKey id.ID) error
	share(author *author.Author, env *api.Envelope, readerPub *ecdsa.PublicKey) (id.ID, error)
}

type querierImpl struct{}

func (q *querierImpl) upload(author *author.Author, content io.Reader) (*api.Envelope, error) {
	envDoc, _, err := author.Upload(content, contentMediaType)
	return envDoc.GetEnvelope(), err
}

func (q *querierImpl) download(author *author.Author, content io.Writer, envKey id.ID) error {
	return author.Download(content, envKey)
}

func (q *querierImpl) share(
	author *author.Author, env *api.Envelope, readerPub *ecdsa.PublicKey,
) (id.ID, error) {
	_, shareEnvKey, err := author.ShareEnvelope(env, readerPub)
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

func newAuthorConfigs(
	dataDir string, librarianAddrs []*net.TCPAddr, nAuthors uint, logLevelStr string,
) []*author.Config {
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
