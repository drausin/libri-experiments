package sim

import (
	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/common/id"
	"io"
	"math/rand"
	"testing"
	"os"
	"io/ioutil"
	"github.com/stretchr/testify/assert"
	"net"
	"time"
	"crypto/ecdsa"
)

func TestRunner_RunStop(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := NewDefaultParameters()
	params.Duration = 500 * time.Millisecond
	params.NAuthors = 5
	params.DocsPerDay = 100000 // has to be ridiculously large to get any queries in 1s

	dataDir, err := ioutil.TempDir("", "sim-data-dir")
	assert.Nil(t, err)
	defer os.RemoveAll(dataDir)
	librarianAddrs := []*net.TCPAddr{{IP: net.ParseIP("192.168.1.1"), Port: 20100}}
	r := NewRunner(params, dataDir, librarianAddrs)
	r.querier = &fixedQuerier{
		uploaded: make(map[string]io.Reader),
		rng:      rng,
	}

	r.Run()
}

type fixedQuerier struct {
	uploaded map[string]io.Reader
	rng      *rand.Rand
}

func (f *fixedQuerier) upload(author *author.Author, content io.Reader) (id.ID, error) {
	envKey := id.NewPseudoRandom(f.rng)
	f.uploaded[envKey.String()] = content
	return envKey, nil
}

func (f *fixedQuerier) download(author *author.Author, content io.Writer, envKey id.ID) error {
	if upContent, in := f.uploaded[envKey.String()]; in {
		_, err := io.Copy(content, upContent)
		return err
	}
	return nil
}

func (f *fixedQuerier) share(author *author.Author, envKey id.ID, readerPub *ecdsa.PublicKey) (id.ID, error) {
	shareEnvKey := id.NewPseudoRandom(f.rng)
	f.uploaded[shareEnvKey.String()] = f.uploaded[envKey.String()]
	return shareEnvKey, nil
}
