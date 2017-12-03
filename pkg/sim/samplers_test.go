package sim

import (
	"testing"
	"math/rand"
	"github.com/drausin/libri/libri/author"
	"crypto/ecdsa"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/stretchr/testify/assert"
)

func TestUploadEventSamplerImplSample(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nSharesPerUpload := uint(2)
	sizeMean := uint(250 * 1024)                 // 250K
	sizeVar := uint((100 * 1024) * (100 * 1024)) // 100K std-dev
	d := &fixedDirectory{
		rng:          rng,
		returnAuthor: &author.Author{},
	}

	s := uploadEventSamplerImpl{
		nSharesPerUpload: nSharesPerUpload,
		content:          newGammaContentSampler(rng, sizeMean, sizeVar),
		authors:          d,
	}
	e := s.sample()
	assert.NotNil(t, e.content)
	assert.NotNil(t, e.from)
	assert.True(t, len(e.shareWith) == int(nSharesPerUpload))
}

type fixedDirectory struct {
	rng          *rand.Rand
	returnAuthor *author.Author
}

func (f *fixedDirectory) sample() (*author.Author, *ecdsa.PublicKey) {
	// just return empty author and random pub key
	id := ecid.NewPseudoRandom(f.rng)
	return f.returnAuthor, &id.Key().PublicKey
}

func (f *fixedDirectory) get(key *ecdsa.PublicKey) *author.Author {
	return f.returnAuthor
}
