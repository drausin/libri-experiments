package sim

import (
	"crypto/ecdsa"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"testing"

	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/stretchr/testify/assert"
)

func TestDirectoryImplSampleGet(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	librarianAddrs := []*net.TCPAddr{{IP: net.ParseIP("192.168.1.1"), Port: 20100}}
	dataDir, err := ioutil.TempDir("", "sim-data-dir")
	defer os.RemoveAll(dataDir)
	assert.Nil(t, err)
	nAuthors := uint(3)
	d := newDirectory(rng, dataDir, librarianAddrs, nAuthors, "info")

	// check sample behaves as expected
	a1, pubKey := d.sample()
	assert.NotNil(t, a1)
	assert.NotNil(t, pubKey)

	// check get returns author equal to a1
	a2 := d.get(pubKey)
	assert.Equal(t, a1, a2)
}

func TestUploadEventSamplerImplSample(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nSharesPerUpload := uint(2)
	d := &fixedDirectory{
		rng:          rng,
		returnAuthor: &author.Author{},
	}

	cs := newGammaContentSampler(rng, DefaultContentSizeKBGammaShape, DefaultContentSizeKBGammaRate)
	s := uploadEventSamplerImpl{
		nSharesPerUpload: nSharesPerUpload,
		content:          cs,
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
