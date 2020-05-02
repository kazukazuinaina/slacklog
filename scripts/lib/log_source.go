package slacklog

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type compressType int

const (
	compressRaw compressType = iota
	compressGzip
	compressBzip2
)

// LogSource provides interface to access log files.
type LogSource interface {
	// Open opens a named log file.
	Open(name string) (io.ReadCloser, error)
}

// DirSource implements LogSource for physical directory.
type DirSource string

// Open opens a named log file.
func (ds DirSource) Open(name string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(string(ds), name))
}

var _ LogSource = DirSource("")

// TarSource implements LogSource for tar archive.
type TarSource struct {
	tarFilename   string
	compressType  compressType
	prefixToStrip string
}

// NewTarSource creates a new TarSource instance.
func NewTarSource(filename string, prefix string) (*TarSource, error) {
	var ct compressType
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".gz":
		ct = compressGzip
	case ".bz2":
		ct = compressBzip2
	case ".tar":
		ct = compressRaw
	default:
		return nil, fmt.Errorf("unsupported compression type: %s", filename)
	}

	fi, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("not regular file: %s", filename)
	}

	return &TarSource{
		tarFilename:   filename,
		compressType:  ct,
		prefixToStrip: prefix,
	}, nil
}

type closers []io.Closer

func (cc closers) Close() error {
	var err error
	for _, c := range cc {
		cerr := c.Close()
		if cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

type compositeReadCloser struct {
	r io.Reader
	c io.Closer
}

// Read implements io.Reader
func (rc *compositeReadCloser) Read(b []byte) (int, error) {
	return rc.r.Read(b)
}

// Close implements io.Closer
func (rc *compositeReadCloser) Close() error {
	if rc.c == nil {
		return nil
	}
	return rc.c.Close()
}

// Open extracts an entry in tar file and return its io.ReadCloser.
func (ts *TarSource) Open(name string) (io.ReadCloser, error) {
	tr, c, err := ts.openTar()
	if err != nil {
		return nil, err
	}
	if ts.prefixToStrip != "" {
		name = path.Join(ts.prefixToStrip, name)
	}
	for {
		h, err := tr.Next()
		if err != nil {
			c.Close()
			if err == io.EOF {
				return nil, &os.PathError{
					Op:   "read tar",
					Path: ts.tarFilename + ":" + name,
					Err:  os.ErrNotExist,
				}
			}
			return nil, err
		}
		if h.Name == name {
			return &compositeReadCloser{r: tr, c: c}, nil
		}
	}
}

func (ts *TarSource) openTar() (*tar.Reader, io.Closer, error) {
	f, err := os.Open(ts.tarFilename)
	if err != nil {
		return nil, nil, err
	}
	switch ts.compressType {
	case compressRaw:
		return tar.NewReader(f), f, nil
	case compressGzip:
		gr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		return tar.NewReader(gr), closers{gr, f}, nil
	case compressBzip2:
		br := bzip2.NewReader(f)
		return tar.NewReader(br), f, nil
	default:
		// never reached.
		return nil, nil, fmt.Errorf("unsupported compressType: %d", ts.compressType)
	}
}

var _ LogSource = (*TarSource)(nil)
