package logging

import (
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FileWriter writes logs to the filesystem.
type FileWriter struct {
	closed              atomic.Value
	config              FileWriterConfig
	mu                  sync.Mutex
	logFile             *os.File
	logFileNameFullPath string
	byteCounter         int64
	compressorWG        sync.WaitGroup
}

// FileWriterConfig contains the configuration for a FileWriter
type FileWriterConfig struct {
	LogDirName  string
	LogFileName string
	Compress    bool
	// If MaxDaysToKeep is 0 we keep the all log files regardless of age
	MaxTimeTimeToKeep   time.Duration
	MaxLogFileSizeBytes int64
}

const (
	minLogFileSizeBytes     = int64(100000)
	maxLogFileSizeBytes     = math.MaxInt64
	logDirPermissions       = 0755
	logFilePermissions      = 0644
	defaultLogFileSizeBytes = int64(1000000)
	defaultLogDirName       = "./log"
	defaultLogFileName      = "log.log"
	archiveNameFormat       = "2006-01-02T15-04-05.00000"
	compressedExtension     = "gz"
	processingExtenstion    = "processing"
)

// NewFileWriter creates a new FileWriter given a FileWriterConfig
func NewFileWriter(c FileWriterConfig) *FileWriter {
	if c.MaxLogFileSizeBytes == 0 {
		c.MaxLogFileSizeBytes = defaultLogFileSizeBytes
		fmt.Printf("filesize %d\n", c.MaxLogFileSizeBytes)
	}
	if c.LogDirName == "" {
		c.LogDirName = defaultLogDirName
	}
	if c.LogFileName == "" {
		c.LogFileName = defaultLogFileName
	}

	fileWriter := FileWriter{
		config:              c,
		logFileNameFullPath: path.Join(c.LogDirName, c.LogFileName),
	}

	err := fileWriter.initialize()
	if err != nil {
		lg.Fatalw("error initializing filewriter", "err", err)
	}

	return &fileWriter
}

// Close the logger.
func (w *FileWriter) Close() error {
	w.closed.Store(true)
	w.compressorWG.Wait()
	return w.logFile.Close()
}

func (w *FileWriter) Write(msg []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed.Load() != nil {
		return 0, os.ErrClosed
	}

	n, err := w.logFile.Write(msg)
	if err != nil {
		fmt.Printf("logfile error : %v", err)
	}

	w.byteCounter += int64(n)

	if w.byteCounter > w.config.MaxLogFileSizeBytes {
		w.rotate()
	}

	return n, err
}

// initialize should only be called from NewFileWriter and assumes that
// w.byteCounter is already 0.
func (w *FileWriter) initialize() error {
	// ensure the logdir exists
	err := os.MkdirAll(w.config.LogDirName, logDirPermissions)
	if err != nil {
		return err
	}

	// perform periodic cleanup tasks before we do anything else
	err = w.cleanup()
	if err != nil {
		return err
	}

	// check if a logfile exists
	info, err := os.Stat(w.logFileNameFullPath)
	if err == nil {
		// if the size is above the threshold we archive it
		if info.Size() >= w.config.MaxLogFileSizeBytes {
			err = w.archive(w.logFileNameFullPath)
			if err != nil {
				return err
			}
			fmt.Println("rotated initial logfile")
		} else {
			// if we are not above the threshold we set the byteCounter to the length of the file
			w.byteCounter = info.Size()
			fmt.Printf("will append to logfile, size=%d\n", info.Size())
		}
	}

	// this will create the file if it doesn't exist and keep appending to it if it does
	w.logFile, err = os.OpenFile(w.logFileNameFullPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, logFilePermissions)
	if err != nil {
		return err
	}

	return nil
}

// cleanup performs housekeeping.
func (w *FileWriter) cleanup() error {
	// check if we have logfiles that are too old
	dirEnts, err := os.ReadDir(w.config.LogDirName)
	if err != nil {
		return err
	}

	for _, dirEnt := range dirEnts {
		info, err := dirEnt.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(w.config.LogDirName, info.Name())

		// if the age is greater than MaxDaysToKeep we delete the file
		if w.config.MaxTimeTimeToKeep > 0 && time.Since(info.ModTime()) > w.config.MaxTimeTimeToKeep {
			err := os.Remove(fullPath)
			if err != nil {
				fmt.Printf("error removing %s: %v\n", fullPath, err)
			}
			fmt.Printf("%s removed %d\n", fullPath, time.Since(info.ModTime()))
			continue
		}

		// if we find an uncompressed archive file we compress it
		if strings.HasSuffix(info.Name(), "log") && info.Name() != w.config.LogFileName {
			fmt.Printf("compress %s\n", info.Name())
			w.compressorWG.Add(1)
			go w.compress(fullPath)
			continue
		}
	}

	return nil
}

// rotate the log file.  This assumes that the w.mu is locked.
func (w *FileWriter) rotate() error {
	if w.logFile != nil {
		err := w.logFile.Close()
		if err != nil {
			return err
		}
	} else {
		panic("logging inconsistency: logFile was nil")
	}

	// ensure the logdir exists
	err := os.MkdirAll(w.config.LogDirName, logDirPermissions)
	if err != nil {
		return err
	}

	w.archive(w.logFileNameFullPath)

	// Open logfile for append.
	w.logFile, err = os.OpenFile(w.logFileNameFullPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, logFilePermissions)
	if err != nil {
		return err
	}

	w.byteCounter = 0

	return nil
}

// archive renames and potentially postprocesses log files
func (w *FileWriter) archive(fn string) error {
	newName := archiveName(w.logFileNameFullPath)
	err := os.Rename(w.logFileNameFullPath, newName)
	if err != nil {
		return err
	}

	if w.config.Compress {
		w.compressorWG.Add(1)
		go w.compress(newName)
	}

	return nil
}

// compress the named file.  Note that before you call this function you MUST
// call w.compressorWG.Add(1)
func (w *FileWriter) compress(fn string) {
	defer w.compressorWG.Done()

	in, err := os.Open(fn)
	if err != nil {
		fmt.Printf("failed to open input file for compression file = %s: %v", fn, err)
		return
	}
	defer in.Close()

	compressedFilename := fn + "." + compressedExtension
	tempFilename := compressedFilename + "." + processingExtenstion

	out, err := os.OpenFile(tempFilename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, logFilePermissions)
	if err != nil {
		lg.Errorw("failed to open output file for compression", "file", tempFilename, "err", err)
		return
	}
	defer out.Close()

	zipper := gzip.NewWriter(out)
	defer zipper.Close()

	n, err := io.Copy(zipper, in)
	if err != nil {
		lg.Errorf("failed to compress %s: %v", tempFilename, err)
		os.Remove(tempFilename)
		return
	}

	err = os.Rename(tempFilename, compressedFilename)
	if err != nil {
		lg.Errorw("failed to rename processed file", "fromName", tempFilename, "toName", compressedFilename, "err", err)
	}

	err = os.Remove(fn)
	if err != nil {
		lg.Errorw("failed to remove processed log file", "filename", fn, "err", err)
	}

	lg.Infow("compressed", "file", compressedFilename, "originalSize", n)
}

// archiveName borrows the formatting from https://github.com/natefinch/lumberjack/
// for compatibility
func archiveName(current string) string {
	dir := filepath.Dir(current)
	filename := filepath.Base(current)
	ext := filepath.Ext(current)
	prefix := filename[:len(filename)-len(ext)]

	timestamp := time.Now().Format(archiveNameFormat)

	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", prefix, timestamp, ext))
}
