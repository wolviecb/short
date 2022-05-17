package shortie

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/patrickmn/go-cache"
)

var (
	store = map[string]cache.Item{"google": {Object: "https://google.com"}}
)

func TestGet(t *testing.T) {
	Pool = cache.NewFrom(Exp, Cleanup, store)
	tt := []struct {
		name string
		i    string
		v    string
		f    bool
	}{
		{
			"Missing",
			"test",
			"",
			false,
		},
		{
			"Found",
			"google",
			"https://google.com",
			true,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			v, f := get(tc.i)
			if !reflect.DeepEqual(v, tc.v) {
				t.Fatalf("expected: %v, got %v", tc.v, v)
			}
			if !reflect.DeepEqual(f, tc.f) {
				t.Fatalf("expected: %v, got %v", tc.f, f)
			}
		})
	}
	Pool.Flush()
}

func TestSet(t *testing.T) {
	Pool = cache.New(Exp, Cleanup)
	tt := []struct {
		name string
		k    string
		v    string
		f    bool
	}{
		{
			"Set",
			"Key",
			"Value",
			true,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			set(tc.v, tc.k)
			v, err := get(tc.k)
			if !reflect.DeepEqual(tc.v, v) {
				t.Fatalf("expected: %v, got %v", tc.v, v)
			}
			if !reflect.DeepEqual(err, true) {
				t.Fatalf("Key %s not found", tc.k)
			}
		})
	}
	Pool.Flush()
}

func TestRandStringBytesMaskImprSrc(t *testing.T) {
	tt := []struct {
		name string
		i    int
	}{
		{
			"10",
			10,
		},
		{
			"1",
			1,
		},
		{
			"100",
			100,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := randStringBytesMaskImprSrc(tc.i)
			if len(s) != tc.i {
				t.Fatalf("Bad Length, expecting %v got %v", tc.i, len(s))
			}
		})
	}
}

func benchmarkRandStringBytesMasImprSrc(i int, b *testing.B) {
	for n := 0; n < b.N; n++ {
		randStringBytesMaskImprSrc(i)
	}
}

func BenchmarkRandStringBytesMasImprSrc10(b *testing.B) {
	benchmarkRandStringBytesMasImprSrc(10, b)
}

func BenchmarkRandStringBytesMasImprSrc100(b *testing.B) {
	benchmarkRandStringBytesMasImprSrc(100, b)
}

func BenchmarkRandStringBytesMasImprSrc1000(b *testing.B) {
	benchmarkRandStringBytesMasImprSrc(1000, b)
}

func TestRedirect(t *testing.T) {
	Pool = cache.NewFrom(Exp, Cleanup, store)
	tt := []struct {
		name string
		i    string
		r    string
		err  error
	}{
		{
			"Not Found",
			"badkey",
			"",
			ErrNotFound,
		},
		{
			"Found",
			"google",
			"https://google.com",
			nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			r, err := redirect(tc.i)
			if !errors.Is(tc.err, err) {
				t.Fatalf("Bad error, expected %v, got %v", tc.err, err)
			}
			if !reflect.DeepEqual(tc.r, r) {
				t.Fatalf("Bad value, expected %v, got %v", tc.r, r)
			}
		})
	}
}

func TestShortener(t *testing.T) {
	tt := []struct {
		name string
		i    string
		s    int
		rs   int
		err  error
	}{
		{
			"Bad Request",
			"badurl",
			10,
			0,
			ErrBadRequest,
		},
		{
			"Good Request",
			"https://www.google.com",
			10,
			10,
			nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			r, err := shortener(tc.i, tc.s)
			if !errors.Is(tc.err, err) {
				t.Fatalf("Bad error, expected %v, got %v", tc.err, err)
			}
			if len(r) != tc.rs {
				t.Fatalf("Bad value, expected %v, got %v (%s)", tc.s, len(r), r)
			}
		})
		Pool.Flush()
	}
}

func TestDumpDbTOFile(t *testing.T) {
	tt := []struct {
		name string
		p    *cache.Cache
		r    int
		f    *os.File
		err  error
	}{
		{
			"Error",
			cache.New(Exp, Cleanup),
			0,
			func() *os.File { f, _ := os.Create(fmt.Sprintf("%s/.json", randStringBytesMaskImprSrc(5))); return f }(),
			os.ErrInvalid,
		},
		{
			"0-v",
			cache.New(Exp, Cleanup),
			0,
			func() *os.File { f, _ := os.Create(fmt.Sprintf("%s.json", randStringBytesMaskImprSrc(5))); return f }(),
			nil,
		},
		{
			"1-v",
			cache.NewFrom(Exp, Cleanup, store),
			1,
			func() *os.File { f, _ := os.Create(fmt.Sprintf("%s.json", randStringBytesMaskImprSrc(5))); return f }(),
			nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			Pool = tc.p
			r, err := dumpDbTOFile(tc.f)
			if !errors.Is(tc.err, err) {
				t.Fatalf("Bad error, expected %#v, got %#v", tc.err, err)
			}
			if tc.r != r {
				t.Fatalf("Bad value, expected %#v, got %#v", tc.r, r)

			}
		})
		if tc.f != nil {
			os.Remove(tc.f.Name())
		}
		Pool.Flush()
	}
}
