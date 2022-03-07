package logging

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileWriter(t *testing.T) {
	dir, err := ioutil.TempDir("", "filewriter-*")
	assert.NoError(t, err)
	assert.NotEmpty(t, dir)
	defer os.RemoveAll(dir)

	fw := NewFileWriter(FileWriterConfig{
		LogDirName:          dir,
		LogFileName:         "logfile.log",
		Compress:            true,
		MaxTimeTimeToKeep:   1,
		MaxLogFileSizeBytes: 1000,
	})

	// generate some log files
	for i := 0; i < 120; i++ {
		n, err := fw.Write([]byte(randomString(50)))
		assert.NoError(t, err)
		assert.Greater(t, n, 0)
	}

	// dirty, but more effective
	fw.compressorWG.Wait()

	// we expect to see at least three files
	files, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 3)

	for _, f := range files {
		fmt.Println(f.Name())
	}

}

func randomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
