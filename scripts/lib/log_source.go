package slacklog

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"errors"
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

// LogSourceIter defines iterator of directory entries.
type LogSourceIter interface {
	Next() error
	Name() string
	Close() error
}

// ReadDirAll reads all entries from LogSourceIter as []string.
func ReadDirAll(iter LogSourceIter) ([]string, error) {
	names := []string{}
	for {
		err := iter.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return names, nil
			}
			return nil, err
		}
		name := iter.Name()
		if name != "" {
			names = append(names, name)
		}
	}
}

// LogSource provides interface to access log files.
type LogSource interface {
	// Open opens a named log file.
	Open(name string) (io.ReadCloser, error)

	// OpenDir opens directory to iterate its entries.
	OpenDir(name string) (LogSourceIter, error)
}

const keyName = "/users.json"

// OpenAsLogSource opens a correct LogSource depending type of name
// automatically.
func OpenAsLogSource(name string) (LogSource, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return DirSource(name), nil
	}

	// open as tar file and detect prefix automatically.
	ts, err := NewTarSource(name, "")
	if err != nil {
		return nil, err
	}
	tr, c, err := ts.openTar()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	for {
		h, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("failed to detect prefix in archive: %s", name)
			}
			return nil, err
		}
		if strings.HasSuffix(h.Name, keyName) {
			ts.prefixToStrip = h.Name[:len(h.Name)-len(keyName)]
			return ts, nil
		}
	}
}

// DirSource implements LogSource for physical directory.
type DirSource string

// Open opens a named log file.
func (ds DirSource) Open(name string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(string(ds), name))
}

// OpenDir opens directory to iterate its entries.
func (ds DirSource) OpenDir(name string) (LogSourceIter, error) {
	path := filepath.Join(string(ds), name)
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", path)
	}
	d, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &dirSourceIter{name: name, dir: d}, nil
}

var _ LogSource = DirSource("")

type dirSourceIter struct {
	name string
	dir  *os.File
	next os.FileInfo
}

func (dsi *dirSourceIter) Next() error {
	dsi.next = nil
	fis, err := dsi.dir.Readdir(1)
	if err != nil {
		return err
	}
	dsi.next = fis[0]
	return nil
}

func (dsi *dirSourceIter) Name() string {
	if dsi.next == nil {
		return ""
	}
	// FIXME: should return only files? (exclude dirs)
	return path.Join(dsi.name, dsi.next.Name())
}

func (dsi *dirSourceIter) Close() error {
	dsi.next = nil
	return dsi.dir.Close()
}

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
			if errors.Is(err, io.EOF) {
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

// OpenDir opens directory to iterate its entries.
func (ts *TarSource) OpenDir(name string) (LogSourceIter, error) {
	tr, c, err := ts.openTar()
	if err != nil {
		return nil, err
	}
	// FIXME: are there better ways to manage trailing '/'
	for strings.HasSuffix(name, "/") {
		name = name[:len(name)-1]
	}
	stripN := 0
	if ts.prefixToStrip != "" {
		name = path.Join(ts.prefixToStrip, name)
		stripN = len(ts.prefixToStrip) + 1
	}
	return &tarSourceIter{
		tr:     tr,
		c:      c,
		prefix: name + "/",
		stripN: stripN,
	}, nil
}

var _ LogSource = (*TarSource)(nil)

type tarSourceIter struct {
	tr   *tar.Reader
	c    io.Closer
	next *tar.Header

	prefix string
	stripN int
}

func (tsi *tarSourceIter) Next() error {
	tsi.next = nil
	for {
		h, err := tsi.tr.Next()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(h.Name, tsi.prefix) {
			continue
		}
		name := h.Name[len(tsi.prefix):]
		if name == "" {
			continue
		}
		x := strings.IndexRune(name, '/')
		switch {
		case x < 0:
			// a file found.
			tsi.next = h
			return nil
		case x == len(name)-1:
			// a dir found.
			tsi.next = h
			return nil
		default:
			// entries in subdir.
			continue
		}
	}
	return nil
}

func (tsi *tarSourceIter) Name() string {
	if tsi.next == nil {
		return ""
	}
	// FIXME: should return only files? (exclude dirs)
	if tsi.next.Typeflag == tar.TypeDir {
		return tsi.next.Name[tsi.stripN : len(tsi.next.Name)-1]
	}
	return tsi.next.Name[tsi.stripN:]
}

func (tsi *tarSourceIter) Close() error {
	tsi.next = nil
	return tsi.c.Close()
}
