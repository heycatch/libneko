package neko_log

import (
	"io"
	"log"
	"os"

	"github.com/heycatch/libneko/syscallw"
)

var (
	LogWriter *logWriter
	LogWriterDisable = false
	TruncateOnStart = true
	NB4AGuiLogWriter io.Writer
)

type logWriter struct {
	writers []io.Writer
}

func SetupLog(maxSize int, path string) (err error) {
	if LogWriter != nil {
		return
	}

	var f *os.File

	f, err = os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err == nil {
		fd := int(f.Fd())

		if TruncateOnStart {
			syscallw.Flock(fd, syscallw.LOCK_EX)

			// Check if need truncate.
			if size, _ := f.Seek(0, io.SeekEnd); size > int64(maxSize) {
				// Read oldBytes for maxSize.
				f.Seek(-int64(maxSize), io.SeekCurrent)
				if oldBytes, err := io.ReadAll(f); err == nil {
					// Truncate file and write oldBytes.
					if err = f.Truncate(0); err == nil {
						if _, err := f.Write(oldBytes); err != nil {
							log.Printf("failed to write old log content: %v", err)
						}
					}
				}
			}
			syscallw.Flock(fd, syscallw.LOCK_UN)
		}
	}
	if err != nil {
		log.Printf("failed to open log: %v", err)
	}

	// For Linux we simply use stdout + file.
	LogWriter = &logWriter{[]io.Writer{os.Stdout, f}}

	// Setup std log.
	log.SetFlags(log.LstdFlags | log.LUTC)
	log.SetOutput(LogWriter)

	return
}

func (w *logWriter) Write(p []byte) (int, error) {
	if LogWriterDisable {
		return len(p), nil
	}

	for _, w := range w.writers {
		if w == nil {
			continue
		}

		if f, ok := w.(*os.File); ok {
			fd := int(f.Fd())
			syscallw.Flock(fd, syscallw.LOCK_EX)
			f.Write(p)
			syscallw.Flock(fd, syscallw.LOCK_UN)
		} else {
			w.Write(p)
		}
	}

	return len(p), nil
}

func (w *logWriter) Truncate() {
	for _, w := range w.writers {
		if w == nil {
			continue
		}

		if f, ok := w.(*os.File); ok {
			_ = f.Truncate(0)
		}
	}
}

func (w *logWriter) Close() error {
	for _, w := range w.writers {
		if w == nil {
			continue
		}

		if f, ok := w.(*os.File); ok {
			_ = f.Close()
		}
	}

	return nil
}
