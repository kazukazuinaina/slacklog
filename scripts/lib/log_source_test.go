package slacklog

import (
	"io/ioutil"
	"os"
	"testing"
)

func testLogSource(t *testing.T, src LogSource) {
	t.Helper()
	for n, tc := range []struct{ name, expect string }{
		{"data01.json", "{}\n"},
		{"data02.json", "{\"foo\":\"bar\",\"baz\":123}\n"},
		{"data03.txt", "Hello World\n"},
	} {
		rc, err := src.Open(tc.name)
		if err != nil {
			t.Fatalf("#%d %s failed to open: %s", n, tc.name, err)
		}
		b, err := ioutil.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("#%d %s failed to read: %s", n, tc.name, err)
		}
		s := string(b)
		if s != tc.expect {
			t.Fatalf("#%d %s content mismatch:\nwant: %q\ngot: %q", n, tc.name, tc.expect, s)
		}
	}
	// try to load unexist file.
	rc, err := src.Open("never_exist")
	if err == nil {
		rc.Close()
		t.Fatal("unexpected success to open")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("not IsNotExist error, got: %s", err)
	}
	//t.Logf("%s: %s", t.Name(), err)
}

func TestDirSource(t *testing.T) {
	testLogSource(t, DirSource("testdata/log_source/dir_source"))
}

func TestTarSource(t *testing.T) {
	src, err := NewTarSource("testdata/log_source/tar_source.tar.gz", "tar_source")
	if err != nil {
		t.Fatalf("failed to NewTarSource: %s", err)
	}
	testLogSource(t, src)
}
