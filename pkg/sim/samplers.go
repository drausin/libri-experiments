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
	"io/ioutil"
	"net"
	"github.com/drausin/libri/libri/common/logging"
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

func newDirectory(rng *rand.Rand, librarianAddrs []*net.TCPAddr, nAuthors uint, logLevelStr string) *directoryImpl {
	dataDir, err := ioutil.TempDir("", "sim-data-dir")
	maybePanic(err)
	authors := make([]*author.Author, nAuthors)
	keys := make([]keychain.GetterSampler, nAuthors)
	logger := server.NewDevLogger(server.GetLogLevel(logLevelStr))
	for i, config := range newAuthorConfigs(dataDir, librarianAddrs, nAuthors, logLevelStr) {

		// create keychains
		err := author.CreateKeychains(logger, config.KeychainDir, authorKeychainAuth,
			veryLightScryptN, veryLightScryptP)
		maybePanic(err)

		// load keychains
		authorKCs, selfReaderKCs, err := author.LoadKeychains(config.KeychainDir, authorKeychainAuth)
		maybePanic(err)
		keys[i] = authorKCs

		// create author
		authors[i], err = author.NewAuthor(config, authorKCs, selfReaderKCs, logger)
		maybePanic(err)
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

func newGammaContentSampler(rng *rand.Rand, sizeMean uint, sizeVar uint) *gammaContentSampler {
	// let x ~ Gamma(α, β) parameterized by shape α and rate β
	//
	// 	E[x] = α / β
	// 	Var[x] = α / β^2
	//
	// with some algebra, we get
	//
	// 	α = E[x]^2 / Var[x]
	//  β = E[x] / Var[x]
	beta := float64(sizeMean) / float64(sizeVar)
	alpha := float64(sizeMean) * beta
	return &gammaContentSampler{
		sizeSampler: &distuv.Gamma{
			Alpha:  alpha,
			Beta:   beta,
			Source: erand.New(erand.NewSource(rng.Uint64())),
		},
		rng: rng,
	}
}

func (s *gammaContentSampler) sample() io.Reader {
	size := int(s.sizeSampler.Rand())
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
