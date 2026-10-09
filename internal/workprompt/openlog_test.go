package workprompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// sameSecond is a fixed start time; every open in these tests uses its second.
var sameSecond = time.Date(2026, 10, 9, 11, 35, 50, 0, time.UTC)

func sameSecondHeader() Header {
	return Header{TaskID: testID, Adapter: "pi", Model: "m", Timeout: time.Minute, Start: sameSecond}
}

func TestOpenLogSuffixSecondAndThirdOpenDoNotOverwrite(t *testing.T) {
	root := t.TempDir()
	base := LogPath(root, testID, "pi", sameSecond)
	var files []*os.File
	for i := 0; i < 3; i++ {
		f, err := OpenLog(base, sameSecondHeader())
		if err != nil {
			t.Fatalf("open %d in the same second: %v", i+1, err)
		}
		files = append(files, f)
	}
	want := []string{
		base,
		strings.TrimSuffix(base, ".log") + "_2.log",
		strings.TrimSuffix(base, ".log") + "_3.log",
	}
	for i, f := range files {
		if f.Name() != want[i] {
			t.Errorf("open %d Name() = %s, want %s", i+1, f.Name(), want[i])
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(data), "TaskFactory worker log\nTask: TF-055\nAdapter: pi\n") {
			t.Errorf("file %d lacks its own header:\n%s", i+1, data)
		}
	}
	if !(want[0] < want[1] && want[1] < want[2]) {
		t.Fatalf("names do not sort by base then suffix: %v", want)
	}
	if !(strings.TrimSuffix(filepath.Base(want[0]), ".log")+".log" < filepath.Base(want[1])) {
		t.Fatalf("suffixed name %s does not sort after the base name", want[1])
	}
}

func TestOpenLogConcurrentOpensAllSucceedWithDistinctFiles(t *testing.T) {
	root := t.TempDir()
	base := LogPath(root, testID, "pi", sameSecond)
	const n = 20
	var wg sync.WaitGroup
	names := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f, err := OpenLog(base, sameSecondHeader())
			if err != nil {
				errs[i] = err
				return
			}
			names[i] = f.Name()
			errs[i] = f.Close()
		}(i)
	}
	wg.Wait()
	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("concurrent open %d: %v", i, errs[i])
		}
		if seen[names[i]] {
			t.Fatalf("two opens returned the same file %s", names[i])
		}
		seen[names[i]] = true
	}
	entries, err := os.ReadDir(filepath.Dir(base))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != n {
		t.Fatalf("log directory has %d files, want %d", len(entries), n)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(base), e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(data), "TaskFactory worker log\n") {
			t.Errorf("%s lacks a header", e.Name())
		}
	}
}

func TestOpenLogExhaustedReturnsErrorAndCreatesNothing(t *testing.T) {
	root := t.TempDir()
	base := LogPath(root, testID, "pi", sameSecond)
	dir := filepath.Dir(base)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stem := strings.TrimSuffix(base, ".log")
	for n := 1; n <= maxLogAttempts; n++ {
		name := base
		if n > 1 {
			name = fmt.Sprintf("%s_%d.log", stem, n)
		}
		if err := os.WriteFile(name, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := OpenLog(base, sameSecondHeader())
	if err == nil || !strings.Contains(err.Error(), "names") {
		if f != nil {
			_ = f.Close()
		}
		t.Fatalf("OpenLog error = %v, want the bound error", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxLogAttempts {
		t.Fatalf("directory has %d files after exhaustion, want %d", len(entries), maxLogAttempts)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "old" {
			t.Fatalf("%s was overwritten", e.Name())
		}
	}
}
