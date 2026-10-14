package utils

import (
	"reflect"
	"testing"
)

func TestParseURIList(t *testing.T) {
	tcs := []struct {
		name string
		data string
		want []string
	}{
		{"empty", "", nil},
		{"single file", "file:///tmp/a.png\n", []string{"/tmp/a.png"}},
		{"multiple files", "file:///tmp/a.png\r\nfile:///home/u/b.txt\nfile:///home/u/c.jpg\r\n", []string{"/tmp/a.png", "/home/u/b.txt", "/home/u/c.jpg"}},
		{"path with spaces", "file:///tmp/my%20file.png\n", []string{"/tmp/my file.png"}},
		{"chinese path", "file:///tmp/%E4%B8%AD%E6%96%87.png\n", []string{"/tmp/中文.png"}},
		{"chinese path raw utf8", "file:///tmp/中文.png\n", []string{"/tmp/中文.png"}},
		{"non-file scheme skipped", "https://example.com/a.png\nmailto:x@y.z\n", nil},
		{"mixed schemes", "https://example.com/a.png\nfile:///tmp/a.png\n", []string{"/tmp/a.png"}},
		{"comment lines skipped", "#comment\nfile:///tmp/a.png\n", []string{"/tmp/a.png"}},
	}
	for _, tc := range tcs {
		if got := parseURIList([]byte(tc.data)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: parseURIList(%q) = %q, want %q", tc.name, tc.data, got, tc.want)
		}
	}
}

func TestParseGnomeCopiedFiles(t *testing.T) {
	tcs := []struct {
		name string
		data string
		want []string
	}{
		{"empty", "", nil},
		{"copy marker skipped", "copy\nfile:///tmp/a.png\n", []string{"/tmp/a.png"}},
		{"cut marker skipped", "cut\nfile:///tmp/a.png\nfile:///tmp/my%20file.png\n", []string{"/tmp/a.png", "/tmp/my file.png"}},
		{"chinese paths", "copy\r\nfile:///tmp/%E4%B8%AD%E6%96%87.png\r\n", []string{"/tmp/中文.png"}},
		{"non-file scheme skipped", "copy\nhttps://example.com/a.png\n", nil},
	}
	for _, tc := range tcs {
		if got := parseGnomeCopiedFiles([]byte(tc.data)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: parseGnomeCopiedFiles(%q) = %q, want %q", tc.name, tc.data, got, tc.want)
		}
	}
}

func TestBuildURIList(t *testing.T) {
	tcs := []struct {
		name  string
		paths []string
		want  string
	}{
		{"empty", nil, ""},
		{"single", []string{"/tmp/a.png"}, "file:///tmp/a.png\n"},
		{"multiple", []string{"/tmp/a.png", "/home/u/b.txt"}, "file:///tmp/a.png\nfile:///home/u/b.txt\n"},
		{"space escaped", []string{"/tmp/my file.png"}, "file:///tmp/my%20file.png\n"},
		{"chinese escaped", []string{"/tmp/中文.png"}, "file:///tmp/%E4%B8%AD%E6%96%87.png\n"},
	}
	for _, tc := range tcs {
		if got := buildURIList(tc.paths); got != tc.want {
			t.Errorf("%s: buildURIList(%q) = %q, want %q", tc.name, tc.paths, got, tc.want)
		}
	}
}

func TestBuildParseRoundtrip(t *testing.T) {
	pathSets := [][]string{
		{"/tmp/a.png"},
		{"/tmp/a.png", "/home/u/b.txt", "/home/u/c.jpg"},
		{"/tmp/my file.png", "/tmp/中文.png", "/tmp/a(b).png"},
	}
	for _, paths := range pathSets {
		if got := parseURIList([]byte(buildURIList(paths))); !reflect.DeepEqual(got, paths) {
			t.Errorf("parseURIList(buildURIList(%q)) = %q, want %q", paths, got, paths)
		}
		if got := parseGnomeCopiedFiles([]byte("copy\n" + buildURIList(paths))); !reflect.DeepEqual(got, paths) {
			t.Errorf("parseGnomeCopiedFiles(copy+buildURIList(%q)) = %q, want %q", paths, got, paths)
		}
	}
}

func TestParseTargets(t *testing.T) {
	tcs := []struct {
		name string
		data string
		want []string
	}{
		{"wl-paste newline separated", "text/plain;charset=utf-8\ntext/plain\nUTF8_STRING\n", []string{"text/plain;charset=utf-8", "text/plain", "UTF8_STRING"}},
		{"xclip whitespace separated", "TARGETS text/plain UTF8_STRING STRING", []string{"TARGETS", "text/plain", "UTF8_STRING", "STRING"}},
		{"empty", "\n", []string{}},
	}
	for _, tc := range tcs {
		if got := parseTargets([]byte(tc.data)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: parseTargets(%q) = %q, want %q", tc.name, tc.data, got, tc.want)
		}
	}
}
