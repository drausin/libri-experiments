package sim

import (
	"crypto/ecdsa"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestRunner_RunStop(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := newDefaultParameters()
	params.Duration = 500 * time.Millisecond
	params.NAuthors = 5
	params.DocsPerDay = 100000 // has to be ridiculously large to get any queries in 1s
	params.Profile = true

	dataDir, err := ioutil.TempDir("", "sim-data-dir")
	assert.Nil(t, err)
	defer os.RemoveAll(dataDir)
	librarianAddrs := []*net.TCPAddr{{IP: net.ParseIP("192.168.1.1"), Port: 20100}}
	r := NewRunner(params, dataDir, librarianAddrs, librarianAddrs)
	r.querier = &fixedQuerier{
		uploaded: make(map[string]io.Reader),
		rng:      rng,
	}

	r.Run()

	profilerAddr := fmt.Sprintf("http://localhost:%d/debug/pprof", localProfilerPort)
	resp, err := http.Get(profilerAddr)
	assert.Nil(t, err)
	assert.Equal(t, "200 OK", resp.Status)
}

type fixedQuerier struct {
	uploaded map[string]io.Reader
	mu       sync.Mutex
	rng      *rand.Rand
}

func (f *fixedQuerier) upload(author *author.Author, content io.Reader) (*api.Envelope, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	env := api.NewTestEnvelope(f.rng)
	envKey, err := api.GetKey(env)
	maybePanic(err)
	f.uploaded[envKey.String()] = content
	return env, nil
}

func (f *fixedQuerier) download(author *author.Author, content io.Writer, envKey id.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if upContent, in := f.uploaded[envKey.String()]; in {
		_, err := io.Copy(content, upContent)
		return err
	}
	return nil
}

func (f *fixedQuerier) share(
	author *author.Author, env *api.Envelope, readerPub *ecdsa.PublicKey,
) (id.ID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	envKey, err := api.GetKey(env)
	maybePanic(err)
	shareEnvKey := id.NewPseudoRandom(f.rng)
	f.uploaded[shareEnvKey.String()] = f.uploaded[envKey.String()]
	return shareEnvKey, nil
}

func newDefaultParameters() *Parameters {
	return &Parameters{
		Duration:                DefaultDuration,
		NAuthors:                DefaultNAuthors,
		DocsPerDay:              DefaultDocsPerDay,
		ContentSizeKBGammaShape: DefaultContentSizeKBGammaShape,
		ContentSizeKBGammaRate:  DefaultContentSizeKBGammaRate,
		SharesPerUpload:         DefaultSharesPerUpload,
		DownloadWaitMin:         DefaultDownloadWaitMin,
		DownloadWaitMax:         DefaultDownloadWaitMax,
		NUploaders:              DefaultNUploaders,
		NDownloaders:            DefaultNDownloaders,
		Profile:                 DefaultProfile,
		LogLevel:                DefaultLogLevel,
	}
}
