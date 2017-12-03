package sim

import (
	"time"
	"math/rand"
	"gonum.org/v1/gonum/stat/distuv"
	erand "golang.org/x/exp/rand"
	"fmt"
	"io"
	"bytes"
	"crypto/ecdsa"
	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/keychain"
	"net"
	"github.com/drausin/libri/libri/common/logging"
)

const (
	nInitialKeys = 8
)

type directory interface {
	get(key *ecdsa.PublicKey) *author.Author
	sample() (*author.Author, *ecdsa.PublicKey)
}

type directoryImpl struct {
	authors    []*author.Author
	keys       []keychain.GetterSampler
	authorPubs map[string]*author.Author
	rng        *rand.Rand
}

func newDirectory(rng *rand.Rand, dataDir string, librarianAddrs []*net.TCPAddr, nAuthors uint, logLevelStr string) *directoryImpl {
	authors := make([]*author.Author, nAuthors)
	keys := make([]keychain.GetterSampler, nAuthors)
	logger := server.NewDevLogger(server.GetLogLevel(logLevelStr))

	configs := newAuthorConfigs(dataDir, librarianAddrs, nAuthors, logLevelStr)
	nWorkers := 8
	for c := 0; c < nWorkers; c++ {
		go func(d int) {
			for i := d; i < len(configs); i += nWorkers {
				// create keychains
				authorKC := keychain.New(nInitialKeys)
				selfReaderKC := keychain.New(nInitialKeys)
				keys[i] = authorKC

				// create author
				var err error
				authors[i], err = author.NewAuthor(configs[i], authorKC, selfReaderKC, logger)
				maybePanic(err)
			}
		}(c)
	}
	return &directoryImpl{
		authors:    authors,
		keys:       keys,
		authorPubs: make(map[string]*author.Author),
		rng:        rng,
	}
}

func (s *directoryImpl) sample() (*author.Author, *ecdsa.PublicKey) {
	nAuthors := len(s.authors)
	i := s.rng.Int31n(int32(nAuthors))
	auth := s.authors[i]
	authorKey, err := s.keys[i].Sample()
	maybePanic(err) // should never happen
	authorPubKey := &authorKey.Key().PublicKey
	authorKeyHex := pubKeyHex(authorPubKey)
	s.authorPubs[authorKeyHex] = auth
	return auth, authorPubKey
}

func (s *directoryImpl) get(key *ecdsa.PublicKey) *author.Author {
	return s.authorPubs[pubKeyHex(key)]
}

type durationSampler interface {
	sample() time.Duration
}

type uniformDurationSampler struct {
	min time.Duration
	max time.Duration
	rng *rand.Rand
}

func (s *uniformDurationSampler) sample() time.Duration {
	return s.min + time.Duration(s.rng.Float32()*float32(s.max-s.min))
}

type exponentialDurationSampler struct {
	innerMS *distuv.Exponential
}

func newExponentialDurationSampler(rng *rand.Rand, meanMS float64) durationSampler {
	return &exponentialDurationSampler{
		innerMS: &distuv.Exponential{
			Rate:   1 / meanMS, // mean = 1 / rate
			Source: erand.New(erand.NewSource(rng.Uint64())),
		},
	}
}

func (s *exponentialDurationSampler) sample() time.Duration {
	return time.Duration(int64(s.innerMS.Rand()) * 1e6) // sample duration in milliseconds
}

type uploadEventSampler interface {
	sample() *uploadEvent
}

type uploadEventSamplerImpl struct {
	nSharesPerUpload uint
	content          contentSampler
	authors          directory
}

func (s *uploadEventSamplerImpl) sample() *uploadEvent {
	from, _ := s.authors.sample()
	shareWith := make([]*ecdsa.PublicKey, s.nSharesPerUpload)
	for i := range shareWith {
		// just sample a random author, not really representative of real world, but for now gets us the Share and
		// Download load we want
		_, shareWith[i] = s.authors.sample()
	}
	return &uploadEvent{
		content:   s.content.sample(),
		from:      from,
		shareWith: shareWith,
	}
}

type contentSampler interface {
	sample() io.Reader
}

type gammaContentSampler struct {
	sizeSampler *distuv.Gamma
	rng         *rand.Rand
}

func newGammaContentSampler(rng *rand.Rand, shape float64, rate float64) *gammaContentSampler {
	return &gammaContentSampler{
		sizeSampler: &distuv.Gamma{
			Alpha:  shape,
			Beta:   rate,
			Source: erand.New(erand.NewSource(rng.Uint64())),
		},
		rng: rng,
	}
}

func (s *gammaContentSampler) sample() io.Reader {
	size := int(s.sizeSampler.Rand() * 1024)
	content := make([]byte, size)
	for c := 0; c < 3; c++ {
		if n, err := s.rng.Read(content); err != nil {
			panic(err)
		} else if n == size {
			return bytes.NewBuffer(content)
		}
	}
	panic(fmt.Errorf("unable to read %d bytes of content", size))
}
